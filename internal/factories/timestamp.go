package factories

import (
	"time"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

// timestampMech returns a moment inside a fixed one-year window anchored at
// 2024-01-01T00:00:00Z. Using a hardcoded anchor keeps output deterministic
// when the same seed is replayed on a different day.
type timestampMech struct{}

func (timestampMech) Name() string   { return "timestamp" }
func (timestampMech) Tags() []string { return []string{"time"} }

func (timestampMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isTimestamp(ctx.Column) {
		return seedapi.NoMatch
	}
	// Columns with suffixes `_at/_on/_date/_time` (created_at, start_date,
	// action_time, applied_on, …) clearly represent a point in time. WeakNameMatch
	// keeps them resolved while a user plugin returning NameMatch still wins.
	// A bare timestamp column with no name signal stays at TypeMatch (→ unresolved)
	// so the user decides what belongs there.
	if nameMatches(ctx.Column, `_at$`, `_on$`, `_date$`, `_time$`, `^deadline$`, `^applied$`) {
		return seedapi.WeakNameMatch
	}
	return seedapi.TypeMatch
}

func (timestampMech) Generate(ctx seedapi.GenContext) any {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	// Return time.Time so formatSQL includes the UTC offset — correct for timestamptz.
	return base.Add(time.Duration(ctx.Rng.Int64N(int64(365 * 24 * time.Hour))))
}
