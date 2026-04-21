package factories

import (
	"fmt"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

type versionStrMech struct{}

func (versionStrMech) Name() string   { return "version_str" }
func (versionStrMech) Tags() []string { return []string{"version", "semver", "app_version"} }

func (versionStrMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isText(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `(^|_)version(_|$)`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

func (versionStrMech) Generate(ctx seedapi.GenContext) any {
	major := ctx.Rng.IntN(5)
	minor := ctx.Rng.IntN(20)
	patch := ctx.Rng.IntN(100)
	return fmt.Sprintf("%d.%d.%d", major, minor, patch)
}
