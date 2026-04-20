package factories

import (
	"fmt"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

// jsonAny emits a minimal JSON object so the column is not NULL. Real-world
// jsonb shapes vary too widely for a useful default — override with a custom
// mechanism or set unresolved:false after editing params.
type jsonAny struct{}

func (jsonAny) Name() string   { return "json_any" }
func (jsonAny) Tags() []string { return []string{"json"} }

func (jsonAny) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if isJSON(ctx.Column) {
		return seedapi.TypeMatch
	}
	return seedapi.NoMatch
}

func (jsonAny) Generate(ctx seedapi.GenContext) any {
	return fmt.Sprintf(`{"row":%d}`, ctx.Row)
}
