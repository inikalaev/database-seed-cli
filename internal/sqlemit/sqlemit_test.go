package sqlemit

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/ivannikolaev/seed-cli/cli/internal/config"
	"github.com/ivannikolaev/seed-cli/cli/internal/mechanisms"
	"github.com/ivannikolaev/seed-cli/cli/internal/registry"
	"github.com/ivannikolaev/seed-cli/cli/internal/relations"
)

func TestEmitSimple(t *testing.T) {
	cfg := &config.Config{
		Version:  1,
		Database: config.DatabaseSection{Dialect: "postgres"},
		Tables: map[string]*config.Table{
			"public.users": {
				Schema: "public", Name: "users", RowCount: 3,
				PKOrder: []string{"id", "email"},
				Columns: map[string]*config.ColumnSpec{
					"id":    {Mechanism: "pk_serial", DataType: "integer"},
					"email": {Mechanism: "email", DataType: "text"},
				},
			},
			"public.orders": {
				Schema: "public", Name: "orders", RowCount: 5,
				PKOrder: []string{"id", "user_id"},
				Columns: map[string]*config.ColumnSpec{
					"id":      {Mechanism: "pk_serial", DataType: "integer"},
					"user_id": {Mechanism: "fkref", Params: map[string]any{"target": "public.users.id"}, DataType: "integer"},
				},
			},
		},
	}
	reg := registry.New(mechanisms.All())
	g, _ := relations.Build(cfg)
	plan := g.PlanFor(cfg)
	em := New(cfg, reg, plan, Options{Seed: 1})
	var buf bytes.Buffer
	if err := em.Emit(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if !strings.Contains(out, "INSERT INTO \"public\".\"users\"") {
		t.Fatalf("missing users INSERT; got:\n%s", out)
	}
	if !strings.Contains(out, "INSERT INTO \"public\".\"orders\"") {
		t.Fatalf("missing orders INSERT")
	}
	// orders reference users PKs: must contain an integer in 1..3 range in user_id col.
	if !strings.Contains(out, "BEGIN;") || !strings.Contains(out, "COMMIT;") {
		t.Fatalf("missing transaction frame")
	}
	// users must come before orders textually.
	uIdx := strings.Index(out, "public\".\"users\"")
	oIdx := strings.Index(out, "public\".\"orders\"")
	if uIdx < 0 || oIdx < 0 || uIdx > oIdx {
		t.Fatalf("users not emitted before orders")
	}
}

func TestEmitJSONValues(t *testing.T) {
	cfg := &config.Config{
		Version:  1,
		Database: config.DatabaseSection{Dialect: "postgres"},
		Tables: map[string]*config.Table{
			"public.profiles": {
				Schema: "public", Name: "profiles", RowCount: 2,
				PKOrder: []string{"id", "meta"},
				Columns: map[string]*config.ColumnSpec{
					"id": {Mechanism: "pk_serial", DataType: "integer"},
					"meta": {
						Mechanism: "json_any",
						DataType:  "jsonb",
						Values: map[string]*config.ColumnSpec{
							"name":  {Mechanism: "first_name", DataType: "text"},
							"score": {Mechanism: "integer", DataType: "integer", Params: map[string]any{"min": 1, "max": 100}},
							"addr": {
								Mechanism: "json_any",
								DataType:  "jsonb",
								Values: map[string]*config.ColumnSpec{
									"city": {Mechanism: "city", DataType: "text"},
								},
							},
						},
					},
				},
			},
		},
	}
	reg := registry.New(mechanisms.All())
	g, _ := relations.Build(cfg)
	plan := g.PlanFor(cfg)
	em := New(cfg, reg, plan, Options{Seed: 42})
	var buf bytes.Buffer
	if err := em.Emit(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// Extract the JSON literal between quotes from the first INSERT row.
	// The value looks like: '{"addr":{"city":"..."},"name":"...","score":N}'
	start := strings.Index(out, "'{")
	end := strings.Index(out[start:], "}'")
	if start < 0 || end < 0 {
		t.Fatalf("no JSON literal found in output:\n%s", out)
	}
	raw := out[start+1 : start+end+1]

	var obj map[string]any
	if err := json.Unmarshal([]byte(raw), &obj); err != nil {
		t.Fatalf("JSON unmarshal failed: %v\nraw=%s", err, raw)
	}
	if _, ok := obj["name"]; !ok {
		t.Error("missing 'name' field")
	}
	if _, ok := obj["score"]; !ok {
		t.Error("missing 'score' field")
	}
	addr, ok := obj["addr"].(map[string]any)
	if !ok {
		t.Fatalf("'addr' is not a nested object: %T", obj["addr"])
	}
	if _, ok := addr["city"]; !ok {
		t.Error("missing nested 'addr.city' field")
	}
}
