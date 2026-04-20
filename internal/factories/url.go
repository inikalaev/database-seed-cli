package factories

import (
	"fmt"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

// urlMech is a NameMatch-tier mechanism for columns clearly holding URLs. It
// does not check string length or regex shape — the user can tighten via a
// custom mechanism if they need constrained domains.
type urlMech struct{}

func (urlMech) Name() string   { return "url" }
func (urlMech) Tags() []string { return []string{"link", "website", "homepage", "url"} }

func (urlMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	return textNameMatch(ctx.Column, "url", urlMech{}.Tags())
}

func (urlMech) Generate(ctx seedapi.GenContext) any {
	return fmt.Sprintf("https://example.com/%d", ctx.Row)
}
