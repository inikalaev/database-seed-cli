package factories

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

type levelMech struct{}

func (levelMech) Name() string   { return "level" }
func (levelMech) Tags() []string { return []string{"level", "tier", "grade"} }

func (levelMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isInt(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `(^|_)level(_|$)|(^|_)tier(_|$)`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

func (levelMech) Generate(ctx seedapi.GenContext) any {
	lo := ctx.Params.Int("min", 1)
	hi := ctx.Params.Int("max", 10)
	return inclusiveIntN(ctx.Rng, lo, hi)
}
