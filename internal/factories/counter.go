package factories

import "github.com/inikalaev/database-seed-cli/pkg/seedapi"

type counterMech struct{}

func (counterMech) Name() string   { return "counter" }
func (counterMech) Tags() []string { return []string{"count"} }

func (counterMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isInt(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `_count$`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

func (counterMech) Generate(ctx seedapi.GenContext) any {
	lo := ctx.Params.Int("min", 0)
	hi := ctx.Params.Int("max", 100)
	return inclusiveIntN(ctx.Rng, lo, hi)
}
