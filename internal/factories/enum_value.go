package factories

import "github.com/inikalaev/database-seed-cli/pkg/seedapi"

// enumValue samples uniformly from the labels discovered during introspection.
// Override via params.weights (map[label]int) once a weighted variant is needed —
// not in MVP.
type enumValue struct{}

func (enumValue) Name() string   { return seedapi.FactoryEnumValue }
func (enumValue) Tags() []string { return []string{"enum"} }

func (enumValue) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if len(ctx.Column.EnumValues) > 0 {
		return seedapi.StrongMatch
	}
	return seedapi.NoMatch
}

// RequiredSetup returns a single required step when params["values"] is empty.
// Introspected PG enums populate Column.EnumValues directly, but when the user
// assigned EnumValue manually without introspected labels the CLI still needs
// to collect values here.
func (enumValue) RequiredSetup(params map[string]any) []seedapi.SetupStep {
	if raw, ok := params["values"].([]any); ok && len(raw) > 0 {
		return nil
	}
	return []seedapi.SetupStep{{
		ParamKey: "values",
		Kind:     seedapi.SetupList,
		Element:  &seedapi.SetupStep{Kind: seedapi.SetupString},
		Prompt:   "Allowed values (comma-separated):",
		Help:     "e.g. draft,published,archived",
		Required: true,
	}}
}

func (enumValue) Generate(ctx seedapi.GenContext) any {
	vals := ctx.Column.EnumValues
	if len(vals) == 0 {
		// Fallback: labels stashed in params["values"] by config.FromModel.
		if raw, ok := ctx.Params["values"].([]any); ok {
			for _, v := range raw {
				if s, ok := v.(string); ok {
					vals = append(vals, s)
				}
			}
		}
	}
	if len(vals) == 0 {
		return nil
	}
	return vals[ctx.Rng.IntN(len(vals))]
}
