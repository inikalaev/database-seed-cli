package factories

import (
	"fmt"
	"math/rand/v2"
	"strings"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

// arrayMech handles 1-D arrays of primitive element types. The element family
// is derived from ctx.Column.UDTName (pg_catalog convention: `_text`, `_int4`,
// …). Nested arrays and composite/element-enum arrays are not supported and
// fall back to `string`/unresolved via NoMatch.
type arrayMech struct{}

func (arrayMech) Name() string   { return "array" }
func (arrayMech) Tags() []string { return nil }

func (arrayMech) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
	if !strings.EqualFold(ctx.Column.DataType, "ARRAY") {
		return seedapi.NoMatch
	}
	if ctx.Column.ArrayDims != 1 {
		return seedapi.NoMatch
	}
	if _, ok := arrayElementKind(ctx.Column.UDTName); !ok {
		return seedapi.NoMatch
	}
	return seedapi.StrongMatch
}

func (arrayMech) Generate(ctx seedapi.GenContext) any {
	// UDTName isn't carried through config.ColumnSpec, so build.go stashes it
	// in params at inference time. Fall back to the column's UDTName when the
	// factory is used programmatically (outside the CLI's config pipeline).
	udt := ctx.Params.String("element", "")
	if udt == "" {
		udt = ctx.Column.UDTName
	}
	kind, ok := arrayElementKind(udt)
	if !ok {
		return nil
	}
	n := ctx.Params.Int("size", 3)
	if n < 0 {
		n = 0
	}
	out := make([]any, n)
	for i := range out {
		out[i] = generateArrayElement(kind, ctx.Row, i, ctx.Rng)
	}
	return out
}

// arrayElementKind extracts the element type family from a pg_catalog array
// UDT name like `_text` → "text". Returns false for unsupported element types.
func arrayElementKind(udt string) (string, bool) {
	raw := strings.TrimPrefix(strings.ToLower(udt), "_")
	switch raw {
	case "text", "varchar", "character varying", "bpchar", "char", "citext":
		return "text", true
	case "int2", "int4", "int8", "smallint", "integer", "bigint", "int":
		return "int", true
	case "numeric", "decimal", "float4", "float8", "real", "double precision":
		return "numeric", true
	case "bool", "boolean":
		return "bool", true
	case "uuid":
		return "uuid", true
	case "json", "jsonb":
		// JSON arrays get minimal `{}::jsonb` placeholders. Enough to satisfy
		// jsonb[] columns without inventing realistic shapes — users override
		// with a custom factory when they need real data.
		return raw, true
	}
	return "", false
}

func generateArrayElement(kind string, row, idx int, rng *rand.Rand) any {
	switch kind {
	case "text":
		return fmt.Sprintf("item_%d_%d", row, idx)
	case "int":
		return rng.IntN(1000)
	case "numeric":
		return float64(rng.IntN(1_000_00)) / 100
	case "bool":
		return rng.IntN(2) == 1
	case "uuid":
		b := make([]byte, 16)
		for i := range b {
			b[i] = byte(rng.UintN(256))
		}
		b[6] = (b[6] & 0x0f) | 0x40
		b[8] = (b[8] & 0x3f) | 0x80
		s := fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
		// uuid element must be typed — `ARRAY[E'...']::uuid[]` isn't how
		// formatSQL renders it, so cast per-element instead.
		return seedapi.Cast{Value: s, Type: "uuid"}
	case "json", "jsonb":
		// Wrap in Cast so formatSQL emits `'{}'::jsonb` — plain quoted strings
		// inside an `ARRAY[...]` literal would fail the element-type check.
		return seedapi.Cast{Value: "{}", Type: kind}
	}
	return nil
}
