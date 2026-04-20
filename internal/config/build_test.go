package config

import (
	"testing"

	"github.com/ivannikolaev/seed-cli/cli/internal/factories"
	"github.com/ivannikolaev/seed-cli/cli/internal/registry"
	"github.com/ivannikolaev/seed-cli/cli/internal/schema"
	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

func TestFromModelEnumColumn(t *testing.T) {
	// Verifies that enum columns are assigned the enum_value factory and carry
	// the enum labels in params["values"]. This requires col.EnumName to use the
	// qualified "schema.name" format so the enumValues lookup in FromModel hits.
	m := &schema.Model{
		Dialect: "postgres",
		Schemas: []string{"public"},
		Enums: []schema.Enum{
			{Schema: "public", Name: "order_status", Values: []string{"pending", "paid", "shipped"}},
		},
		Tables: []schema.Table{
			{
				Schema:     "public",
				Name:       "orders",
				PrimaryKey: []string{"id"},
				Columns: []schema.Column{
					{Name: "id", DataType: "integer", Position: 1},
					{Name: "status", DataType: "USER-DEFINED", UDTName: "order_status",
						EnumName: "public.order_status", Position: 2},
				},
			},
		},
	}
	reg := registry.New(factories.All())
	cfg := FromModel(m, reg, DefaultsSection{})

	tbl := cfg.Tables["public.orders"]
	if tbl == nil {
		t.Fatal("public.orders not found in config")
	}

	statusCol, ok := tbl.Columns["status"]
	if !ok {
		t.Fatal("status column not found")
	}
	if statusCol.Factory != seedapi.FactoryEnumValue {
		t.Errorf("status factory = %q, want %q", statusCol.Factory, seedapi.FactoryEnumValue)
	}
	if statusCol.Unresolved {
		t.Error("status column should not be marked unresolved")
	}
	vals, ok := statusCol.Params["values"].([]any)
	if !ok || len(vals) != 3 {
		t.Errorf("status params[values] = %v, want 3 enum labels", statusCol.Params["values"])
	}
}

func TestFromModelDefaultRowCount(t *testing.T) {
	m := &schema.Model{
		Dialect: "postgres",
		Schemas: []string{"public"},
		Tables: []schema.Table{
			{Schema: "public", Name: "items", PrimaryKey: []string{"id"},
				Columns: []schema.Column{{Name: "id", DataType: "integer", Position: 1}}},
		},
	}
	reg := registry.New(factories.All())
	cfg := FromModel(m, reg, DefaultsSection{})
	tbl := cfg.Tables["public.items"]
	if tbl == nil {
		t.Fatal("public.items not found")
	}
	if tbl.RowCount == nil || *tbl.RowCount != defaultRowCount {
		t.Errorf("RowCount = %v, want %d", tbl.RowCount, defaultRowCount)
	}
}
