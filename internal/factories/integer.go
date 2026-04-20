package factories

import (
	"strings"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

// intMech — generic integer fallback. Owns WeakNameMatch для обычных integer-
// колонок (не уйдут в unresolved, но любой плагин с NameMatch перебьёт) и
// откатывается в TypeMatch (т.е. unresolved) для подозрительных имён:
// осиротевший `*_id` без FK и enum-подобные `status/type`.
type intMech struct{}

func (intMech) Name() string   { return "integer" }
func (intMech) Tags() []string { return []string{"numeric"} }

func (intMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !isInt(ctx.Column) {
		return seedapi.NoMatch
	}
	lower := strings.ToLower(ctx.Column.Name)
	// `*_id` без FKTarget — подозрительно: скорее всего это FK, у которой в БД
	// не объявлен constraint. Остаёмся на TypeMatch (→ unresolved), чтобы user
	// проверил (fkref + target).
	if strings.HasSuffix(lower, "_id") && ctx.Column.FKTarget == "" {
		return seedapi.TypeMatch
	}
	// Имена вокруг "status" / "type" (как отдельное слово с границами `_` или
	// начала/конца имени) почти всегда означают enum/дискриминатор — случайный
	// int даст бессмысленные значения. Оставляем unresolved — user явно решит
	// (enum_value, CHECK-range, и т.д.). Regex'ы с границей слова защищают от
	// false-positive: `prototype_number` или `subtype_sequence` теперь не
	// попадают сюда.
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
