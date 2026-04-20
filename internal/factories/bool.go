package factories

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

type boolMech struct{}

func (boolMech) Name() string   { return "bool" }
func (boolMech) Tags() []string { return []string{"logic"} }

func (boolMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	// Boolean is unambiguous (true/false). WeakNameMatch keeps it out of unresolved
	// while still letting any user plugin with a name-based signal win.
	if isBool(ctx.Column) {
		return seedapi.WeakNameMatch
	}
	return seedapi.NoMatch
}

func (boolMech) Generate(ctx seedapi.GenContext) any {
	return ctx.Rng.IntN(2) == 1
}
