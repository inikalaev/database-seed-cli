package mechanisms

import (
	"fmt"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type phoneMech struct{}

func (phoneMech) Name() string   { return "phone" }
func (phoneMech) Tags() []string { return []string{"contact"} }

func (phoneMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isText(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `phone`, `tel`, `mobile`) {
		return seedapi.StrongMatch
	}
	return seedapi.NoMatch
}

func (phoneMech) Generate(ctx seedapi.GenContext) any {
	// International dialing codes + 8-digit subscriber number.
	codes := []string{"+1", "+7", "+44", "+49", "+33", "+81", "+86", "+91", "+55", "+52", "+61", "+27", "+234", "+971"}
	code := codes[ctx.Rng.IntN(len(codes))]
	return fmt.Sprintf("%s%08d", code, ctx.Rng.Int64N(100_000_000))
}
