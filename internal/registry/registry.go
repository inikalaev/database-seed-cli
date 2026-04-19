// Package registry houses the mechanism dispatch layer.
//
// It wraps seedapi.Registry with inference helpers that pick the best-matching
// mechanism for a given column, and with glue that converts schema.Column into
// seedapi.Column for the scoring call.
package registry

import (
	"github.com/ivannikolaev/seed-cli/cli/internal/schema"
	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type Registry struct {
	ordered []seedapi.Mechanism
	byName  map[string]seedapi.Mechanism
}

func New(mechanisms []seedapi.Mechanism) *Registry {
	r := &Registry{byName: map[string]seedapi.Mechanism{}}
	for _, m := range mechanisms {
		r.ordered = append(r.ordered, m)
		r.byName[m.Name()] = m
	}
	return r
}

func (r *Registry) Get(name string) (seedapi.Mechanism, bool) {
	m, ok := r.byName[name]
	return m, ok
}

func (r *Registry) All() []seedapi.Mechanism { return r.ordered }

// InferenceResult carries the best mechanism and its confidence.
type InferenceResult struct {
	Mechanism  seedapi.Mechanism
	Score      seedapi.MatchScore
	Unresolved bool
}

// Infer picks the highest-scoring mechanism for the column, ties broken by
// registration order (i.e. the order in which mechanisms.All() returns them).
func (r *Registry) Infer(col seedapi.Column, locale string) InferenceResult {
	ctx := seedapi.MatchContext{Column: col, Locale: locale}
	var best seedapi.Mechanism
	var bestScore seedapi.MatchScore
	for _, m := range r.ordered {
		s := m.Match(ctx)
		if s > bestScore {
			best = m
			bestScore = s
		}
	}
	res := InferenceResult{Mechanism: best, Score: bestScore}
	// If nothing scored, mark unresolved and fall back to a type-derived placeholder
	// chosen by the caller (config layer decides the fallback name).
	if best == nil || bestScore == seedapi.NoMatch {
		res.Unresolved = true
	} else if bestScore < seedapi.TypeMatch {
		// Weak-match columns are still unresolved: the generic string fallback is
		// technically a match, but we want the user to review.
		res.Unresolved = true
	}
	return res
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
