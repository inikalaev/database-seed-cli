package factories

import "github.com/inikalaev/database-seed-cli/pkg/seedapi"

type yearMech struct{}

func (yearMech) Name() string   { return "year" }
func (yearMech) Tags() []string { return []string{"year"} }

func (yearMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isInt(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `(^|_)year(_|$)`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

func (yearMech) Generate(ctx seedapi.GenContext) any {
	lo := ctx.Params.Int("min", 2015)
	hi := ctx.Params.Int("max", 2026)
	return inclusiveIntN(ctx.Rng, lo, hi)
}
