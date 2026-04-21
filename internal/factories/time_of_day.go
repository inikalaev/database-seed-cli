package factories

import (
	"fmt"
	"strings"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

type timeOfDayMech struct{}

func (timeOfDayMech) Name() string   { return "time_of_day" }
func (timeOfDayMech) Tags() []string { return []string{"time"} }

func (timeOfDayMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	switch strings.ToLower(ctx.Column.UDTName) {
	case "time", "timetz":
		return seedapi.TypeMatch
	}
	return seedapi.NoMatch
}

func (timeOfDayMech) Generate(ctx seedapi.GenContext) any {
	h := ctx.Rng.IntN(24)
	m := ctx.Rng.IntN(60)
	s := ctx.Rng.IntN(60)
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}
