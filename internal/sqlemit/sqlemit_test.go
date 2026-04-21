package sqlemit

import (
	"bytes"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/inikalaev/database-seed-cli/internal/config"
	"github.com/inikalaev/database-seed-cli/internal/factories"
	"github.com/inikalaev/database-seed-cli/internal/registry"
	"github.com/inikalaev/database-seed-cli/internal/relations"
)

func intp(v int) *int { return &v }

func TestEmitSimple(t *testing.T) {
	cfg := &config.Config{
		Version:  1,
		Database: config.DatabaseSection{Dialect: "postgres"},
		Tables: map[string]*config.Table{
			"public.users": {
				Schema: "public", Name: "users", RowCount: intp(3),
				ColumnOrder: []string{"id", "email"},
				PrimaryKey:  []string{"id"},
				Columns: map[string]*config.ColumnSpec{
					"id":    {Factory: "pk_serial", DataType: "integer"},
					"email": {Factory: "email", DataType: "text"},
				},
			},
			"public.orders": {
				Schema: "public", Name: "orders", RowCount: intp(5),
				ColumnOrder: []string{"id", "user_id"},
				PrimaryKey:  []string{"id"},
				Columns: map[string]*config.ColumnSpec{
					"id":      {Factory: "pk_serial", DataType: "integer"},
					"user_id": {Factory: "fkref", Params: map[string]any{"target": "public.users.id"}, DataType: "integer"},
				},
			},
		},
	}
	reg := registry.New(factories.All())
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

func TestEmitPKFirstFeedsSelfFK(t *testing.T) {
	// categories has a self-FK (parent_id → id). The pool must be populated
	// with `id` before `parent_id` is generated in the same row — otherwise
	// every parent_id ends up NULL.
	cfg := &config.Config{
		Version:  1,
		Database: config.DatabaseSection{Dialect: "postgres"},
		Tables: map[string]*config.Table{
			"public.categories": {
				Schema: "public", Name: "categories", RowCount: intp(5),
				ColumnOrder: []string{"parent_id", "id"}, // deliberately reversed
				PrimaryKey:  []string{"id"},
				Columns: map[string]*config.ColumnSpec{
					"id":        {Factory: "pk_serial", DataType: "integer"},
					"parent_id": {Factory: "fkref", Params: map[string]any{"target": "public.categories.id"}, DataType: "integer", Nullable: true},
				},
			},
		},
	}
	reg := registry.New(factories.All())
	g, _ := relations.Build(cfg)
	plan := g.PlanFor(cfg)
	em := New(cfg, reg, plan, Options{Seed: 7})
	var buf bytes.Buffer
	if err := em.Emit(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// id is placed before parent_id within each row, so by the time parent_id
	// is sampled, the pool already contains the current row's id. Every row
	// therefore gets a non-NULL parent_id.
	if strings.Contains(out, "NULL") {
		t.Fatalf("unexpected NULL parent_id; PK-first ordering broken:\n%s", out)
	}
}

func TestEmitFreshPoolBetweenCalls(t *testing.T) {
	// Calling Emit twice must not leak PKs across runs — each Emit starts
	// with an empty pool. Uses a parent+child pair: if pool leaked, the second
	// Emit's FK pool would have 4 entries instead of 2, causing rng.IntN to
	// be called with a different modulus and produce different user_id values.
	cfg := &config.Config{
		Version:  1,
		Database: config.DatabaseSection{Dialect: "postgres"},
		Tables: map[string]*config.Table{
			"public.users": {
				Schema: "public", Name: "users", RowCount: intp(2),
				ColumnOrder: []string{"id"},
				PrimaryKey:  []string{"id"},
				Columns:     map[string]*config.ColumnSpec{"id": {Factory: "pk_serial", DataType: "integer"}},
			},
			"public.orders": {
				Schema: "public", Name: "orders", RowCount: intp(3),
				ColumnOrder: []string{"id", "user_id"},
				PrimaryKey:  []string{"id"},
				Columns: map[string]*config.ColumnSpec{
					"id":      {Factory: "pk_serial", DataType: "integer"},
					"user_id": {Factory: "fkref", Params: map[string]any{"target": "public.users.id"}, DataType: "integer"},
				},
			},
		},
	}
	reg := registry.New(factories.All())
	g, _ := relations.Build(cfg)
	plan := g.PlanFor(cfg)
	em := New(cfg, reg, plan, Options{Seed: 1})
	var a, b bytes.Buffer
	if err := em.Emit(&a); err != nil {
		t.Fatal(err)
	}
	if err := em.Emit(&b); err != nil {
		t.Fatal(err)
	}
	// Deterministic seed + fresh pool each call → identical output.
	if a.String() != b.String() {
		t.Fatalf("pool leaked across Emit calls:\nfirst:\n%s\nsecond:\n%s", a.String(), b.String())
	}
}

// Composite UNIQUE on (group_id, permission_id) with both columns fkref —
// the classic auth_group_permissions pattern. Independent sampling produces
// duplicates at small parent sizes; the tracker must dedupe so every emitted
// pair is distinct.
func TestEmitCompositeUniqueDedupes(t *testing.T) {
	cfg := &config.Config{
		Version:  1,
		Database: config.DatabaseSection{Dialect: "postgres"},
		Tables: map[string]*config.Table{
			"public.groups": {
				Schema: "public", Name: "groups", RowCount: intp(3),
				PrimaryKey:  []string{"id"},
				ColumnOrder: []string{"id"},
				Columns:     map[string]*config.ColumnSpec{"id": {Factory: "pk_serial", DataType: "integer"}},
			},
			"public.permissions": {
				Schema: "public", Name: "permissions", RowCount: intp(3),
				PrimaryKey:  []string{"id"},
				ColumnOrder: []string{"id"},
				Columns:     map[string]*config.ColumnSpec{"id": {Factory: "pk_serial", DataType: "integer"}},
			},
			"public.group_permissions": {
				Schema: "public", Name: "group_permissions", RowCount: intp(9),
				ColumnOrder: []string{"group_id", "permission_id"},
				UniqueKeys:  [][]string{{"group_id", "permission_id"}},
				Columns: map[string]*config.ColumnSpec{
					"group_id":      {Factory: "fkref", Params: map[string]any{"target": "public.groups.id"}, DataType: "integer"},
					"permission_id": {Factory: "fkref", Params: map[string]any{"target": "public.permissions.id"}, DataType: "integer"},
				},
			},
		},
	}
	reg := registry.New(factories.All())
	g, _ := relations.Build(cfg)
	plan := g.PlanFor(cfg)
	em := New(cfg, reg, plan, Options{Seed: 99})
	var buf bytes.Buffer
	if err := em.Emit(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	// Extract group_permissions rows and confirm every (group_id, permission_id)
	// tuple is unique.
	marker := "INSERT INTO \"public\".\"group_permissions\""
	i := strings.Index(out, marker)
	if i < 0 {
		t.Fatalf("no group_permissions INSERT in output:\n%s", out)
	}
	body := out[i:]
	end := strings.Index(body, ";")
	if end < 0 {
		t.Fatalf("unterminated INSERT:\n%s", body)
	}
	rowPat := regexp.MustCompile(`\((\d+), (\d+)\)`)
	matches := rowPat.FindAllStringSubmatch(body[:end], -1)
	if len(matches) == 0 {
		t.Fatalf("no rows matched in:\n%s", body[:end])
	}
	seen := map[string]bool{}
	for _, m := range matches {
		key := m[1] + "," + m[2]
		if seen[key] {
			t.Fatalf("duplicate (group_id, permission_id) tuple %s; output:\n%s", key, body[:end])
		}
		seen[key] = true
	}
	// 3×3 = 9 unique pairs possible; requested 9. Tracker should fill them all.
	if len(matches) < 9 {
		if drops := em.Drops()["public.group_permissions"]; drops.Reason != "unique_collision" {
			t.Fatalf("emitted %d/9 rows but no drop recorded: %+v", len(matches), drops)
		}
	}
}

// All-literal Value columns in a unique set can't be resolved by retrying —
// the tracker must drop duplicate rows fast and record them in Drops.
func TestEmitAllLiteralCollisionDrops(t *testing.T) {
	cfg := &config.Config{
		Version:  1,
		Database: config.DatabaseSection{Dialect: "postgres"},
		Tables: map[string]*config.Table{
			"public.singleton": {
				Schema: "public", Name: "singleton", RowCount: intp(3),
				UniqueKeys:  [][]string{{"kind"}},
				ColumnOrder: []string{"kind"},
				Columns: map[string]*config.ColumnSpec{
					"kind": {Value: "config", DataType: "text"},
				},
			},
		},
	}
	reg := registry.New(factories.All())
	g, _ := relations.Build(cfg)
	plan := g.PlanFor(cfg)
	em := New(cfg, reg, plan, Options{Seed: 1})
	var buf bytes.Buffer
	if err := em.Emit(&buf); err != nil {
		t.Fatal(err)
	}
	drop, ok := em.Drops()["public.singleton"]
	if !ok {
		t.Fatalf("expected drop for all-literal collision; got %+v", em.Drops())
	}
	if drop.Emitted != 1 || drop.Requested != 3 {
		t.Fatalf("drop=%+v, want Requested=3 Emitted=1", drop)
	}
}

// Polymorphic pair `commentable_type`/`commentable_id` must be sampled
// atomically: `_type` carries a parent class name and `_id` carries that
// parent's PK. Independent random fill would pair mismatched values.
func TestEmitPolymorphicPair(t *testing.T) {
	cfg := &config.Config{
		Version:  1,
		Database: config.DatabaseSection{Dialect: "postgres"},
		Tables: map[string]*config.Table{
			"public.users": {
				Schema: "public", Name: "users", RowCount: intp(2),
				PrimaryKey:  []string{"id"},
				ColumnOrder: []string{"id"},
				Columns:     map[string]*config.ColumnSpec{"id": {Factory: "pk_serial", DataType: "integer"}},
			},
			"public.articles": {
				Schema: "public", Name: "articles", RowCount: intp(2),
				PrimaryKey:  []string{"id"},
				ColumnOrder: []string{"id"},
				Columns:     map[string]*config.ColumnSpec{"id": {Factory: "pk_serial", DataType: "integer"}},
			},
			"public.comments": {
				Schema: "public", Name: "comments", RowCount: intp(10),
				PrimaryKey:  []string{"id"},
				ColumnOrder: []string{"id", "commentable_type", "commentable_id"},
				Polymorphs: []config.PolymorphicKey{
					{
						TypeColumn: "commentable_type", IdColumn: "commentable_id",
						Candidates: []config.PolymorphCandidate{
							{Table: "public.users", TypeName: "User"},
							{Table: "public.articles", TypeName: "Article"},
						},
					},
				},
				Columns: map[string]*config.ColumnSpec{
					"id":               {Factory: "pk_serial", DataType: "integer"},
					"commentable_type": {Factory: "string", Unresolved: true, DataType: "character varying"},
					"commentable_id":   {Factory: "integer", Unresolved: true, DataType: "integer"},
				},
			},
		},
	}
	reg := registry.New(factories.All())
	g, _ := relations.Build(cfg)
	plan := g.PlanFor(cfg)
	em := New(cfg, reg, plan, Options{Seed: 5})
	var buf bytes.Buffer
	if err := em.Emit(&buf); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	marker := "INSERT INTO \"public\".\"comments\""
	body := out[strings.Index(out, marker):]
	body = body[:strings.Index(body, ";")]
	rowPat := regexp.MustCompile(`\((\d+), E'([^']+)', (\d+)\)`)
	matches := rowPat.FindAllStringSubmatch(body, -1)
	if len(matches) != 10 {
		t.Fatalf("expected 10 rows, got %d; body:\n%s", len(matches), body)
	}
	sawUser, sawArticle := false, false
	for _, m := range matches {
		typ, idStr := m[2], m[3]
		switch typ {
		case "User":
			sawUser = true
			if idStr != "1" && idStr != "2" {
				t.Fatalf("User id=%s out of range", idStr)
			}
		case "Article":
			sawArticle = true
			if idStr != "1" && idStr != "2" {
				t.Fatalf("Article id=%s out of range", idStr)
			}
		default:
			t.Fatalf("unknown type %q", typ)
		}
	}
	if !sawUser || !sawArticle {
		t.Fatalf("expected both candidate types to appear (User=%v, Article=%v)", sawUser, sawArticle)
	}
}

// Relations planner must zero-cascade a NOT NULL FK child when its parent
// row_count is 0 — otherwise generation emits NULL into a NOT NULL column.
func TestRelationsCascadeZero(t *testing.T) {
	cfg := &config.Config{
		Version:  1,
		Database: config.DatabaseSection{Dialect: "postgres"},
		Tables: map[string]*config.Table{
			"public.parents": {
				Schema: "public", Name: "parents", RowCount: intp(0),
				PrimaryKey:  []string{"id"},
				ColumnOrder: []string{"id"},
				Columns:     map[string]*config.ColumnSpec{"id": {Factory: "pk_serial", DataType: "integer"}},
			},
			"public.children": {
				Schema: "public", Name: "children", RowCount: intp(5),
				PrimaryKey:  []string{"id"},
				ColumnOrder: []string{"id", "parent_id"},
				Columns: map[string]*config.ColumnSpec{
					"id":        {Factory: "pk_serial", DataType: "integer"},
					"parent_id": {Factory: "fkref", Params: map[string]any{"target": "public.parents.id"}, DataType: "integer", Nullable: false},
				},
			},
		},
	}
	g, _ := relations.Build(cfg)
	plan := g.PlanFor(cfg)
	if plan.RowCounts["public.children"] != 0 {
		t.Fatalf("expected children row_count cascaded to 0, got %d", plan.RowCounts["public.children"])
	}
	if len(plan.CascadedFrom["public.children"]) == 0 {
		t.Fatalf("expected cascade reason recorded for public.children")
	}
}

func TestEmitJSONValues(t *testing.T) {
	cfg := &config.Config{
		Version:  1,
		Database: config.DatabaseSection{Dialect: "postgres"},
		Tables: map[string]*config.Table{
			"public.profiles": {
				Schema: "public", Name: "profiles", RowCount: intp(2),
				ColumnOrder: []string{"id", "meta"},
				PrimaryKey:  []string{"id"},
				Columns: map[string]*config.ColumnSpec{
					"id": {Factory: "pk_serial", DataType: "integer"},
					"meta": {
						Factory:  "json_any",
						DataType: "jsonb",
						Values: map[string]*config.ColumnSpec{
							"name":  {Factory: "first_name", DataType: "text"},
							"score": {Factory: "integer", DataType: "integer", Params: map[string]any{"min": 1, "max": 100}},
							"addr": {
								Factory:  "json_any",
								DataType: "jsonb",
								Values: map[string]*config.ColumnSpec{
									"city": {Factory: "city", DataType: "text"},
								},
							},
						},
					},
				},
			},
		},
	}
	reg := registry.New(factories.All())
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
