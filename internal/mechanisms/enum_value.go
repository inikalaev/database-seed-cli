package mechanisms

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

// enumValue samples uniformly from the labels discovered during introspection.
// Override via params.weights (map[label]int) once a weighted variant is needed —
// not in MVP.
type enumValue struct{}

func (enumValue) Name() string   { return "enum_value" }
func (enumValue) Tags() []string { return []string{"enum"} }

func (enumValue) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if len(ctx.Column.EnumValues) > 0 {
		return seedapi.StrongMatch
	}
	return seedapi.NoMatch
}

func (enumValue) Generate(ctx seedapi.GenContext) any {
	vals := ctx.Column.EnumValues
	if len(vals) == 0 {
		return nil
	}
	return vals[ctx.Rng.IntN(len(vals))]
}
