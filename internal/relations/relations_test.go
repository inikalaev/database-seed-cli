package relations

import (
	"testing"

	"github.com/inikalaev/database-seed-cli/internal/config"
)

func mkCol(mech string, params map[string]any) *config.ColumnSpec {
	return &config.ColumnSpec{Factory: mech, Params: params}
}

func intp(v int) *int { return &v }

func TestPlanOrdersParentsFirst(t *testing.T) {
	cfg := &config.Config{
		Tables: map[string]*config.Table{
			"public.users": {Schema: "public", Name: "users", RowCount: intp(10), Columns: map[string]*config.ColumnSpec{
				"id": mkCol("pk_serial", nil),
			}},
			"public.orders": {Schema: "public", Name: "orders", RowCount: intp(30), Columns: map[string]*config.ColumnSpec{
				"id":      mkCol("pk_serial", nil),
				"user_id": mkCol("fkref", map[string]any{"target": "public.users.id"}),
			}},
		},
	}
	g, err := Build(cfg)
	if err != nil {
		t.Fatal(err)
	}
	plan := g.PlanFor(cfg)
	if len(plan.Order) != 2 {
		t.Fatalf("want 2 tables in order, got %d", len(plan.Order))
	}
	if plan.Order[0].Name != "users" {
		t.Fatalf("users must come before orders, got order[0]=%s", plan.Order[0].Name)
	}
	if len(plan.Cycles) != 0 {
		t.Fatalf("no cycles expected")
	}
}

func TestPlanRowCountPerChainsInTopoOrder(t *testing.T) {
	// C depends on B which depends on A. With random map iteration, C's count
	// can be computed before B's, leaving it at 0. Topological resolution
	// guarantees parents settle first.
	cfg := &config.Config{
		Tables: map[string]*config.Table{
			"public.a": {Schema: "public", Name: "a", RowCount: intp(10), Columns: map[string]*config.ColumnSpec{
				"id": mkCol("pk_serial", nil),
			}},
			"public.b": {Schema: "public", Name: "b", RowCountPer: map[string][2]int{"public.a": {2, 2}}, Columns: map[string]*config.ColumnSpec{
				"id":   mkCol("pk_serial", nil),
				"a_id": mkCol("fkref", map[string]any{"target": "public.a.id"}),
			}},
			"public.c": {Schema: "public", Name: "c", RowCountPer: map[string][2]int{"public.b": {3, 3}}, Columns: map[string]*config.ColumnSpec{
				"id":   mkCol("pk_serial", nil),
				"b_id": mkCol("fkref", map[string]any{"target": "public.b.id"}),
			}},
		},
	}
	g, _ := Build(cfg)
	plan := g.PlanFor(cfg)
	if plan.RowCounts["public.a"] != 10 {
		t.Fatalf("a: want 10, got %d", plan.RowCounts["public.a"])
	}
	if plan.RowCounts["public.b"] != 20 {
		t.Fatalf("b: want 10*2=20, got %d", plan.RowCounts["public.b"])
	}
	if plan.RowCounts["public.c"] != 60 {
		t.Fatalf("c: want 20*3=60, got %d", plan.RowCounts["public.c"])
	}
}

func TestPlanDetectsCycle(t *testing.T) {
	cfg := &config.Config{
		Tables: map[string]*config.Table{
			"public.a": {Schema: "public", Name: "a", Columns: map[string]*config.ColumnSpec{
				"b_id": mkCol("fkref", map[string]any{"target": "public.b.id"}),
			}},
			"public.b": {Schema: "public", Name: "b", Columns: map[string]*config.ColumnSpec{
				"a_id": mkCol("fkref", map[string]any{"target": "public.a.id"}),
			}},
		},
	}
	g, _ := Build(cfg)
	plan := g.PlanFor(cfg)
	if len(plan.Cycles) != 1 {
		t.Fatalf("want 1 cycle, got %d", len(plan.Cycles))
	}
}

func TestNonDeferrableEdgesIn(t *testing.T) {
	cfg := &config.Config{
		Tables: map[string]*config.Table{
			"public.a": {Schema: "public", Name: "a", Columns: map[string]*config.ColumnSpec{
				"b_id": mkCol("fkref", map[string]any{"target": "public.b.id"}), // NOT deferrable
			}},
			"public.b": {Schema: "public", Name: "b", Columns: map[string]*config.ColumnSpec{
				"a_id": mkCol("fkref", map[string]any{"target": "public.a.id", "deferrable": true}),
			}},
		},
	}
	g, _ := Build(cfg)
	plan := g.PlanFor(cfg)
	if len(plan.Cycles) != 1 {
		t.Fatalf("expected 1 cycle")
	}
	bad := plan.NonDeferrableEdgesIn(plan.Cycles[0])
	if len(bad) != 1 {
		t.Fatalf("want 1 non-deferrable edge, got %d: %+v", len(bad), bad)
	}
	if bad[0].From.Name != "a" || bad[0].Column != "b_id" {
		t.Fatalf("unexpected bad edge: %+v", bad[0])
	}
}
