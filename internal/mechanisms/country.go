package mechanisms

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

type countryMech struct{}

func (countryMech) Name() string   { return "country" }
func (countryMech) Tags() []string { return []string{"address"} }

func (countryMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isText(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `country`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

func (countryMech) Generate(ctx seedapi.GenContext) any {
	pool := []string{
		"US", "GB", "CA", "AU", "NZ",
		"DE", "FR", "ES", "IT", "NL", "PL", "SE", "NO", "CH", "AT",
		"RU", "UA", "CZ", "HU", "RO",
		"CN", "JP", "KR", "IN", "ID", "TH", "SG", "VN", "PH", "MY",
		"BR", "MX", "AR", "CO", "CL", "PE",
		"ZA", "NG", "KE", "EG", "MA", "GH",
		"TR", "SA", "AE", "IL", "PK", "BD",
	}
	return pool[ctx.Rng.IntN(len(pool))]
}
