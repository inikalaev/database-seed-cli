package factories

import (
	"fmt"
	"time"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type timestampStrMech struct{}

func (timestampStrMech) Name() string   { return "timestamp_str" }
func (timestampStrMech) Tags() []string { return []string{"created_time", "updated_time"} }

func (timestampStrMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isText(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `_time$`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

// Generate returns an ISO 8601 timestamp string within the year starting
// 2024-01-01T00:00:00Z. Hardcoded anchor preserves determinism across days.
func (timestampStrMech) Generate(ctx seedapi.GenContext) any {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	t := base.Add(time.Duration(ctx.Rng.Int64N(int64(365 * 24 * time.Hour))))
	return fmt.Sprintf("%d-%02d-%02dT%02d:%02d:%02d+00:00",
		t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second())
}
