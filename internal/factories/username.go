package factories

import (
	"fmt"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

type usernameMech struct{}

func (usernameMech) Name() string         { return "username" }
func (usernameMech) UniquePerRow() bool   { return true }
func (usernameMech) Tags() []string { return []string{"username", "user_name", "login", "login_name"} }

func (usernameMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	return textNameMatch(ctx.Column, "username", usernameMech{}.Tags())
}

var usernameAdj = []string{"fast", "bold", "quiet", "bright", "cool", "dark", "swift", "sharp"}

func (usernameMech) Generate(ctx seedapi.GenContext) any {
	adj := usernameAdj[ctx.Rng.IntN(len(usernameAdj))]
	return fmt.Sprintf("%s_user_%d", adj, ctx.Row)
}
