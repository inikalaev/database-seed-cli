// Package seedapi is the public contract consumed by user-written mechanisms.
//
// Users drop Go files into a directory referenced via `seed generate --generators ./dir`.
// Each file registers mechanisms from its init() using Register. The CLI wraps the
// directory into a throwaway module, re-compiles, and runs the binary — so this
// package is the single import surface that external code may depend on.
package seedapi

import (
	"fmt"
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
type MatchScore int

const (
	NoMatch      MatchScore = 0
	WeakMatch    MatchScore = 10
	TypeMatch    MatchScore = 40
	NameMatch    MatchScore = 70
	StrongMatch  MatchScore = 90
	FKMatch      MatchScore = 100 // FK reference — never override without explicit user intent.
)

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

// Mechanism is the interface user code implements.
type Mechanism interface {
	Name() string
	Tags() []string
	Match(ctx MatchContext) MatchScore
	Generate(ctx GenContext) any
}

// Registry collects mechanisms discovered at init() time.
type Registry struct {
	mu       sync.RWMutex
	byName   map[string]Mechanism
	ordered  []Mechanism
}

var global = &Registry{byName: map[string]Mechanism{}}

func Register(m Mechanism) {
	global.mu.Lock()
	defer global.mu.Unlock()
	if _, exists := global.byName[m.Name()]; exists {
		panic(fmt.Sprintf("seedapi: mechanism %q registered twice", m.Name()))
	}
	global.byName[m.Name()] = m
	global.ordered = append(global.ordered, m)
}

func Default() *Registry { return global }

func (r *Registry) All() []Mechanism {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Mechanism, len(r.ordered))
	copy(out, r.ordered)
	return out
}

func (r *Registry) Get(name string) (Mechanism, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	m, ok := r.byName[name]
	return m, ok
}
