package factories

import (
	"fmt"
	"strings"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type pointMech struct{}

func (pointMech) Name() string   { return "point" }
func (pointMech) Tags() []string { return []string{"coordinates", "coordinate", "point", "location"} }

func (pointMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if strings.EqualFold(ctx.Column.UDTName, "point") {
		return seedapi.TypeMatch
	}
	return seedapi.NoMatch
}

// Generate returns a PostgreSQL point literal "(lat,lon)".
func (pointMech) Generate(ctx seedapi.GenContext) any {
	lat := -90.0 + ctx.Rng.Float64()*180.0
	lon := -180.0 + ctx.Rng.Float64()*360.0
	return fmt.Sprintf("(%.6f,%.6f)", lat, lon)
}
