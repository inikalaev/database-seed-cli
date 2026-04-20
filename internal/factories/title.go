package factories

import (
	"fmt"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type titleMech struct{}

func (titleMech) Name() string   { return "title" }
func (titleMech) Tags() []string { return []string{"title", "headline", "heading", "caption", "subject"} }

func (titleMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	return textNameMatch(ctx.Column, "title", titleMech{}.Tags())
}

func (titleMech) Generate(ctx seedapi.GenContext) any {
	adjectives := []string{"Новый", "Современный", "Основной", "Передовой", "Практический", "Базовый", "Расширенный", "Специальный"}
	nouns := []string{"курс", "модуль", "раздел", "урок", "блок", "тема", "материал", "проект"}
	adj := adjectives[ctx.Rng.IntN(len(adjectives))]
	noun := nouns[ctx.Rng.IntN(len(nouns))]
	return fmt.Sprintf("%s %s %d", adj, noun, ctx.Row)
}
