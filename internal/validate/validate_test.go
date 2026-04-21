package validate

import (
	"testing"

	"github.com/inikalaev/database-seed-cli/internal/registry"
	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

type plainFactory struct{ name string }

func (p plainFactory) Name() string                    { return p.name }
func (p plainFactory) Tags() []string                  { return nil }
func (p plainFactory) Generate(seedapi.GenContext) any { return nil }

type confFactory struct {
	name string
	keys []string
}

func (c confFactory) Name() string                    { return c.name }
func (c confFactory) Tags() []string                  { return nil }
func (c confFactory) Generate(seedapi.GenContext) any { return nil }

func (c confFactory) RequiredSetup(params map[string]any) []seedapi.SetupStep {
	var out []seedapi.SetupStep
	for _, k := range c.keys {
		if _, ok := params[k]; ok {
			continue
		}
		out = append(out, seedapi.SetupStep{ParamKey: k, Kind: seedapi.SetupString, Required: true})
	}
	return out
}

func TestMissingFactoryParamsIssue_UnknownFactory(t *testing.T) {
	reg := registry.New(nil)
	if _, ok := missingFactoryParamsIssue(reg, "ghost", nil, "loc", "t", "c", ""); ok {
		t.Fatal("unknown factory must not emit an issue")
	}
}

func TestMissingFactoryParamsIssue_NotConfigurable(t *testing.T) {
	reg := registry.New([]seedapi.Factory{plainFactory{name: "plain"}})
	if _, ok := missingFactoryParamsIssue(reg, "plain", nil, "loc", "t", "c", ""); ok {
		t.Fatal("non-Configurable factory must not emit an issue")
	}
}

func TestMissingFactoryParamsIssue_Satisfied(t *testing.T) {
	reg := registry.New([]seedapi.Factory{confFactory{name: "conf", keys: []string{"a"}}})
	params := map[string]any{"a": "x"}
	if _, ok := missingFactoryParamsIssue(reg, "conf", params, "loc", "t", "c", ""); ok {
		t.Fatal("satisfied factory must not emit an issue")
	}
}

func TestMissingFactoryParamsIssue_MultipleRequired(t *testing.T) {
	reg := registry.New([]seedapi.Factory{confFactory{name: "conf", keys: []string{"a", "b"}}})
	issue, ok := missingFactoryParamsIssue(reg, "conf", nil, "t.c", "t", "c", "")
	if !ok {
		t.Fatal("expected issue")
	}
	if issue.Kind != KindMissingFactoryParam {
		t.Fatalf("kind = %v", issue.Kind)
	}
	params, _ := issue.Fix.Ctx["params"].([]string)
	if len(params) != 2 || params[0] != "a" || params[1] != "b" {
		t.Fatalf("params = %v", params)
	}
	if issue.Message != `factory conf requires a, b` {
		t.Fatalf("message = %q", issue.Message)
	}
}

func TestMissingFactoryParamsIssue_DefensiveCopy(t *testing.T) {
	reg := registry.New([]seedapi.Factory{confFactory{name: "conf", keys: []string{"a"}}})
	issue, _ := missingFactoryParamsIssue(reg, "conf", nil, "t.c", "t", "c", "")
	params := issue.Fix.Ctx["params"].([]string)
	params[0] = "mutated"
	issue2, _ := missingFactoryParamsIssue(reg, "conf", nil, "t.c", "t", "c", "")
	if issue2.Fix.Ctx["params"].([]string)[0] != "a" {
		t.Fatal("Ctx params must not share state between calls")
	}
}

func TestMissingFactoryParamsIssue_FieldLocation(t *testing.T) {
	reg := registry.New([]seedapi.Factory{confFactory{name: "conf", keys: []string{"a"}}})
	issue, _ := missingFactoryParamsIssue(reg, "conf", nil, "t.c.f", "t", "c", "f")
	if issue.Fix.Field != "f" {
		t.Fatalf("field = %q", issue.Fix.Field)
	}
	if issue.Location != "t.c.f" {
		t.Fatalf("location = %q", issue.Location)
	}
}
