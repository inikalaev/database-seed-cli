package mechanisms

import (
	"time"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

// timestampMech returns a moment inside a fixed one-year window anchored at
// 2020-09-13 UTC. For richer distributions (business hours, skewed recency)
// write a domain-specific mechanism.
type timestampMech struct{}

func (timestampMech) Name() string   { return "timestamp" }
func (timestampMech) Tags() []string { return []string{"time"} }

func (timestampMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if isTimestamp(ctx.Column) {
		return seedapi.TypeMatch
	}
	return seedapi.NoMatch
}

func (timestampMech) Generate(ctx seedapi.GenContext) any {
	base := time.Unix(1_600_000_000, 0).UTC()
	return base.Add(time.Duration(ctx.Rng.Int64N(int64(365*24*time.Hour))) * 1).Format("2006-01-02 15:04:05")
}
