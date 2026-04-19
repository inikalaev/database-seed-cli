package mechanisms

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

type boolMech struct{}

func (boolMech) Name() string   { return "bool" }
func (boolMech) Tags() []string { return []string{"logic"} }

func (boolMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if isBool(ctx.Column) {
		return seedapi.TypeMatch
	}
	return seedapi.NoMatch
}

func (boolMech) Generate(ctx seedapi.GenContext) any {
	return ctx.Rng.IntN(2) == 1
}
