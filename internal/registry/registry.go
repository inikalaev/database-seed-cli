// Package registry houses the mechanism dispatch layer.
//
// It wraps seedapi.Registry with inference helpers that pick the best-matching
// mechanism for a given column, and with glue that converts schema.Column into
// seedapi.Column for the scoring call.
package registry

import (
	"strings"

	"github.com/ivannikolaev/seed-cli/cli/internal/factories"
	"github.com/ivannikolaev/seed-cli/cli/internal/schema"
	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type Registry struct {
	ordered []seedapi.Factory
	byName  map[string]seedapi.Factory
}

// New builds a registry from the given factories. Duplicates by Name() are
// silently dropped — the first occurrence wins, so callers can safely pass
// builtins followed by user-registered plugins without pre-dedup.
func New(mechanisms []seedapi.Factory) *Registry {
	r := &Registry{byName: map[string]seedapi.Factory{}}
	for _, m := range mechanisms {
		if _, exists := r.byName[m.Name()]; exists {
			continue
		}
		r.byName[m.Name()] = m
		r.ordered = append(r.ordered, m)
	}
	return r
}

func (r *Registry) Get(name string) (seedapi.Factory, bool) {
	m, ok := r.byName[name]
	return m, ok
}

func (r *Registry) All() []seedapi.Factory { return r.ordered }

// Default assembles the canonical registry: builtin factories first, then any
// user-registered factories from init() via seedapi.Register. Single source of
// truth shared by the CLI, pkg/seed, and third-party embeddings.
func Default() *Registry {
	mechs := append([]seedapi.Factory{}, factories.All()...)
	mechs = append(mechs, seedapi.Default().All()...)
	return New(mechs)
}

// InferenceResult carries the best factory and its confidence.
type InferenceResult struct {
	Factory    seedapi.Factory
	Score      seedapi.MatchScore
	Unresolved bool
}

// Infer picks the highest-scoring factory for the column, ties broken by
// registration order (i.e. the order in which factories.All() returns them).
// If a factory implements seedapi.Matcher its Match() is called; otherwise
// the registry auto-scores by Name() (StrongMatch) and Tags() (NameMatch).
//
// Since WeakNameMatch(60) was introduced, generic builtins (bool/int/decimal/date/
// hstore/timestamp-by-pattern) no longer compete with named factories on a tie:
// NameMatch(70) beats WeakNameMatch(60) by score alone, regardless of registration
// order. Tie-breaking is only meaningful within the same tier — typically between
// two plugins or between a WeakNameMatch plugin and a WeakNameMatch builtin.
func (r *Registry) Infer(col seedapi.Column, locale string) InferenceResult {
	ctx := seedapi.MatchContext{Column: col, Locale: locale}
	var best seedapi.Factory
	var bestScore seedapi.MatchScore
	for _, f := range r.ordered {
		var s seedapi.MatchScore
		if m, ok := f.(seedapi.Matcher); ok {
			s = m.Match(ctx)
		} else {
			s = autoMatch(f, col)
		}
		if s > bestScore {
			best = f
			bestScore = s
		}
	}
	res := InferenceResult{Factory: best, Score: bestScore}
	// Unresolved threshold is WeakNameMatch. TypeMatch (type-only signal without a
	// name hint, e.g. bare `timestamp` or integer named `status`) stays unresolved
	// so the user reviews the default. WeakNameMatch and above is considered
	// confident enough (generic type with a sensible default: bool, date, decimal,
	// hstore, ordinary integer, timestamp by name pattern). Any plugin returning
	// NameMatch or higher overrides WeakNameMatch.
	if best == nil || bestScore < seedapi.WeakNameMatch {
		res.Unresolved = true
	}
	return res
}

// autoMatch scores a factory against a column using Name() and Tags().
// Both sides are normalised (lowercase, underscores and hyphens stripped)
// before comparison so "first_name" matches "firstname" etc.
func autoMatch(f seedapi.Factory, col seedapi.Column) seedapi.MatchScore {
	colNorm := factories.NormName(col.Name)
	if colNorm == factories.NormName(f.Name()) {
		return seedapi.StrongMatch
	}
	for _, tag := range f.Tags() {
		if strings.Contains(colNorm, factories.NormName(tag)) {
			return seedapi.NameMatch
		}
	}
	return seedapi.NoMatch
}

// ToAPIColumn converts an internal schema.Column into the public API type.
func ToAPIColumn(tbl schema.Table, col schema.Column, enumValues []string, fkTarget string) seedapi.Column {
	return seedapi.Column{
		Schema:       tbl.Schema,
		Table:        tbl.Name,
		Name:         col.Name,
		DataType:     col.DataType,
		UDTName:      col.UDTName,
		Nullable:     col.Nullable,
		ArrayDims:    col.ArrayDims,
		EnumName:     col.EnumName,
		EnumValues:   enumValues,
		CharMaxLen:   col.CharMaxLen,
		NumPrecision: col.NumPrecision,
		NumScale:     col.NumScale,
		FKTarget:     fkTarget,
	}
}
