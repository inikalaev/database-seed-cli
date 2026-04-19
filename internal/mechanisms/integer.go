package mechanisms

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

// intMech is the generic fallback for integer types. Name-driven integer
// columns (ages, counts, cents) should get dedicated mechanisms — this one
// only owns TypeMatch tier.
type intMech struct{}

func (intMech) Name() string   { return "integer" }
func (intMech) Tags() []string { return []string{"numeric"} }

func (intMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if isInt(ctx.Column) {
		return seedapi.TypeMatch
	}
	return seedapi.NoMatch
}

func (intMech) Generate(ctx seedapi.GenContext) any {
	lo := ctx.Params.Int("min", 0)
	hi := ctx.Params.Int("max", 1_000_000)
	if hi <= lo {
		return lo
	}
	return lo + ctx.Rng.IntN(hi-lo)
}
