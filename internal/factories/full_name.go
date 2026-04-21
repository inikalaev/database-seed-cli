package factories

import "github.com/inikalaev/database-seed-cli/pkg/seedapi"

type fullName struct{}

func (fullName) Name() string   { return "full_name" }
func (fullName) Tags() []string { return []string{"display_name"} }

// Match implements seedapi.Matcher to additionally catch bare "name" columns.
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
