package config

import "testing"

func intp(v int) *int { return &v }

func TestMergePreservesUserMechanism(t *testing.T) {
	existing := &Config{
		Version:  1,
		Database: DatabaseSection{Dialect: "postgres", Schemas: []string{"public"}},
		Tables: map[string]*Table{
			"public.users": {
				Schema: "public", Name: "users", RowCount: intp(500),
				Columns: map[string]*ColumnSpec{
					"id":    {Factory: "pk_serial"},
					"email": {Factory: "email", Params: map[string]any{"domain": "custom.co"}},
				},
			},
		},
	}
	incoming := &Config{
		Version:  1,
		Database: DatabaseSection{Dialect: "postgres", Schemas: []string{"public"}},
		Tables: map[string]*Table{
			"public.users": {
				Schema: "public", Name: "users", RowCount: intp(100),
				Columns: map[string]*ColumnSpec{
					"id":    {Factory: "pk_serial"},
					"email": {Factory: "email"},
					"phone": {Factory: "phone"}, // new column
				},
			},
		},
	}
	merged := Merge(existing, incoming)
	users := merged.Tables["public.users"]
	if users.RowCount == nil || *users.RowCount != 500 {
		t.Fatalf("row_count not preserved: got %v", users.RowCount)
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
			"public.old":   {Schema: "public", Name: "old", Columns: map[string]*ColumnSpec{"id": {Factory: "pk_serial"}}},
			"public.users": {Schema: "public", Name: "users", Columns: map[string]*ColumnSpec{"id": {Factory: "pk_serial"}, "gone": {Factory: "string"}}},
		},
	}
	incoming := &Config{
		Version:  1,
		Database: DatabaseSection{Dialect: "postgres"},
		Tables: map[string]*Table{
			"public.users": {Schema: "public", Name: "users", Columns: map[string]*ColumnSpec{"id": {Factory: "pk_serial"}}},
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

func TestMergeIsIdempotent(t *testing.T) {
	// Merge(Merge(a,b), b) must equal Merge(a,b): re-syncing against the same
	// incoming data is a no-op.
	existing := &Config{
		Version:  1,
		Database: DatabaseSection{Dialect: "postgres"},
		Tables: map[string]*Table{
			"public.users": {
				Schema: "public", Name: "users", RowCount: intp(500),
				PrimaryKey: []string{"id"},
				Columns: map[string]*ColumnSpec{
					"id":    {Factory: "pk_serial", DataType: "integer"},
					"email": {Factory: "email", Params: map[string]any{"domain": "custom.co"}, DataType: "text"},
				},
			},
		},
	}
	incoming := &Config{
		Version:  1,
		Database: DatabaseSection{Dialect: "postgres"},
		Tables: map[string]*Table{
			"public.users": {
				Schema: "public", Name: "users", RowCount: intp(100),
				PrimaryKey: []string{"id"},
				Columns: map[string]*ColumnSpec{
					"id":    {Factory: "pk_serial", DataType: "integer"},
					"email": {Factory: "email", DataType: "text"},
				},
			},
		},
	}
	first := Merge(existing, incoming)
	second := Merge(first, incoming)
	rc1, rc2 := first.Tables["public.users"].RowCount, second.Tables["public.users"].RowCount
	if rc1 == nil || rc2 == nil || *rc1 != *rc2 {
		t.Fatalf("row_count drift across re-merge: %v → %v", rc1, rc2)
	}
	if first.Tables["public.users"].Columns["email"].Params["domain"] !=
		second.Tables["public.users"].Columns["email"].Params["domain"] {
		t.Fatalf("params drift across re-merge")
	}
}

func TestMergeRowCountZeroIsPreserved(t *testing.T) {
	// row_count=0 is a user-authored "do not generate" — merge must not reset
	// it to the incoming default.
	zero := 0
	existing := &Config{
		Tables: map[string]*Table{
			"public.logs": {Schema: "public", Name: "logs", RowCount: &zero,
				Columns: map[string]*ColumnSpec{"id": {Factory: "pk_serial"}}},
		},
	}
	incoming := &Config{
		Tables: map[string]*Table{
			"public.logs": {Schema: "public", Name: "logs", RowCount: intp(100),
				Columns: map[string]*ColumnSpec{"id": {Factory: "pk_serial"}}},
		},
	}
	merged := Merge(existing, incoming)
	rc := merged.Tables["public.logs"].RowCount
	if rc == nil || *rc != 0 {
		t.Fatalf("row_count=0 not preserved: %v", rc)
	}
}

func TestMergePreservesValues(t *testing.T) {
	existing := &Config{
		Tables: map[string]*Table{
			"public.users": {Schema: "public", Name: "users", Columns: map[string]*ColumnSpec{
				"meta": {Factory: "json_any", Values: map[string]*ColumnSpec{
					"city": {Factory: "city"},
				}},
			}},
		},
	}
	incoming := &Config{
		Tables: map[string]*Table{
			"public.users": {Schema: "public", Name: "users", Columns: map[string]*ColumnSpec{
				"meta": {Factory: "json_any", DataType: "jsonb"},
			}},
		},
	}
	merged := Merge(existing, incoming)
	meta := merged.Tables["public.users"].Columns["meta"]
	if len(meta.Values) == 0 || meta.Values["city"] == nil {
		t.Fatalf("nested values not preserved: %+v", meta)
	}
	if meta.DataType != "jsonb" {
		t.Fatalf("schema-derived data_type not refreshed: %q", meta.DataType)
	}
}

func TestRoundTripPreservesValues(t *testing.T) {
	in := &Config{
		Version:  1,
		Database: DatabaseSection{Dialect: "postgres", Schemas: []string{"public"}},
		Tables: map[string]*Table{
			"public.users": {
				Schema: "public", Name: "users", RowCount: intp(1),
				Columns: map[string]*ColumnSpec{
					"meta": {Factory: "json_any", DataType: "jsonb", Values: map[string]*ColumnSpec{
						"city":   {Factory: "city"},
						"nested": {Factory: "json_any", Values: map[string]*ColumnSpec{"x": {Factory: "integer"}}},
					}},
				},
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
	meta := back.Tables["public.users"].Columns["meta"]
	if meta.Values["city"] == nil {
		t.Fatalf("lost meta.city:\n%s", data)
	}
	if meta.Values["nested"] == nil || meta.Values["nested"].Values["x"] == nil {
		t.Fatalf("lost deep nested values:\n%s", data)
	}
}

func TestRoundTripPreservesStringValueTypes(t *testing.T) {
	// Regression: user-authored `value: "42"` / `value: "true"` must survive
	// the YAML round-trip as strings — earlier scalarNode coerced them to
	// int/bool on reload.
	in := &Config{
		Version:  1,
		Database: DatabaseSection{Dialect: "postgres"},
		Tables: map[string]*Table{
			"public.t": {
				Schema: "public", Name: "t", RowCount: intp(1),
				Columns: map[string]*ColumnSpec{
					"a": {Value: "42"},
					"b": {Value: "true"},
					"c": {Value: "null"},
					"d": {Value: 42}, // real int should stay int
				},
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
	cols := back.Tables["public.t"].Columns
	for _, k := range []string{"a", "b", "c"} {
		if _, ok := cols[k].Value.(string); !ok {
			t.Fatalf("value %q lost string type, got %T (%v)\n%s", k, cols[k].Value, cols[k].Value, data)
		}
	}
	if _, ok := cols["d"].Value.(int); !ok {
		// yaml.v3 may decode into int or int64 — accept either as numeric.
		if _, ok2 := cols["d"].Value.(int64); !ok2 {
			t.Fatalf("int value lost numeric type: %T\n%s", cols["d"].Value, data)
		}
	}
}

func TestMergeRefreshesSchemaDerivedParams(t *testing.T) {
	// enum labels are schema-derived params["values"]. When the schema adds
	// a new label, merge must pick it up even if existing has its own params.
	existing := &Config{
		Tables: map[string]*Table{
			"public.t": {Schema: "public", Name: "t", Columns: map[string]*ColumnSpec{
				"status": {
					Factory: "enum_value",
					Params: map[string]any{
						"values":  []any{"active", "inactive"}, // stale
						"weights": []any{1, 1},                 // user-added, must survive
					},
				},
			}},
		},
	}
	incoming := &Config{
		Tables: map[string]*Table{
			"public.t": {Schema: "public", Name: "t", Columns: map[string]*ColumnSpec{
				"status": {
					Factory: "enum_value",
					Params:  map[string]any{"values": []any{"active", "inactive", "archived"}},
				},
			}},
		},
	}
	merged := Merge(existing, incoming)
	params := merged.Tables["public.t"].Columns["status"].Params
	vals, _ := params["values"].([]any)
	if len(vals) != 3 {
		t.Fatalf("enum values not refreshed: %v", params["values"])
	}
	if _, ok := params["weights"]; !ok {
		t.Fatalf("user-added params['weights'] dropped: %v", params)
	}
}

func TestRoundTripMarshal(t *testing.T) {
	in := &Config{
		Version:  1,
		Database: DatabaseSection{Dialect: "postgres", Schemas: []string{"public"}},
		Defaults: DefaultsSection{Locale: "ru_RU", Seed: 42},
		Tables: map[string]*Table{
			"public.users": {
				Schema: "public", Name: "users", RowCount: intp(10),
				Columns: map[string]*ColumnSpec{
					"id":    {Factory: "pk_serial", DataType: "integer"},
					"email": {Factory: "email", Params: map[string]any{"domain": "x.io"}, DataType: "text"},
				},
				ColumnOrder: []string{"id", "email"},
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
	if rc := back.Tables["public.users"].RowCount; rc == nil || *rc != 10 {
		t.Fatalf("lost row_count; got:\n%s", data)
	}
	if back.Tables["public.users"].Columns["email"].Factory != "email" {
		t.Fatalf("lost mechanism; got:\n%s", data)
	}
}
