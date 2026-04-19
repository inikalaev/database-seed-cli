package registry

import (
	"testing"

	"github.com/ivannikolaev/seed-cli/cli/internal/mechanisms"
	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

func TestInferBuiltins(t *testing.T) {
	r := New(mechanisms.All())
	cases := []struct {
		name     string
		col      seedapi.Column
		wantMech string
	}{
		{"pk id", seedapi.Column{Name: "id", DataType: "integer"}, "pk_serial"},
		{"email", seedapi.Column{Name: "email", DataType: "text"}, "email"},
		{"first_name", seedapi.Column{Name: "first_name", DataType: "text"}, "first_name"},
		{"fk", seedapi.Column{Name: "user_id", DataType: "integer", FKTarget: "public.users.id"}, "fkref"},
		{"bool", seedapi.Column{Name: "active", DataType: "boolean"}, "bool"},
		{"json", seedapi.Column{Name: "meta", DataType: "jsonb"}, "json_any"},
		{"uuid", seedapi.Column{Name: "token", DataType: "uuid"}, "uuid"},
		{"enum", seedapi.Column{Name: "status", DataType: "USER-DEFINED", EnumValues: []string{"a", "b"}}, "enum_value"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := r.Infer(c.col, "ru_RU")
			if res.Mechanism == nil || res.Mechanism.Name() != c.wantMech {
				got := "nil"
				if res.Mechanism != nil {
					got = res.Mechanism.Name()
				}
				t.Fatalf("want %s, got %s (score %d)", c.wantMech, got, res.Score)
			}
		})
	}
}

func TestInferUnresolved(t *testing.T) {
	r := New(mechanisms.All())
	// Unknown weird type → no strong matcher; falls back to `string` (weak) with unresolved=true.
	res := r.Infer(seedapi.Column{Name: "xyz", DataType: "text"}, "")
	if !res.Unresolved {
		t.Fatalf("expected unresolved=true for generic text column")
	}
	if res.Mechanism.Name() != "string" {
		t.Fatalf("expected string fallback, got %s", res.Mechanism.Name())
	}
}
