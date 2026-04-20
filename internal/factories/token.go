package factories

import (
	"fmt"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type tokenMech struct{}

func (tokenMech) Name() string   { return "token" }
func (tokenMech) Tags() []string { return []string{"token", "api_key", "secret", "access_token", "auth_token"} }

func (tokenMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	return textNameMatch(ctx.Column, "token", tokenMech{}.Tags())
}

func (tokenMech) Generate(ctx seedapi.GenContext) any {
	length := ctx.Params.Int("length", 32)
	b := make([]byte, length/2+1)
	for i := range b {
		b[i] = byte(ctx.Rng.IntN(256))
	}
	return fmt.Sprintf("%x", b)[:length]
}
