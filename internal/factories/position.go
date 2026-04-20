package factories

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

type positionMech struct{}

func (positionMech) Name() string   { return "position" }
func (positionMech) Tags() []string { return []string{"position", "order", "sort_order", "rank"} }

func (positionMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isInt(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `(^|_)position(_|$)|(^|_)sort_order(_|$)`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

func (positionMech) Generate(ctx seedapi.GenContext) any {
	return 1 + ctx.Rng.IntN(100)
}
