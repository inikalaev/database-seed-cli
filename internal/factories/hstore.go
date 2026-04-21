package factories

import (
	"fmt"
	"strings"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

type hstoreMech struct{}

func (hstoreMech) Name() string   { return "hstore" }
func (hstoreMech) Tags() []string { return []string{"hstore"} }

func (hstoreMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	// The hstore UDT is unambiguous; generating key-value pairs is always sensible.
	// WeakNameMatch keeps it resolved but yields to any plugin returning NameMatch.
	if strings.EqualFold(ctx.Column.UDTName, "hstore") {
		return seedapi.WeakNameMatch
	}
	return seedapi.NoMatch
}

var hstoreKeys = []string{"lang", "locale", "theme", "mode", "version", "format", "encoding"}

func (hstoreMech) Generate(ctx seedapi.GenContext) any {
	n := ctx.Params.Int("pairs", 2)
	if n < 1 {
		n = 1
	}
	parts := make([]string, n)
	for i := range parts {
		k := hstoreKeys[ctx.Rng.IntN(len(hstoreKeys))]
		parts[i] = fmt.Sprintf(`"%s_%d"=>"%d"`, k, i, ctx.Rng.IntN(100))
	}
	return strings.Join(parts, ",")
}
