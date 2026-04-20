package factories

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

type genderMech struct{}

func (genderMech) Name() string   { return "gender" }
func (genderMech) Tags() []string { return []string{"gender", "sex"} }

func (genderMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	return textNameMatch(ctx.Column, "gender", genderMech{}.Tags())
}

func (genderMech) Generate(ctx seedapi.GenContext) any {
	if ctx.Row%2 == 0 {
		return "male"
	}
	return "female"
}
