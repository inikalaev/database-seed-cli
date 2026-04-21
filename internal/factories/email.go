package factories

import (
	"fmt"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

type emailMech struct{}

func (emailMech) Name() string   { return "email" }
func (emailMech) Tags() []string { return []string{"e_mail", "email_address"} }

func (emailMech) UniquePerRow() bool { return true }

func (emailMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	return textNameMatch(ctx.Column, "email", emailMech{}.Tags())
}

func (emailMech) Generate(ctx seedapi.GenContext) any {
	domain := ctx.Params.String("domain", "example.com")
	return fmt.Sprintf("user%d_%d@%s", ctx.Row, ctx.Rng.IntN(10_000), domain)
}
