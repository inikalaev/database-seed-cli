package factories

import "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"

type boolMech struct{}

func (boolMech) Name() string   { return "bool" }
func (boolMech) Tags() []string { return []string{"logic"} }

func (boolMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	// Boolean — единственный тип с однозначной семантикой (true/false).
	// WeakNameMatch: достаточно, чтобы не попасть в unresolved, но ниже
	// NameMatch — любой пользовательский плагин с name-based сигналом победит.
	if isBool(ctx.Column) {
		return seedapi.WeakNameMatch
	}
	return seedapi.NoMatch
}

func (boolMech) Generate(ctx seedapi.GenContext) any {
	return ctx.Rng.IntN(2) == 1
}
