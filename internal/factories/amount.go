package factories

import (
	"math"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type amountMech struct{}

func (amountMech) Name() string   { return "amount" }
func (amountMech) Tags() []string { return []string{"amount", "price", "cost", "fee", "balance"} }

func (amountMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isNumeric(ctx.Column) && !isInt(ctx.Column) {
		return seedapi.NoMatch
	}
	col := normName(ctx.Column.Name)
	for _, tag := range (amountMech{}).Tags() {
		if contains(col, normName(tag)) {
			return seedapi.NameMatch
		}
	}
	return seedapi.NoMatch
}

// Generate returns a monetary value rounded to 2 decimal places.
func (amountMech) Generate(ctx seedapi.GenContext) any {
	lo := ctx.Params.Float("min", 0)
	hi := ctx.Params.Float("max", 9_999.99)
	v := lo + ctx.Rng.Float64()*(hi-lo)
	return math.Round(v*100) / 100
}
