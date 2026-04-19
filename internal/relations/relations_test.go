package relations

import (
	"testing"

	"github.com/ivannikolaev/seed-cli/cli/internal/config"
)

func mkCol(mech string, params map[string]any) *config.ColumnSpec {
	return &config.ColumnSpec{Mechanism: mech, Params: params}
}

func TestPlanOrdersParentsFirst(t *testing.T) {
	cfg := &config.Config{
		Tables: map[string]*config.Table{
			"public.users": {Schema: "public", Name: "users", RowCount: 10, Columns: map[string]*config.ColumnSpec{
				"id": mkCol("pk_serial", nil),
			}},
			"public.orders": {Schema: "public", Name: "orders", RowCount: 30, Columns: map[string]*config.ColumnSpec{
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
