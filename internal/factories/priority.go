package factories

import "github.com/inikalaev/database-seed-cli/pkg/seedapi"

type priorityMech struct{}

func (priorityMech) Name() string   { return "priority" }
func (priorityMech) Tags() []string { return []string{"priority"} }

func (priorityMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isInt(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `(^|_)priority(_|$)`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

func (priorityMech) Generate(ctx seedapi.GenContext) any {
	lo := ctx.Params.Int("min", 1)
	hi := ctx.Params.Int("max", 10)
	return inclusiveIntN(ctx.Rng, lo, hi)
}
