package mechanisms

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

type decimalMech struct{}

func (decimalMech) Name() string   { return "decimal" }
func (decimalMech) Tags() []string { return []string{"numeric"} }

func (decimalMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if isNumeric(ctx.Column) {
		return seedapi.TypeMatch
	}
	return seedapi.NoMatch
}

func (decimalMech) Generate(ctx seedapi.GenContext) any {
	lo := ctx.Params.Float("min", 0)
	hi := ctx.Params.Float("max", 10_000)
	return lo + ctx.Rng.Float64()*(hi-lo)
}
