package factories

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

type portMech struct{}

func (portMech) Name() string   { return "port" }
func (portMech) Tags() []string { return []string{"port"} }

func (portMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isInt(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `(^|_)port(_|$)`) {
		return seedapi.NameMatch
	}
	return seedapi.NoMatch
}

// Generate returns an unprivileged port in [1024, 65535].
func (portMech) Generate(ctx seedapi.GenContext) any {
	lo := ctx.Params.Int("min", 1024)
	hi := ctx.Params.Int("max", 65535)
	return inclusiveIntN(ctx.Rng, lo, hi)
}
