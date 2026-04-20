package factories

import (
	"fmt"
	"strings"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

// textMech is the intentional weak fallback for any text column that no other
// mechanism claimed. It scores WeakMatch so the registry flags the column as
// `unresolved: true` — the user should see it and decide.
type textMech struct{}

func (textMech) Name() string   { return "string" }
func (textMech) Tags() []string { return []string{"text"} }

func (textMech) UniquePerRow() bool { return true }

func (textMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if isText(ctx.Column) {
		return seedapi.WeakMatch
	}
	return seedapi.NoMatch
}

func (textMech) Generate(ctx seedapi.GenContext) any {
	return fmt.Sprintf("%s_%d", strings.ToLower(ctx.Column.Name), ctx.Row)
}
