package mechanisms

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

type companyMech struct{}

func (companyMech) Name() string   { return "company" }
func (companyMech) Tags() []string { return []string{"business"} }

func (companyMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isText(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `company`, `organi[sz]ation`, `employer`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

func (companyMech) Generate(ctx seedapi.GenContext) any {
	pool := []string{"Acme", "Globex", "Initech", "Umbrella", "Soylent", "Wayne Enterprises", "Stark Industries"}
	return pool[ctx.Rng.IntN(len(pool))]
}
