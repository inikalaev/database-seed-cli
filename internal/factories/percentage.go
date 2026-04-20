package factories

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

type percentageMech struct{}

func (percentageMech) Name() string   { return "percentage" }
func (percentageMech) Tags() []string { return []string{"score", "percent", "progress", "rating", "grade"} }

func (percentageMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isInt(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `(^|_)(score|percent|progress|rating|grade)(_|$)`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

func (percentageMech) Generate(ctx seedapi.GenContext) any {
	lo := ctx.Params.Int("min", 0)
	hi := ctx.Params.Int("max", 100)
	return inclusiveIntN(ctx.Rng, lo, hi)
}
