package factories

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

type versionIntMech struct{}

func (versionIntMech) Name() string   { return "version_int" }
func (versionIntMech) Tags() []string { return []string{"version", "revision"} }

func (versionIntMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isInt(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `(^|_)version(_|$)|(^|_)revision(_|$)`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

func (versionIntMech) Generate(ctx seedapi.GenContext) any {
	return 1 + ctx.Rng.IntN(50)
}
