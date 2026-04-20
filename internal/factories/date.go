package factories

import (
	"time"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type dateMech struct{}

func (dateMech) Name() string   { return "date" }
func (dateMech) Tags() []string { return []string{"time"} }

func (dateMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	// Тип date однозначен, дефолт "случайная дата в окне" осмыслен — возвращаем
	// WeakNameMatch: не unresolved, но плагин с NameMatch перебьёт.
	if isDate(ctx.Column) {
		return seedapi.WeakNameMatch
	}
	return seedapi.NoMatch
}

func (dateMech) Generate(ctx seedapi.GenContext) any {
	// Anchor 2024-01-01 — matches timestamp.go for consistency.
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	return base.AddDate(0, 0, ctx.Rng.IntN(365*5)).Format("2006-01-02")
}
