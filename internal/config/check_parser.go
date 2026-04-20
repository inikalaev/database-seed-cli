package config

import (
	"regexp"
	"strconv"
	"strings"
)

// applyCheckConstraints merges parseable CHECK expressions into column params.
// Returns the set of check names that were fully recognized so validate can
// mark the rest as unparsed.
func applyCheckConstraints(t *Table) map[string]bool {
	recognized := map[string]bool{}
	for _, chk := range t.Checks {
		// Single-column checks only. Multi-column requires correlated
		// generation which the MVP doesn't attempt.
		if len(chk.Columns) != 1 {
			continue
		}
		colName := chk.Columns[0]
		col, ok := t.Columns[colName]
		if !ok || col.Removed {
			continue
		}
		if parseInto(col, colName, chk.Expression) {
			recognized[chk.Name] = true
		}
	}
	return recognized
}

// parseInto attempts to interpret a CHECK expression for a single column and
// merges results into col.Params. Returns true iff every conjunct was
// recognized. Unrecognized conjuncts cause the whole check to fall through so
// validate can surface it — a partial match would silently relax constraints.
func parseInto(col *ColumnSpec, colName, expr string) bool {
	e := normalizeCheckExpr(expr)
	out := map[string]any{}
	// Try whole expression as a single conjunct first (e.g. BETWEEN contains
	// the keyword AND internally — we must not split it).
	if !parseConjunct(e, colName, out) {
		out = map[string]any{}
		for _, p := range splitTopLevelAND(e) {
			if !parseConjunct(p, colName, out) {
				return false
			}
		}
	}
	if len(out) == 0 {
		return false
	}
	return applyParsedParams(col, out)
}

func applyParsedParams(col *ColumnSpec, out map[string]any) bool {
	if col.Params == nil {
		col.Params = map[string]any{}
	}
	// params.max_len already carries varchar(N). Take the tighter of the two.
	if newMax, ok := out["max_len"].(int); ok {
		if cur, ok := col.Params["max_len"].(int); ok && cur > 0 && cur < newMax {
			delete(out, "max_len")
		}
	}
	for k, v := range out {
		col.Params[k] = v
	}
	// If the CHECK enumerates a literal set, the column is effectively an enum.
	// Reroute the factory so generation samples from the parsed list. The
	// column was auto-inferred here — user edits after init override via merge.
	if _, ok := out["values"]; ok {
		col.Factory = "enum_value"
		col.Unresolved = false
	}
	return true
}

func parseConjunct(e, colName string, out map[string]any) bool {
	e = strings.TrimSpace(stripOuterParens(e))
	if n := tryLengthBound(e, colName); n > 0 {
		mergeIntParam(out, "max_len", n)
		return true
	}
	if p := tryBetween(e, colName); p != nil {
		mergeIntParam(out, "min", p["min"])
		mergeIntParam(out, "max", p["max"])
		return true
	}
	if p := tryInList(e, colName); p != nil {
		out["values"] = p["values"]
		return true
	}
	if ok, key, v := tryBinaryCompare(e, colName); ok {
		if key == "values" {
			out["values"] = v
		} else {
			mergeIntParam(out, key, v)
		}
		return true
	}
	return false
}

var (
	reBetween = regexp.MustCompile(`^` + identPattern + `\s+BETWEEN\s+(-?\d+(?:\.\d+)?)\s+AND\s+(-?\d+(?:\.\d+)?)$`)
	reLength  = regexp.MustCompile(`^(?:char_length|length)\(` + identPattern + `\)\s*(<=|<)\s*(\d+)$`)
	reCompare = regexp.MustCompile(`^` + identPattern + `\s*(>=|<=|>|<|=)\s*(-?\d+(?:\.\d+)?)$`)
	reAnyList = regexp.MustCompile(`^` + identPattern + `\s*=\s*ANY\s*\(\s*ARRAY\[(.+)\]\s*\)$`)
	reInList  = regexp.MustCompile(`^` + identPattern + `\s+IN\s*\((.+)\)$`)
	// identPattern matches the column name: optional quotes, optional parens,
	// optional trailing ::type cast. Normalization drops casts but the regex
	// stays lenient for hand-written expressions.
	identPattern = `"?([a-zA-Z_][a-zA-Z0-9_]*)"?`
)

func tryBetween(e, colName string) map[string]any {
	m := reBetween.FindStringSubmatch(e)
	if m == nil || !strings.EqualFold(m[1], colName) {
		return nil
	}
	lo, err1 := parseNumericLiteral(m[2])
	hi, err2 := parseNumericLiteral(m[3])
	if err1 != nil || err2 != nil {
		return nil
	}
	return map[string]any{"min": lo, "max": hi}
}

func tryLengthBound(e, colName string) int {
	m := reLength.FindStringSubmatch(e)
	if m == nil || !strings.EqualFold(m[1], colName) {
		return 0
	}
	n, err := strconv.Atoi(m[3])
	if err != nil || n <= 0 {
		return 0
	}
	if m[2] == "<" {
		n--
	}
	return n
}

func tryBinaryCompare(e, colName string) (bool, string, any) {
	m := reCompare.FindStringSubmatch(e)
	if m == nil || !strings.EqualFold(m[1], colName) {
		return false, "", nil
	}
	op := m[2]
	v, err := parseNumericLiteral(m[3])
	if err != nil {
		return false, "", nil
	}
	switch op {
	case ">=":
		return true, "min", v
	case ">":
		return true, "min", bumpUp(v)
	case "<=":
		return true, "max", v
	case "<":
		return true, "max", bumpDown(v)
	case "=":
		// Reduce to both bounds equal — handled as values list instead to
		// keep generation deterministic.
		return true, "values", []any{v}
	}
	return false, "", nil
}

func tryInList(e, colName string) map[string]any {
	if m := reAnyList.FindStringSubmatch(e); m != nil && strings.EqualFold(m[1], colName) {
		if vals := parseLiteralList(m[2]); vals != nil {
			return map[string]any{"values": vals}
		}
	}
	if m := reInList.FindStringSubmatch(e); m != nil && strings.EqualFold(m[1], colName) {
		if vals := parseLiteralList(m[2]); vals != nil {
			return map[string]any{"values": vals}
		}
	}
	return nil
}

// normalizeCheckExpr removes PG's cosmetic noise so regex matching has a
// chance. Drops ::type_name casts, `(expr)::type` wrappers around bare idents,
// and trims one layer of outer parens that pg_get_expr adds.
func normalizeCheckExpr(e string) string {
	e = strings.TrimSpace(e)
	// Drop `::type_name` casts including schema-qualified and parameterized
	// forms: `::character varying`, `::numeric(10,2)`, `::public.status`.
	castRe := regexp.MustCompile(`::[a-zA-Z_][a-zA-Z0-9_ .]*(\([^)]*\))?`)
	e = castRe.ReplaceAllString(e, "")
	// `(col)::text` becomes `(col)` after cast removal — unwrap single-ident parens.
	unwrapIdentRe := regexp.MustCompile(`\(\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*\)`)
	e = unwrapIdentRe.ReplaceAllString(e, "$1")
	// Same trick for numeric literals — `> (0)` is normalized to `> 0`.
	unwrapNumRe := regexp.MustCompile(`\(\s*(-?\d+(?:\.\d+)?)\s*\)`)
	e = unwrapNumRe.ReplaceAllString(e, "$1")
	e = stripOuterParens(e)
	return strings.TrimSpace(e)
}

func stripOuterParens(e string) string {
	e = strings.TrimSpace(e)
	for len(e) >= 2 && e[0] == '(' && e[len(e)-1] == ')' && isBalanced(e[1:len(e)-1]) {
		e = strings.TrimSpace(e[1 : len(e)-1])
	}
	return e
}

func isBalanced(e string) bool {
	depth := 0
	for _, r := range e {
		switch r {
		case '(':
			depth++
		case ')':
			depth--
			if depth < 0 {
				return false
			}
		}
	}
	return depth == 0
}

// splitTopLevelAND splits on `AND` (case-insensitive) that sit at paren depth 0.
func splitTopLevelAND(e string) []string {
	var out []string
	depth := 0
	last := 0
	up := strings.ToUpper(e)
	for i := 0; i < len(e); i++ {
		c := e[i]
		switch c {
		case '(':
			depth++
		case ')':
			depth--
		}
		if depth == 0 && i+5 <= len(e) && up[i:i+5] == " AND " {
			out = append(out, strings.TrimSpace(e[last:i]))
			last = i + 5
			i += 4
		}
	}
	out = append(out, strings.TrimSpace(e[last:]))
	return out
}

// parseLiteralList parses a comma-separated literal list: numeric or single-quoted.
func parseLiteralList(raw string) []any {
	parts := splitTopLevelComma(raw)
	out := make([]any, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if len(p) >= 2 && p[0] == '\'' && p[len(p)-1] == '\'' {
			out = append(out, strings.ReplaceAll(p[1:len(p)-1], "''", "'"))
			continue
		}
		if n, err := parseNumericLiteral(p); err == nil {
			out = append(out, n)
			continue
		}
		return nil
	}
	return out
}

func splitTopLevelComma(raw string) []string {
	var out []string
	depth := 0
	inStr := false
	last := 0
	for i := 0; i < len(raw); i++ {
		c := raw[i]
		switch c {
		case '\'':
			if inStr && i+1 < len(raw) && raw[i+1] == '\'' {
				i++ // escaped quote
				continue
			}
			inStr = !inStr
		case '(':
			if !inStr {
				depth++
			}
		case ')':
			if !inStr {
				depth--
			}
		case ',':
			if !inStr && depth == 0 {
				out = append(out, raw[last:i])
				last = i + 1
			}
		}
	}
	out = append(out, raw[last:])
	return out
}

func parseNumericLiteral(s string) (any, error) {
	s = strings.TrimSpace(s)
	if strings.ContainsAny(s, ".eE") {
		f, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil, err
		}
		return f, nil
	}
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil, err
	}
	return int(n), nil
}

// bumpUp/bumpDown turn strict inequalities into inclusive params. Only
// integer-stepped — floats stay as-is (practical impact on random generation
// is negligible).
func bumpUp(v any) any {
	if i, ok := v.(int); ok {
		return i + 1
	}
	return v
}

func bumpDown(v any) any {
	if i, ok := v.(int); ok {
		return i - 1
	}
	return v
}

// mergeIntParam writes key=v unless an existing entry is stricter. Multiple
// conjuncts on the same column pick the tightest bound: max(min), min(max),
// min(max_len). Handles int and float64 uniformly (CHECK on NUMERIC produces
// floats from parseNumericLiteral).
func mergeIntParam(out map[string]any, key string, v any) {
	cur, exists := out[key]
	if !exists {
		out[key] = v
		return
	}
	cf, ok1 := asFloat(cur)
	nf, ok2 := asFloat(v)
	if !ok1 || !ok2 {
		return
	}
	switch key {
	case "min":
		if nf > cf {
			out[key] = v
		}
	case "max", "max_len":
		if nf < cf {
			out[key] = v
		}
	}
}

func asFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case int:
		return float64(t), true
	case int64:
		return float64(t), true
	case float64:
		return t, true
	case float32:
		return float64(t), true
	}
	return 0, false
}
