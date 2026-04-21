package factories

import (
	"fmt"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

type slugMech struct{}

func (slugMech) Name() string   { return "slug" }
func (slugMech) Tags() []string { return []string{"slug"} }

func (slugMech) UniquePerRow() bool { return true }

func (slugMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	return textNameMatch(ctx.Column, "slug", slugMech{}.Tags())
}

func (slugMech) Generate(ctx seedapi.GenContext) any {
	words := []string{"alpha", "beta", "gamma", "delta", "epsilon", "zeta", "theta", "lambda", "sigma", "omega"}
	return fmt.Sprintf("%s-%s-%d", words[ctx.Rng.IntN(len(words))], words[ctx.Rng.IntN(len(words))], ctx.Row)
}
