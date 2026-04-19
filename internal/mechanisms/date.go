package mechanisms

import (
	"time"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type dateMech struct{}

func (dateMech) Name() string   { return "date" }
func (dateMech) Tags() []string { return []string{"time"} }

func (dateMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if isDate(ctx.Column) {
		return seedapi.TypeMatch
	}
	return seedapi.NoMatch
}

func (dateMech) Generate(ctx seedapi.GenContext) any {
	base := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	return base.AddDate(0, 0, ctx.Rng.IntN(365*5)).Format("2006-01-02")
}
