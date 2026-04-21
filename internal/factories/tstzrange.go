package factories

import (
	"fmt"
	"strings"
	"time"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

type tstzrangeMech struct{}

func (tstzrangeMech) Name() string   { return "tstzrange" }
func (tstzrangeMech) Tags() []string { return []string{"validity", "valid_period", "period", "range"} }

func (tstzrangeMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if strings.EqualFold(ctx.Column.UDTName, "tstzrange") {
		return seedapi.TypeMatch
	}
	return seedapi.NoMatch
}

// Generate returns a PostgreSQL tstzrange literal like
// ["2024-01-01 00:00:00+00","2024-03-01 00:00:00+00").
func (tstzrangeMech) Generate(ctx seedapi.GenContext) any {
	anchor := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	startOffset := time.Duration(ctx.Rng.IntN(365)) * 24 * time.Hour
	start := anchor.Add(-startOffset)
	durationDays := 30 + ctx.Rng.IntN(335) // 30..364 days
	end := start.Add(time.Duration(durationDays) * 24 * time.Hour)
	const f = "2006-01-02 15:04:05+00"
	return fmt.Sprintf(`["%s","%s")`, start.Format(f), end.Format(f))
}
