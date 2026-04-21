package factories

import "github.com/inikalaev/database-seed-cli/pkg/seedapi"

type companyMech struct{}

func (companyMech) Name() string { return "company" }
func (companyMech) Tags() []string {
	return []string{"organisation", "organization", "employer", "company_name"}
}

func (companyMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	return textNameMatch(ctx.Column, "company", companyMech{}.Tags())
}

func (companyMech) Generate(ctx seedapi.GenContext) any {
	pool := []string{"Acme", "Globex", "Initech", "Umbrella", "Soylent", "Wayne Enterprises", "Stark Industries"}
	return pool[ctx.Rng.IntN(len(pool))]
}
