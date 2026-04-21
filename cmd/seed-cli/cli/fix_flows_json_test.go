package cli

import (
	"testing"

	"github.com/inikalaev/database-seed-cli/internal/config"
)

func TestLookupJsonField_Depth1(t *testing.T) {
	col := &config.ColumnSpec{
		Values: map[string]*config.ColumnSpec{
			"plan": {Factory: "string"},
		},
	}
	got, ok := lookupJsonField(col, "plan")
	if !ok || got.Factory != "string" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
}

func TestLookupJsonField_Nested(t *testing.T) {
	col := &config.ColumnSpec{
		Values: map[string]*config.ColumnSpec{
			"addr": {Values: map[string]*config.ColumnSpec{
				"city": {Factory: "city"},
			}},
		},
	}
	got, ok := lookupJsonField(col, "addr.city")
	if !ok || got.Factory != "city" {
		t.Fatalf("got %+v ok=%v", got, ok)
	}
}

func TestLookupJsonField_Missing(t *testing.T) {
	col := &config.ColumnSpec{
		Values: map[string]*config.ColumnSpec{
			"addr": {Values: map[string]*config.ColumnSpec{
				"city": {Factory: "city"},
			}},
		},
	}
	if _, ok := lookupJsonField(col, "addr.ghost"); ok {
		t.Fatal("missing segment must return ok=false")
	}
	if _, ok := lookupJsonField(col, "addr.city.further"); ok {
		t.Fatal("walking past leaf must return ok=false")
	}
}

func TestLookupJsonField_EmptyPath(t *testing.T) {
	col := &config.ColumnSpec{Values: map[string]*config.ColumnSpec{"a": {}}}
	if _, ok := lookupJsonField(col, ""); ok {
		t.Fatal("empty path must return ok=false")
	}
	if _, ok := lookupJsonField(nil, "a"); ok {
		t.Fatal("nil col must return ok=false")
	}
}
