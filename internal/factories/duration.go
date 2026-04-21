package factories

import "github.com/inikalaev/database-seed-cli/pkg/seedapi"

type durationMech struct{}

func (durationMech) Name() string   { return "duration" }
func (durationMech) Tags() []string { return []string{"duration", "time_spent", "elapsed"} }

func (durationMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isInt(ctx.Column) && !isNumeric(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `(^|_)(duration|time_spent|elapsed)(_|$)`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

// Generate returns seconds in [0, 7200].
func (durationMech) Generate(ctx seedapi.GenContext) any {
	lo := ctx.Params.Int("min", 0)
	hi := ctx.Params.Int("max", 7200)
	return inclusiveIntN(ctx.Rng, lo, hi)
}
