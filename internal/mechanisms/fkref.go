package mechanisms

import (
	"strings"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

// fkRef samples a previously-generated PK from the referenced table. Matches
// whenever the column has a FKTarget (set by the config builder from schema
// introspection) and always beats name/type-based mechanisms.
type fkRef struct{}

func (fkRef) Name() string   { return "fkref" }
func (fkRef) Tags() []string { return []string{"fk"} }

func (fkRef) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if ctx.Column.FKTarget != "" {
		return seedapi.FKMatch
	}
	return seedapi.NoMatch
}

func (fkRef) Generate(ctx seedapi.GenContext) any {
	target := ctx.Params.String("target", ctx.Column.FKTarget)
	if target == "" {
		return nil
	}
	parts := strings.Split(target, ".")
	var s, t, c string
	switch len(parts) {
	case 3:
		s, t, c = parts[0], parts[1], parts[2]
	case 2:
		s, t, c = "public", parts[0], parts[1]
	default:
		return nil
	}
	if v, ok := ctx.FKPool.Pick(s, t, c, ctx.Rng); ok {
		return v
	}
	return nil
}
