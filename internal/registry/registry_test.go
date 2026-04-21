package registry

import (
	"testing"

	"github.com/inikalaev/database-seed-cli/internal/factories"
	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

func TestInferBuiltins(t *testing.T) {
	r := New(factories.All())
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
			if res.Factory == nil || res.Factory.Name() != c.wantMech {
				got := "nil"
				if res.Factory != nil {
					got = res.Factory.Name()
				}
				t.Fatalf("want %s, got %s (score %d)", c.wantMech, got, res.Score)
			}
		})
	}
}

func TestInferUnresolved(t *testing.T) {
	r := New(factories.All())
	// Unknown weird type → no strong matcher; falls back to `string` (weak) with unresolved=true.
	res := r.Infer(seedapi.Column{Name: "xyz", DataType: "text"}, "")
	if !res.Unresolved {
		t.Fatalf("expected unresolved=true for generic text column")
	}
	if res.Factory.Name() != "string" {
		t.Fatalf("expected string fallback, got %s", res.Factory.Name())
	}
}

// TestInferPicksNamedOverGeneric guards the invariant that a named numeric factory
// (amount, percentage, latitude, …) beats the generic intMech/decimalMech/boolMech/
// dateMech/hstoreMech. Since generics now return WeakNameMatch, this is driven by
// score (NameMatch > WeakNameMatch), not by registration order in factories.All().
func TestInferPicksNamedOverGeneric(t *testing.T) {
	r := New(factories.All())
	cases := []struct {
		name     string
		col      seedapi.Column
		wantMech string
	}{
		{"payment_amount→amount", seedapi.Column{Name: "payment_amount", DataType: "numeric"}, "amount"},
		{"discount_amount→amount", seedapi.Column{Name: "discount_amount", DataType: "numeric"}, "amount"},
		{"user_score→percentage", seedapi.Column{Name: "user_score", DataType: "integer"}, "percentage"},
		{"latitude→latitude", seedapi.Column{Name: "latitude", DataType: "numeric"}, "latitude"},
		{"user_count→counter", seedapi.Column{Name: "user_count", DataType: "integer"}, "counter"},
		{"birth_year→year", seedapi.Column{Name: "birth_year", DataType: "integer"}, "year"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := r.Infer(c.col, "")
			if res.Factory == nil || res.Factory.Name() != c.wantMech {
				got := "nil"
				if res.Factory != nil {
					got = res.Factory.Name()
				}
				t.Fatalf("want %s, got %s (score %d)", c.wantMech, got, res.Score)
			}
			if res.Unresolved {
				t.Fatalf("expected unresolved=false for %s (named factory wins)", c.name)
			}
		})
	}
}

// TestInferPluginWinsOverGeneric is a key regression guard: a user plugin returning
// NameMatch(70) must beat the built-in generic (boolMech/intMech/decimalMech/
// dateMech/hstoreMech) which now returns WeakNameMatch(60). Before the fix both
// returned NameMatch and the builtin won by registration order — a silent breaking
// change for plugin authors.
func TestInferPluginWinsOverGeneric(t *testing.T) {
	plugin := testPluginFactory{
		name:  "my_plugin",
		match: seedapi.NameMatch,
	}
	// Plugin is appended AFTER builtins — the order that broke plugins before the fix.
	mechs := append([]seedapi.Factory{}, factories.All()...)
	mechs = append(mechs, plugin)
	r := New(mechs)

	cases := []struct {
		name string
		col  seedapi.Column
	}{
		{"bool", seedapi.Column{Name: "active", DataType: "boolean"}},
		{"int-generic", seedapi.Column{Name: "counter", DataType: "integer"}},
		{"numeric", seedapi.Column{Name: "value", DataType: "numeric"}},
		{"date", seedapi.Column{Name: "logged", DataType: "date"}},
		{"hstore", seedapi.Column{Name: "props", UDTName: "hstore"}},
		{"timestamp-named", seedapi.Column{Name: "created_at", DataType: "timestamp"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			res := r.Infer(c.col, "")
			if res.Factory == nil || res.Factory.Name() != "my_plugin" {
				got := "nil"
				if res.Factory != nil {
					got = res.Factory.Name()
				}
				t.Fatalf("plugin must win over generic builtin: got %s (score %d)", got, res.Score)
			}
		})
	}
}

// TestInferPluginTieLosesToBuiltinByOrder is the counterpart to the previous test:
// when a plugin declares WeakNameMatch (equal to the generic builtin), tie-breaking
// falls back to registration order — the builtin registered earlier wins. This is
// the observable contract: plugins beat generics only at NameMatch or higher, not
// at the same WeakNameMatch level.
func TestInferPluginTieLosesToBuiltinByOrder(t *testing.T) {
	plugin := testPluginFactory{
		name:  "my_plugin",
		match: seedapi.WeakNameMatch,
	}
	mechs := append([]seedapi.Factory{}, factories.All()...)
	mechs = append(mechs, plugin)
	r := New(mechs)

	res := r.Infer(seedapi.Column{Name: "active", DataType: "boolean"}, "")
	if res.Factory == nil || res.Factory.Name() != "bool" {
		got := "nil"
		if res.Factory != nil {
			got = res.Factory.Name()
		}
		t.Fatalf("builtin bool must win tie against plugin at WeakNameMatch: got %s (score %d)", got, res.Score)
	}
	if res.Unresolved {
		t.Fatalf("expected unresolved=false on tie at WeakNameMatch")
	}
}

// testPluginFactory is a minimal stub for verifying tie-breaking between builtins
// and plugins. Match() returns a fixed score for any column.
type testPluginFactory struct {
	name  string
	match seedapi.MatchScore
}

func (p testPluginFactory) Name() string                                  { return p.name }
func (p testPluginFactory) Tags() []string                                { return nil }
func (p testPluginFactory) Match(seedapi.MatchContext) seedapi.MatchScore { return p.match }
func (p testPluginFactory) Generate(seedapi.GenContext) any               { return nil }
