package factories

import (
	"math"
	"math/rand/v2"
	"regexp"
	"strings"
	"sync"

	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

// inclusiveIntN draws a uniform int from [lo, hi] inclusive. Callers pass
// user-supplied bounds, so we handle two nasty edges:
//   - hi <= lo: nothing to randomise, return lo.
//   - hi - lo == math.MaxInt: the naive `IntN(hi-lo+1)` overflows to MinInt and
//     panics. We fall back to a full-range draw when the span saturates, which
//     gives a uniform sample across the entire int domain above lo.
func inclusiveIntN(rng *rand.Rand, lo, hi int) int {
	if hi <= lo {
		return lo
	}
	span := hi - lo
	if span == math.MaxInt {
		return lo + rng.IntN(math.MaxInt)
	}
	return lo + rng.IntN(span+1)
}

var regexpCache sync.Map // map[string]*regexp.Regexp

func compilePattern(p string) *regexp.Regexp {
	if v, ok := regexpCache.Load(p); ok {
		return v.(*regexp.Regexp)
	}
	re := regexp.MustCompile(p)
	regexpCache.Store(p, re)
	return re
}

// nameMatches evaluates a lowered column name against any of the given regex
// patterns. Patterns are raw — callers include anchors/flags themselves.
// Compiled regexps are cached package-globally to avoid recompilation in hot paths.
func nameMatches(col seedapi.Column, patterns ...string) bool {
	n := strings.ToLower(col.Name)
	for _, p := range patterns {
		if compilePattern(p).MatchString(n) {
			return true
		}
	}
	return false
}

func isText(col seedapi.Column) bool {
	switch strings.ToLower(col.DataType) {
	case "text", "character varying", "varchar", "character", "char", "citext":
		return true
	}
	return false
}

func isInt(col seedapi.Column) bool {
	switch strings.ToLower(col.DataType) {
	case "smallint", "integer", "bigint", "int", "int2", "int4", "int8":
		return true
	}
	return false
}

func isNumeric(col seedapi.Column) bool {
	switch strings.ToLower(col.DataType) {
	case "numeric", "decimal", "real", "double precision", "float", "float4", "float8":
		return true
	}
	return false
}

func isBool(col seedapi.Column) bool {
	return strings.EqualFold(col.DataType, "boolean") || strings.EqualFold(col.UDTName, "bool")
}

func isTimestamp(col seedapi.Column) bool {
	switch strings.ToLower(col.DataType) {
	case "timestamp without time zone", "timestamp with time zone", "timestamptz", "timestamp":
		return true
	}
	return false
}

func isDate(col seedapi.Column) bool {
	return strings.EqualFold(col.DataType, "date")
}

func isJSON(col seedapi.Column) bool {
	switch strings.ToLower(col.DataType) {
	case "json", "jsonb":
		return true
	}
	return false
}

func isUUID(col seedapi.Column) bool {
	return strings.EqualFold(col.DataType, "uuid")
}

func isBytea(col seedapi.Column) bool {
	return strings.EqualFold(col.DataType, "bytea")
}

// textNameMatch reproduces registry.autoMatch scoring but gated on text types.
// Name-based factories (first_name, email, url, …) call this so they don't
// claim non-text columns by accident — an "email" column typed as integer
// would otherwise get quoted strings generated into it.
func textNameMatch(col seedapi.Column, name string, tags []string) seedapi.MatchScore {
	if !isText(col) {
		return seedapi.NoMatch
	}
	colNorm := normName(col.Name)
	if colNorm == normName(name) {
		return seedapi.StrongMatch
	}
	for _, tag := range tags {
		if strings.Contains(colNorm, normName(tag)) {
			return seedapi.NameMatch
		}
	}
	return seedapi.NoMatch
}

// NormName normalises a column or factory name for case-insensitive matching:
// lowercase, underscores and hyphens stripped.
func NormName(s string) string {
	s = strings.ToLower(s)
	s = strings.ReplaceAll(s, "_", "")
	s = strings.ReplaceAll(s, "-", "")
	return s
}

func normName(s string) string { return NormName(s) }

func contains(s, sub string) bool {
	return strings.Contains(s, sub)
}
