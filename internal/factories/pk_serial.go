package factories

import (
	"strings"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

// pkSerial produces sequential integers for primary keys. Matches conservatively:
// only a column literally named "id" with an integer type. Composite or
// non-standard PKs must be declared explicitly by the user.
type pkSerial struct{}

func (pkSerial) Name() string   { return "pk_serial" }
func (pkSerial) Tags() []string { return []string{"pk"} }

func (pkSerial) UniquePerRow() bool { return true }

func (pkSerial) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if strings.EqualFold(ctx.Column.Name, "id") && isInt(ctx.Column) {
		return seedapi.StrongMatch
	}
	return seedapi.NoMatch
}

func (pkSerial) Generate(ctx seedapi.GenContext) any {
	return ctx.Params.Int("start", 1) + ctx.Row
}
