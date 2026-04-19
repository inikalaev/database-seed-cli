package mechanisms

import (
	"fmt"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

// emailMech produces unique per-row addresses using params.domain (default
// example.com). Row index + random tail guarantees uniqueness without a
// separate set/bloom lookup.
type emailMech struct{}

func (emailMech) Name() string   { return "email" }
func (emailMech) Tags() []string { return []string{"contact"} }

func (emailMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isText(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `email`, `e_mail`) {
		return seedapi.StrongMatch
	}
	return seedapi.NoMatch
}

func (emailMech) Generate(ctx seedapi.GenContext) any {
	domain := ctx.Params.String("domain", "example.com")
	return fmt.Sprintf("user%d_%d@%s", ctx.Row, ctx.Rng.IntN(10_000), domain)
}
