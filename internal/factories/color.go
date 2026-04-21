package factories

import (
	"fmt"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

type colorMech struct{}

func (colorMech) Name() string   { return "color" }
func (colorMech) Tags() []string { return []string{"color", "colour"} }

func (colorMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isText(ctx.Column) {
		return seedapi.NoMatch
	}
	col := normName(ctx.Column.Name)
	for _, tag := range (colorMech{}).Tags() {
		if contains(col, normName(tag)) {
			return seedapi.NameMatch
		}
	}
	return seedapi.NoMatch
}

func (colorMech) Generate(ctx seedapi.GenContext) any {
	r := ctx.Rng.IntN(256)
	g := ctx.Rng.IntN(256)
	b := ctx.Rng.IntN(256)
	return fmt.Sprintf("#%02X%02X%02X", r, g, b)
}
