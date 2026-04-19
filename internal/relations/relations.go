// Package relations builds the FK dependency graph and row-count plan.
//
// It is the single source of truth about insert order and PK pool availability:
// CLI, wrappers, and the skill all rely on the same graph, so the package must
// remain self-contained and dialect-neutral.
package relations

import (
	"fmt"
	"strings"

	"github.com/ivannikolaev/seed-cli/cli/internal/config"
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
}

func Build(cfg *config.Config) (*Graph, error) {
	g := &Graph{}
	for key, t := range cfg.Tables {
		if t.Removed {
			continue
		}
		ref := TableRef{Schema: t.Schema, Name: t.Name}
		if ref.Schema == "" || ref.Name == "" {
			return nil, fmt.Errorf("table key %q missing schema.table", key)
		}
		g.Tables = append(g.Tables, ref)
		for cname, col := range t.Columns {
			if col.Removed || col.Mechanism != "fkref" {
				continue
			}
			target, _ := col.Params["target"].(string)
			if target == "" {
				continue
			}
			ts, tt, tc := splitFKTarget(target)
			g.Edges = append(g.Edges, Edge{
				From:         ref,
				To:           TableRef{Schema: ts, Name: tt},
				Column:       cname,
				TargetColumn: tc,
			})
		}
	}
	return g, nil
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

// PlanFor resolves insert order with Tarjan's SCC algorithm. Each SCC with more
// than one table becomes a cycle (all edges inside it are marked Deferrable).
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
		}
		order = append(order, refs...)
	}

	// Row counts: simple model — honor row_count; row_count_per overrides based on
	// parent count * midpoint of the [lo,hi] range.
	counts := map[string]int{}
	for key, t := range cfg.Tables {
		if t.Removed {
			continue
		}
		counts[key] = t.RowCount
	}
	for key, t := range cfg.Tables {
		if len(t.RowCountPer) == 0 {
			continue
		}
		total := 0
		for parent, rng := range t.RowCountPer {
			parentKey := parent
			if !strings.Contains(parentKey, ".") {
				parentKey = "public." + parentKey
			}
			mid := (rng[0] + rng[1]) / 2
			if mid < 1 {
				mid = 1
			}
			total += counts[parentKey] * mid
		}
		if total > 0 {
			counts[key] = total
		}
	}

	return &Plan{Order: order, Cycles: cycles, RowCounts: counts}
}
