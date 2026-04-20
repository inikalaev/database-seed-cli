package factories

import (
	"strings"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

// intMech is the generic integer fallback. It returns WeakNameMatch for ordinary
// integer columns (resolved, but any NameMatch plugin wins) and falls back to
// TypeMatch (unresolved) for suspicious names: orphan `*_id` without a FK target
// and enum-like `status`/`type` columns.
type intMech struct{}

func (intMech) Name() string   { return "integer" }
func (intMech) Tags() []string { return []string{"numeric"} }

func (intMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isInt(ctx.Column) {
		return seedapi.NoMatch
	}
	lower := strings.ToLower(ctx.Column.Name)
	// `*_id` without a FKTarget is suspicious: likely a FK with no declared
	// constraint. Stay at TypeMatch (→ unresolved) so the user can wire up fkref.
	if strings.HasSuffix(lower, "_id") && ctx.Column.FKTarget == "" {
		return seedapi.TypeMatch
	}
	// Names containing "status" or "type" as a whole word (bounded by `_` or
	// start/end of name) almost always signal an enum or discriminator — a random
	// int would be meaningless. Stay unresolved so the user picks explicitly
	// (enum_value, CHECK range, etc.). Word-boundary regexes prevent false
	// positives on names like `prototype_number` or `subtype_sequence`.
	if nameMatches(ctx.Column, `(^|_)status(_|$)`, `(^|_)type(_|$)`) {
		return seedapi.TypeMatch
	}
	return seedapi.WeakNameMatch
}

func (intMech) Generate(ctx seedapi.GenContext) any {
	lo := ctx.Params.Int("min", 0)
	hi := ctx.Params.Int("max", defaultIntMax(ctx.Column))
	return inclusiveIntN(ctx.Rng, lo, hi)
}

// defaultIntMax picks a safe upper bound based on the PG integer width so we
// never emit a value that overflows the column. Callers can still override via
// params["max"] — that path is always honored.
func defaultIntMax(col seedapi.Column) int {
	switch strings.ToLower(col.DataType) {
	case "smallint", "int2":
		return 32_767
	case "integer", "int", "int4":
		return 1_000_000
	}
	// bigint and anything unfamiliar: stay at the generic million-ceiling,
	// which fits comfortably inside int8.
	return 1_000_000
}
