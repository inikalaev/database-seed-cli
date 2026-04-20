package registry

import (
	"testing"

	"github.com/ivannikolaev/seed-cli/cli/internal/factories"
	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
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

// TestInferPicksNamedOverGeneric — guard на инвариант: именованная числовая
// фабрика (amount, percentage, latitude, …) должна выигрывать над generic
// intMech/decimalMech/boolMech/dateMech/hstoreMech. После перевода generic
// на WeakNameMatch это зависит не от load-bearing порядка регистрации в
// factories.All(), а от score (NameMatch > WeakNameMatch).
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

// TestInferPluginWinsOverGeneric — ключевой guard по итогам ревью: пользовательский
// плагин, возвращающий NameMatch(70), должен перебивать встроенный generic
// (boolMech/intMech/decimalMech/dateMech/hstoreMech), который теперь даёт
// WeakNameMatch(60). До правки оба возвращали NameMatch и встроенный выигрывал
// за счёт более раннего порядка регистрации — это был silent breaking change.
func TestInferPluginWinsOverGeneric(t *testing.T) {
	plugin := testPluginFactory{
		name:  "my_plugin",
		match: seedapi.NameMatch,
	}
	// Важно: плагин идёт ПОСЛЕ билтинов — именно этот порядок ломал плагины до фикса.
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

// TestInferPluginTieLosesToBuiltinByOrder — оборотная сторона предыдущего теста:
// если плагин объявляет score WeakNameMatch (равный generic builtin), tie-break
// идёт по порядку регистрации — builtin зарегистрирован раньше и выигрывает.
// Это наблюдаемый контракт: плагины перебивают generic только с NameMatch или
// выше, а не с тем же WeakNameMatch.
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

// testPluginFactory — минимальный stub для проверки tie-break между билтинами
// и плагинами. Match() возвращает фиксированный score для любой колонки.
type testPluginFactory struct {
	name  string
	match seedapi.MatchScore
}

func (p testPluginFactory) Name() string                                  { return p.name }
func (p testPluginFactory) Tags() []string                                { return nil }
func (p testPluginFactory) Match(seedapi.MatchContext) seedapi.MatchScore { return p.match }
func (p testPluginFactory) Generate(seedapi.GenContext) any               { return nil }
