package factories

import "github.com/inikalaev/database-seed-cli/pkg/seedapi"

type cityMech struct{}

func (cityMech) Name() string   { return "city" }
func (cityMech) Tags() []string { return []string{"town", "locality", "city_name"} }

func (cityMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	return textNameMatch(ctx.Column, "city", cityMech{}.Tags())
}

func (cityMech) Generate(ctx seedapi.GenContext) any {
	pool := []string{
		// North America
		"New York", "Los Angeles", "Chicago", "Toronto", "Mexico City",
		// South America
		"São Paulo", "Buenos Aires", "Bogotá", "Lima", "Santiago",
		// Europe
		"London", "Paris", "Berlin", "Madrid", "Rome", "Amsterdam", "Vienna", "Warsaw",
		// Russia / Eastern Europe
		"Moscow", "Saint Petersburg", "Kyiv", "Prague", "Budapest",
		// Middle East / Africa
		"Dubai", "Istanbul", "Cairo", "Lagos", "Nairobi", "Casablanca",
		// South Asia
		"Mumbai", "Delhi", "Bangalore", "Karachi", "Dhaka",
		// East / Southeast Asia
		"Tokyo", "Shanghai", "Beijing", "Seoul", "Jakarta", "Bangkok", "Singapore",
		// Oceania
		"Sydney", "Melbourne", "Auckland",
	}
	return pool[ctx.Rng.IntN(len(pool))]
}
