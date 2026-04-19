package mechanisms

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

// fullName composes a first+last pair for columns named name / full_name /
// display_name. It delegates to firstName and lastName so pool updates land
// everywhere at once.
type fullName struct{}

func (fullName) Name() string   { return "full_name" }
func (fullName) Tags() []string { return []string{"person", "name"} }

func (fullName) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isText(ctx.Column) {
		return seedapi.NoMatch
	}
	if nameMatches(ctx.Column, `^full_?name$`, `^name$`, `^display_?name$`) {
		return seedapi.StrongMatch
	}
	return seedapi.NoMatch
}

func (fullName) Generate(ctx seedapi.GenContext) any {
	f := firstName{}.Generate(ctx).(string)
	l := lastName{}.Generate(ctx).(string)
	return f + " " + l
}
