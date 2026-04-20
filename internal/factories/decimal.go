package factories

import (
	"math"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type decimalMech struct{}

func (decimalMech) Name() string   { return "decimal" }
func (decimalMech) Tags() []string { return []string{"numeric"} }

func (decimalMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	// numeric/float is unambiguous and [0, 10_000] is a sensible default for
	// typical score/weight/percent columns. WeakNameMatch keeps it resolved while
	// any named factory (amount, percentage, latitude, …) or NameMatch plugin wins.
	if isNumeric(ctx.Column) {
		return seedapi.WeakNameMatch
	}
	return seedapi.NoMatch
}

func (decimalMech) Generate(ctx seedapi.GenContext) any {
	lo := ctx.Params.Float("min", 0)
	hi := ctx.Params.Float("max", 10_000)
	// Clamp hi to the maximum value representable by the column's declared
	// precision so generated values never cause a PG "numeric field overflow".
	// numeric(p,s) holds values strictly less than 10^(p-s); the largest
	// representable magnitude is 10^(p-s) - 10^(-s). We subtract one ULP to
	// stay on the right side of that boundary even after float rounding.
	if ctx.Column.NumPrecision != nil && *ctx.Column.NumPrecision > 0 {
		scale := 0
		if ctx.Column.NumScale != nil && *ctx.Column.NumScale >= 0 {
			scale = *ctx.Column.NumScale
		}
		maxVal := math.Pow(10, float64(*ctx.Column.NumPrecision-scale)) - math.Pow(10, -float64(scale))
		if maxVal < hi {
			hi = maxVal
		}
	}
	return lo + ctx.Rng.Float64()*(hi-lo)
}
