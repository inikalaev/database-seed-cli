package factories

import (
	"fmt"
	"strings"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type pgIntervalMech struct{}

func (pgIntervalMech) Name() string   { return "pg_interval" }
func (pgIntervalMech) Tags() []string { return []string{"interval", "duration"} }

func (pgIntervalMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if strings.EqualFold(ctx.Column.UDTName, "interval") {
		return seedapi.TypeMatch
	}
	return seedapi.NoMatch
}

// Generate returns a PostgreSQL interval string like "02:35:00".
func (pgIntervalMech) Generate(ctx seedapi.GenContext) any {
	totalMinutes := 30 + ctx.Rng.IntN(12*60) // 30 min .. ~12 hours
	h := totalMinutes / 60
	m := totalMinutes % 60
	return fmt.Sprintf("%02d:%02d:00", h, m)
}
