// Package relations builds the FK dependency graph and row-count plan.
//
// It is the single source of truth about insert order and PK pool availability:
// CLI, wrappers, and the skill all rely on the same graph, so the package must
// remain self-contained and dialect-neutral.
package relations

import (
	"fmt"
	"sort"
	"strings"

	"github.com/inikalaev/database-seed-cli/internal/config"
	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

type TableRef struct {
	Schema string
	Name   string
}

func (r TableRef) Key() string { return r.Schema + "." + r.Name }

type Edge struct {
	From         TableRef
	To           TableRef
	Column       string
	TargetColumn string
	Deferrable   bool
	// Nullable is whether the FK column itself accepts NULL. Used by the
	// row-count planner to decide whether a parent's row_count=0 must
	// propagate to the child (NOT NULL FK with empty parent pool always
	// fails at apply time).
	Nullable bool
}

type Graph struct {
	Tables []TableRef
	Edges  []Edge
}

// Plan describes the insertion strategy: ordered batches of tables, plus any
// tables that participate in FK cycles (emitted together under DEFERRABLE).
type Plan struct {
	Order  []TableRef
	Cycles [][]TableRef
	// RowCounts[TableRef.Key()] = how many rows to generate.
	RowCounts map[string]int
	// Graph is the source graph so callers (validate) can inspect per-edge
	// deferrability within cycles without re-walking the config.
	Graph *Graph
	// CascadedFrom records tables zeroed automatically because a NOT NULL FK
	// pointed at a zero-count parent. Keyed by child table; values list every
	// (parent, column) pair that contributed. Surfaced by `generate` as a
	// warning so the user understands silent row-count changes.
	CascadedFrom map[string][]CascadeCause
}

func Build(cfg *config.Config) (*Graph, error) {
	g := &Graph{}
	// Iterate in sorted key order so graph construction is deterministic —
	// Tarjan's emit order depends on it.
	keys := make([]string, 0, len(cfg.Tables))
	for k := range cfg.Tables {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, key := range keys {
		t := cfg.Tables[key]
		if t.Removed {
			continue
		}
		ref := TableRef{Schema: t.Schema, Name: t.Name}
		if ref.Schema == "" || ref.Name == "" {
			return nil, fmt.Errorf("table key %q missing schema.table", key)
		}
		g.Tables = append(g.Tables, ref)
		cnames := make([]string, 0, len(t.Columns))
		for cname := range t.Columns {
			cnames = append(cnames, cname)
		}
		sort.Strings(cnames)
		for _, cname := range cnames {
			col := t.Columns[cname]
			if col.Removed || col.Factory != seedapi.FactoryFKRef {
				continue
			}
			target, _ := col.Params["target"].(string)
			if target == "" {
				continue
			}
			ts, tt, tc := splitFKTarget(target)
			if ts == "" || tt == "" {
				// Malformed target — skip silently; validate surfaces it as a
				// separate issue.
				continue
			}
			deferrable, _ := col.Params["deferrable"].(bool)
			g.Edges = append(g.Edges, Edge{
				From:         ref,
				To:           TableRef{Schema: ts, Name: tt},
				Column:       cname,
				TargetColumn: tc,
				Deferrable:   deferrable,
				Nullable:     col.Nullable,
			})
		}
		// Polymorphic pairs depend on each candidate table's PK pool, so
		// register them as FK edges. This keeps parents emitted before
		// children and surfaces cycles through the same Tarjan pass as real
		// FKs. Nullable: true — an empty candidate list or empty pool emits
		// NULL rather than failing.
		for _, pk := range t.Polymorphs {
			for _, cand := range pk.Candidates {
				cs, ct := splitTableKey(cand.Table)
				if ct == "" {
					continue
				}
				pkCol := cand.PkColumn
				if pkCol == "" {
					pkCol = "id"
				}
				g.Edges = append(g.Edges, Edge{
					From:         ref,
					To:           TableRef{Schema: cs, Name: ct},
					Column:       pk.IdColumn,
					TargetColumn: pkCol,
					Nullable:     true,
				})
			}
		}
	}
	return g, nil
}

func splitTableKey(qualified string) (string, string) {
	if i := strings.Index(qualified, "."); i >= 0 {
		return qualified[:i], qualified[i+1:]
	}
	return "public", qualified
}

// orderSCCByNonNullable performs a topological sort limited to one SCC
// using only non-nullable edges. Any edge that is nullable is ignored,
// which often collapses "A ↔ B via nullable pointer" cycles into a
// sensible order (the non-null direction wins). On true non-null cycles
// the Kahn sort can't make progress — fall back to the input order.
func orderSCCByNonNullable(members []TableRef, edges []Edge) []TableRef {
	inSet := map[string]bool{}
	for _, r := range members {
		inSet[r.Key()] = true
	}
	indeg := map[string]int{}
	adj := map[string][]string{}
	for _, e := range edges {
		if e.Nullable {
			continue
		}
		from, to := e.From.Key(), e.To.Key()
		if !inSet[from] || !inSet[to] || from == to {
			continue
		}
		// Edge means "From depends on To" — so To must come before From.
		adj[to] = append(adj[to], from)
		indeg[from]++
	}
	// Kahn's algorithm.
	queue := make([]string, 0, len(members))
	byKey := map[string]TableRef{}
	for _, r := range members {
		byKey[r.Key()] = r
		if indeg[r.Key()] == 0 {
			queue = append(queue, r.Key())
		}
	}
	sort.Strings(queue) // deterministic output for ties
	out := make([]TableRef, 0, len(members))
	seen := map[string]bool{}
	for len(queue) > 0 {
		k := queue[0]
		queue = queue[1:]
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, byKey[k])
		next := append([]string(nil), adj[k]...)
		sort.Strings(next)
		for _, w := range next {
			indeg[w]--
			if indeg[w] == 0 {
				queue = append(queue, w)
			}
		}
	}
	if len(out) != len(members) {
		// Non-null cycle couldn't be topologically flattened. Keep original
		// Tarjan order — the downstream apply will need DEFERRED to work.
		return members
	}
	return out
}

func splitFKTarget(s string) (string, string, string) {
	parts := strings.Split(s, ".")
	switch len(parts) {
	case 3:
		return parts[0], parts[1], parts[2]
	case 2:
		return "public", parts[0], parts[1]
	}
	return "", "", ""
}

// PlanFor resolves insert order with Tarjan's SCC algorithm. Each SCC with
// more than one table becomes a cycle; within each cycle tables are
// re-ordered by dropping nullable FK edges, which turns cycles like
// "users ↔ workplaces via nullable pointer" into a directed order (the
// non-null edge wins). The nullable column emits NULL and may be updated
// after the fact by the user.
func (g *Graph) PlanFor(cfg *config.Config) *Plan {
	adj := map[string][]string{}
	for _, e := range g.Edges {
		adj[e.From.Key()] = append(adj[e.From.Key()], e.To.Key())
	}
	ids := map[string]int{}
	for i, t := range g.Tables {
		ids[t.Key()] = i
	}

	// Tarjan
	type state struct {
		idx, low int
		onStack  bool
	}
	states := map[string]*state{}
	var stack []string
	next := 0
	var sccs [][]string

	var strong func(v string)
	strong = func(v string) {
		states[v] = &state{idx: next, low: next, onStack: true}
		next++
		stack = append(stack, v)
		for _, w := range adj[v] {
			if _, seen := states[w]; !seen {
				if _, known := ids[w]; !known {
					continue
				}
				strong(w)
				if states[w].low < states[v].low {
					states[v].low = states[w].low
				}
			} else if states[w].onStack {
				if states[w].idx < states[v].low {
					states[v].low = states[w].idx
				}
			}
		}
		if states[v].low == states[v].idx {
			var scc []string
			for {
				w := stack[len(stack)-1]
				stack = stack[:len(stack)-1]
				states[w].onStack = false
				scc = append(scc, w)
				if w == v {
					break
				}
			}
			sccs = append(sccs, scc)
		}
	}
	for _, t := range g.Tables {
		if _, seen := states[t.Key()]; !seen {
			strong(t.Key())
		}
	}

	// Tarjan emits SCCs in reverse dependency order (leaves first), which is
	// exactly the insert order we want: parents before children.
	order := make([]TableRef, 0, len(g.Tables))
	var cycles [][]TableRef
	for _, scc := range sccs {
		refs := make([]TableRef, 0, len(scc))
		for _, k := range scc {
			parts := strings.SplitN(k, ".", 2)
			refs = append(refs, TableRef{Schema: parts[0], Name: parts[1]})
		}
		if len(refs) > 1 {
			cycles = append(cycles, refs)
			// Break the cycle into a DAG by dropping nullable edges. The
			// nullable side can emit NULL safely, so the non-null side
			// drives the insert order within the SCC. Pure non-null cycles
			// are untouched and still need DEFERRED to apply.
			refs = orderSCCByNonNullable(refs, g.Edges)
		}
		order = append(order, refs...)
	}

	// Row counts: simple model — honor row_count; row_count_per overrides based on
	// parent count * midpoint of the [lo,hi] range. Resolve in topological order
	// (order returned by Tarjan is parents-before-children) so row_count_per
	// chains like A→B→C see populated parent counts.
	counts := map[string]int{}
	for key, t := range cfg.Tables {
		if t.Removed {
			continue
		}
		if t.RowCount != nil {
			counts[key] = *t.RowCount
		}
	}
	// Two passes: the second resolves row_count_per references inside SCC
	// cycles where the parent/child order within the cycle is arbitrary.
	// Stable schemas converge after one extra pass; non-converging chains in
	// cycles are best-effort and surface via validate.
	for pass := 0; pass < 2; pass++ {
		for _, ref := range order {
			key := ref.Key()
			t := cfg.Tables[key]
			if t == nil || t.Removed || len(t.RowCountPer) == 0 {
				continue
			}
			// Track that row_count_per was resolved even when the result is
			// zero — an explicit `[0,0]` means "no rows", not "fall back to
			// default row count".
			total := 0
			resolved := false
			for parent, rng := range t.RowCountPer {
				parentKey := parent
				if !strings.Contains(parentKey, ".") {
					parentKey = "public." + parentKey
				}
				parentCount, ok := counts[parentKey]
				if !ok {
					continue // parent not yet resolved; next pass will retry
				}
				resolved = true
				// Explicit [0,0] is a literal "no rows" — skip the min-1 clamp.
				if rng[0] == 0 && rng[1] == 0 {
					continue
				}
				mid := (rng[0] + rng[1]) / 2
				if mid < 1 {
					mid = 1
				}
				total += parentCount * mid
			}
			if resolved {
				counts[key] = total
			}
		}
	}

	plan := &Plan{Order: order, Cycles: cycles, RowCounts: counts, Graph: g}
	plan.cascadeZero()
	return plan
}

// cascadeZero propagates row_count=0 from parent tables to children with a
// NOT NULL FK. A child with a non-nullable FK can never emit a valid row when
// its parent pool is empty, so the apply would fail at the first attempted
// INSERT. Iterated to a fixed point — the parent→child relation is a DAG
// after SCC collapse, so termination is guaranteed in O(tables * edges).
func (p *Plan) cascadeZero() {
	if p.Graph == nil {
		return
	}
	zeroed := map[string]bool{}
	for k, n := range p.RowCounts {
		if n == 0 {
			zeroed[k] = true
		}
	}
	for {
		changed := false
		for _, e := range p.Graph.Edges {
			if e.Nullable {
				continue
			}
			if !zeroed[e.To.Key()] {
				continue
			}
			from := e.From.Key()
			if zeroed[from] {
				continue
			}
			if _, ok := p.RowCounts[from]; !ok {
				// Child table not in plan (removed / schema-only); skip.
				continue
			}
			p.RowCounts[from] = 0
			p.CascadedFrom = appendCause(p.CascadedFrom, from, e.To.Key(), e.Column)
			zeroed[from] = true
			changed = true
		}
		if !changed {
			return
		}
	}
}

// CascadeCause records why a table was zeroed during cascadeZero — the
// triggering parent and the NOT NULL FK column responsible. Surfaces via the
// generate command so the user sees every table that got silently zeroed.
type CascadeCause struct {
	Parent string
	Column string
}

func appendCause(m map[string][]CascadeCause, child, parent, col string) map[string][]CascadeCause {
	if m == nil {
		m = map[string][]CascadeCause{}
	}
	m[child] = append(m[child], CascadeCause{Parent: parent, Column: col})
	return m
}

// NonDeferrableEdgesIn returns edges inside a cycle whose FK isn't DEFERRABLE.
// sqlemit's `SET CONSTRAINTS ALL DEFERRED` only affects deferrable constraints,
// so a cycle with a non-deferrable edge breaks at apply time.
func (p *Plan) NonDeferrableEdgesIn(cycle []TableRef) []Edge {
	if p.Graph == nil {
		return nil
	}
	inCycle := map[string]bool{}
	for _, r := range cycle {
		inCycle[r.Key()] = true
	}
	var out []Edge
	for _, e := range p.Graph.Edges {
		if inCycle[e.From.Key()] && inCycle[e.To.Key()] && !e.Deferrable {
			out = append(out, e)
		}
	}
	return out
}
