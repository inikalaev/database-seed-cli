package config

import "testing"

func TestMergePreservesUserMechanism(t *testing.T) {
	existing := &Config{
		Version:  1,
		Database: DatabaseSection{Dialect: "postgres", Schemas: []string{"public"}},
		Tables: map[string]*Table{
			"public.users": {
				Schema: "public", Name: "users", RowCount: 500,
				Columns: map[string]*ColumnSpec{
					"id":    {Mechanism: "pk_serial", Origin: "user"},
					"email": {Mechanism: "email", Params: map[string]any{"domain": "custom.co"}, Origin: "user"},
				},
			},
		},
	}
	incoming := &Config{
		Version:  1,
		Database: DatabaseSection{Dialect: "postgres", Schemas: []string{"public"}},
		Tables: map[string]*Table{
			"public.users": {
				Schema: "public", Name: "users", RowCount: 100,
				Columns: map[string]*ColumnSpec{
					"id":    {Mechanism: "pk_serial", Origin: "inferred"},
					"email": {Mechanism: "email", Origin: "inferred"},
					"phone": {Mechanism: "phone", Origin: "inferred"}, // new column
				},
			},
		},
	}
	merged := Merge(existing, incoming)
	users := merged.Tables["public.users"]
	if users.RowCount != 500 {
		t.Fatalf("row_count not preserved: got %d", users.RowCount)
	}
	if got := users.Columns["email"].Params["domain"]; got != "custom.co" {
		t.Fatalf("email params not preserved: %v", got)
	}
	if users.Columns["phone"] == nil {
		t.Fatalf("new column phone not added")
	}
}

func TestMergeMarksRemoved(t *testing.T) {
	existing := &Config{
		Version:  1,
		Database: DatabaseSection{Dialect: "postgres"},
		Tables: map[string]*Table{
			"public.old":   {Schema: "public", Name: "old", Columns: map[string]*ColumnSpec{"id": {Mechanism: "pk_serial", Origin: "user"}}},
			"public.users": {Schema: "public", Name: "users", Columns: map[string]*ColumnSpec{"id": {Mechanism: "pk_serial", Origin: "user"}, "gone": {Mechanism: "string", Origin: "user"}}},
		},
	}
	incoming := &Config{
		Version:  1,
		Database: DatabaseSection{Dialect: "postgres"},
		Tables: map[string]*Table{
			"public.users": {Schema: "public", Name: "users", Columns: map[string]*ColumnSpec{"id": {Mechanism: "pk_serial", Origin: "inferred"}}},
		},
	}
	merged := Merge(existing, incoming)
	if !merged.Tables["public.old"].Removed {
		t.Fatalf("removed table not flagged")
	}
	if !merged.Tables["public.users"].Columns["gone"].Removed {
		t.Fatalf("removed column not flagged")
	}
}

func TestRoundTripMarshal(t *testing.T) {
	in := &Config{
		Version:  1,
		Database: DatabaseSection{Dialect: "postgres", Schemas: []string{"public"}},
		Defaults: DefaultsSection{Locale: "ru_RU", Seed: 42},
		Tables: map[string]*Table{
			"public.users": {
				Schema: "public", Name: "users", RowCount: 10,
				Columns: map[string]*ColumnSpec{
					"id":    {Mechanism: "pk_serial", DataType: "integer"},
					"email": {Mechanism: "email", Params: map[string]any{"domain": "x.io"}, DataType: "text"},
				},
				PKOrder: []string{"id", "email"},
			},
		},
	}
	data, err := Marshal(in)
	if err != nil {
		t.Fatal(err)
	}
	back, err := Unmarshal(data)
	if err != nil {
		t.Fatalf("unmarshal: %v\n%s", err, data)
	}
	if back.Tables["public.users"].RowCount != 10 {
		t.Fatalf("lost row_count; got:\n%s", data)
	}
	if back.Tables["public.users"].Columns["email"].Mechanism != "email" {
		t.Fatalf("lost mechanism; got:\n%s", data)
	}
}
