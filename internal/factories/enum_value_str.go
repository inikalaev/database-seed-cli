package factories

import "github.com/inikalaev/database-seed-cli/pkg/seedapi"

// enumValue samples uniformly from the labels discovered during introspection.
// Override via params.weights (map[label]int) once a weighted variant is needed —
// not in MVP.
type enumValueStr struct{}

func (enumValueStr) Name() string   { return "EnumValueStr" }
func (enumValueStr) Tags() []string { return []string{"type", "status"} }

func (enumValueStr) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isText(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `_type$`, `_status$`, `^type$`, `^status$`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

// RequiredSetup returns a single required step when params["values"] is empty.
// Once values are present, no setup is needed.
func (enumValueStr) RequiredSetup(params map[string]any) []seedapi.SetupStep {
	if raw, ok := params["values"].([]any); ok && len(raw) > 0 {
		return nil
	}
	return []seedapi.SetupStep{{
		ParamKey: "values",
		Kind:     seedapi.SetupList,
		Element:  &seedapi.SetupStep{Kind: seedapi.SetupString},
		Prompt:   "Allowed values (comma-separated):",
		Help:     "e.g. pending,active,cancelled",
		Required: true,
	}}
}

func (enumValueStr) Generate(ctx seedapi.GenContext) any {
	vals := []string{}
	// Fallback: labels stashed in params["values"] by config.FromModel.
	if raw, ok := ctx.Params["values"].([]any); ok {
		for _, v := range raw {
			if s, ok := v.(string); ok {
				vals = append(vals, s)
			}
		}
	}
	if len(vals) == 0 {
		return nil
	}
	return vals[ctx.Rng.IntN(len(vals))]
}
