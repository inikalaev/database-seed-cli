package mechanisms

import (
	"regexp"
	"strings"

	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

// nameMatches evaluates a lowered column name against any of the given regex
// patterns. Patterns are raw — callers include anchors/flags themselves.
func nameMatches(col seedapi.Column, patterns ...string) bool {
	n := strings.ToLower(col.Name)
	for _, p := range patterns {
		if ok, _ := regexp.MatchString(p, n); ok {
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
