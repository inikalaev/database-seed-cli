package factories

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

// localizedJSON generates a multilingual JSON object for jsonb columns that
// hold localized strings (e.g. {"ru": "...", "en": "..."}).
// It scores NameMatch for jsonb columns whose name suggests text content.
type localizedJSON struct{}

func (localizedJSON) Name() string   { return "localized_json" }
func (localizedJSON) Tags() []string { return nil }

var localizedNamePatterns = []string{
	"name", "title", "description", "text", "label", "caption",
	"heading", "localized", "button_name",
}

func (localizedJSON) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isJSON(ctx.Column) {
		return seedapi.NoMatch
	}
	col := strings.ToLower(ctx.Column.Name)
	for _, p := range localizedNamePatterns {
		if strings.Contains(col, p) {
			return seedapi.NameMatch
		}
	}
	return seedapi.NoMatch
}

func (localizedJSON) Generate(ctx seedapi.GenContext) any {
	locales := ctx.Params.String("locales", "ru,en")
	parts := strings.Split(locales, ",")
	entries := make([]string, 0, len(parts))
	for _, loc := range parts {
		loc = strings.TrimSpace(loc)
		key, _ := json.Marshal(loc)
		val, _ := json.Marshal(fmt.Sprintf("%s %d", ctx.Column.Name, ctx.Row))
		entries = append(entries, string(key)+":"+string(val))
	}
	return "{" + strings.Join(entries, ",") + "}"
}
