package factories

import (
	"math"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

type latitudeMech struct{}

func (latitudeMech) Name() string   { return "latitude" }
func (latitudeMech) Tags() []string { return []string{"latitude", "lat"} }

func (latitudeMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isNumeric(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `(^|_)lat(itude)?(_|$)`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

func (latitudeMech) Generate(ctx seedapi.GenContext) any {
	v := -90.0 + ctx.Rng.Float64()*180.0
	return math.Round(v*1e6) / 1e6
}

type longitudeMech struct{}

func (longitudeMech) Name() string   { return "longitude" }
func (longitudeMech) Tags() []string { return []string{"longitude", "lon", "lng"} }

func (longitudeMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isNumeric(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `(^|_)lo?n(gitude)?(_|$)`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

func (longitudeMech) Generate(ctx seedapi.GenContext) any {
	v := -180.0 + ctx.Rng.Float64()*360.0
	return math.Round(v*1e6) / 1e6
}
