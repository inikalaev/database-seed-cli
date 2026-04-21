// Package seedapi is the public contract consumed by user-written mechanisms.
//
// Users drop Go files into a directory referenced via `seed generate --factories ./dir`.
// Each file registers mechanisms from its init() using Register. The CLI wraps the
// directory into a throwaway module, re-compiles, and runs the binary — so this
// package is the single import surface that external code may depend on.
package seedapi

import (
	"math/rand/v2"
	"sync"
)

// Column mirrors schema.Column but lives here to keep the public API free of
// internal imports. The CLI converts between the two.
type Column struct {
	Schema       string
	Table        string
	Name         string
	DataType     string
	UDTName      string
	Nullable     bool
	ArrayDims    int
	EnumName     string
	EnumValues   []string
	CharMaxLen   *int
	NumPrecision *int
	NumScale     *int
	FKTarget     string // "schema.table.column" if this column is a FK, else empty.
}

// MatchScore is the output of inference: higher = more confident.
// Convention: 0 = no match, 1–49 = weak (fallback), 50–89 = typical, 90+ = strong.
// WeakNameMatch is the resolved/unresolved threshold: any score >= WeakNameMatch
// is considered confident enough to omit unresolved: true. Generic type-only
// builtins (bool/integer/decimal/date/hstore, timestamp by name pattern) return
// WeakNameMatch so that any plugin returning NameMatch beats them.
type MatchScore int

const (
	NoMatch       MatchScore = 0
	WeakMatch     MatchScore = 10
	TypeMatch     MatchScore = 40
	WeakNameMatch MatchScore = 60
	NameMatch     MatchScore = 70
	StrongMatch   MatchScore = 90
	FKMatch       MatchScore = 100 // FK reference — never override without explicit user intent.
)

// FactoryFKRef is the registered name of the built-in FK-reference factory.
// relations.Build and other layers recognise this factory as the signal that a
// column sources its value from the FK pool.
const FactoryFKRef = "fkref"

// FactoryEnumValue is the registered name of the built-in enum sampler factory.
const FactoryEnumValue = "enum_value"

// Cast wraps a value with an explicit Postgres type cast. Use it when a literal
// needs `::type` appended in the emitted SQL — e.g. `'{}'::jsonb` as an element
// inside `ARRAY[...]` where the array factory can't otherwise signal the
// element type. `Value` is formatted by the normal literal rules, then
// `::Type` is appended.
type Cast struct {
	Value any
	Type  string
}

// Params carries user-provided configuration for a mechanism (from YAML `params`).
type Params map[string]any

func (p Params) String(key, def string) string {
	if v, ok := p[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return def
}

func (p Params) Int(key string, def int) int {
	if v, ok := p[key]; ok {
		switch n := v.(type) {
		case int:
			return n
		case int64:
			return int(n)
		case float64:
			return int(n)
		}
	}
	return def
}

func (p Params) Float(key string, def float64) float64 {
	if v, ok := p[key]; ok {
		switch n := v.(type) {
		case float64:
			return n
		case int:
			return float64(n)
		case int64:
			return float64(n)
		}
	}
	return def
}

type MatchContext struct {
	Column Column
	// Locale from config.defaults, e.g. "ru_RU".
	Locale string
}

type GenContext struct {
	Column Column
	Row    int
	Rng    *rand.Rand
	Params Params
	// FKPool provides primary-key values previously generated for referenced tables.
	FKPool FKPool
	Locale string
}

type FKPool interface {
	Pick(schema, table, column string, rng *rand.Rand) (any, bool)
}

// Factory is the interface user code implements.
// Tags() doubles as column-name patterns for auto-matching: the registry
// scores Name() as StrongMatch and each tag as NameMatch (substring,
// case-insensitive, underscores/hyphens stripped).
// Implement Matcher to override auto-matching with custom scoring logic.
type Factory interface {
	Name() string
	Tags() []string
	Generate(ctx GenContext) any
}

// Matcher is an optional interface a Factory can implement to override
// the default name/tag-based inference.
type Matcher interface {
	Match(ctx MatchContext) MatchScore
}

// UniqueGenerator may be implemented by a Factory to declare that its Generate
// output is guaranteed unique across all rows within a single Emit call.
// validate uses this to suppress false warnings on UNIQUE-constrained columns.
type UniqueGenerator interface {
	Factory
	UniquePerRow() bool
}

// Registry collects mechanisms discovered at init() time.
type Registry struct {
	mu      sync.RWMutex
	byName  map[string]Factory
	ordered []Factory
}

var global = &Registry{byName: map[string]Factory{}}

// Register adds a factory to the global registry. First registration wins —
// later duplicates by Name() are silently dropped, matching registry.New. This
// lets a plugin file be accidentally imported twice without crashing the CLI.
func Register(m Factory) {
	global.mu.Lock()
	defer global.mu.Unlock()
	if _, exists := global.byName[m.Name()]; exists {
		return
	}
	global.byName[m.Name()] = m
	global.ordered = append(global.ordered, m)
}

func Default() *Registry { return global }

func (r *Registry) All() []Factory {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Factory, len(r.ordered))
	copy(out, r.ordered)
	return out
}

func (r *Registry) Get(name string) (Factory, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.byName[name]
	return m, ok
}
