package config

import (
	"reflect"
	"testing"
)

func TestApplyCheckConstraints(t *testing.T) {
	cases := []struct {
		name   string
		expr   string
		col    string
		want   map[string]any
		parsed bool
	}{
		{
			name:   "gte_integer",
			expr:   "(age >= 0)",
			col:    "age",
			want:   map[string]any{"min": 0},
			parsed: true,
		},
		{
			name:   "strict_gt_integer",
			expr:   "(price > 0)",
			col:    "price",
			want:   map[string]any{"min": 1},
			parsed: true,
		},
		{
			name:   "numeric_with_cast",
			expr:   "(price > (0)::numeric)",
			col:    "price",
			want:   map[string]any{"min": 1},
			parsed: true,
		},
		{
			name:   "between",
			expr:   "(age BETWEEN 0 AND 120)",
			col:    "age",
			want:   map[string]any{"min": 0, "max": 120},
			parsed: true,
		},
		{
			name:   "and_range",
			expr:   "((age >= 0) AND (age <= 120))",
			col:    "age",
			want:   map[string]any{"min": 0, "max": 120},
			parsed: true,
		},
		{
			name:   "char_length",
			expr:   "(char_length((name)::text) <= 100)",
			col:    "name",
			want:   map[string]any{"max_len": 100},
			parsed: true,
		},
		{
			name:   "in_string",
			expr:   "(status = ANY (ARRAY['pending'::text, 'paid'::text]))",
			col:    "status",
			want:   map[string]any{"values": []any{"pending", "paid"}},
			parsed: true,
		},
		{
			name:   "in_literal",
			expr:   "(status IN ('a', 'b'))",
			col:    "status",
			want:   map[string]any{"values": []any{"a", "b"}},
			parsed: true,
		},
		{
			name:   "unparseable_function",
			expr:   "(now() > created_at)",
			col:    "created_at",
			parsed: false,
		},
		{
			name:   "unparseable_multi",
			expr:   "(start_date < end_date)",
			col:    "start_date",
			parsed: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			col := &ColumnSpec{}
			ok := parseInto(col, tc.col, tc.expr)
			if ok != tc.parsed {
				t.Fatalf("parsed=%v, want %v (params=%v)", ok, tc.parsed, col.Params)
			}
			if tc.parsed && !reflect.DeepEqual(col.Params, tc.want) {
				t.Fatalf("params=%v want %v", col.Params, tc.want)
			}
		})
	}
}

func TestApplyCheckConstraints_RespectsTighterMaxLen(t *testing.T) {
	col := &ColumnSpec{Params: map[string]any{"max_len": 50}}
	// CHECK says <=100 — varchar(50) is stricter, keep 50.
	if !parseInto(col, "name", "(char_length((name)::text) <= 100)") {
		t.Fatalf("expected parsed")
	}
	if got, _ := col.Params["max_len"].(int); got != 50 {
		t.Fatalf("max_len=%v, want 50", got)
	}
}

func TestApplyCheckConstraints_LoosensWhenTighter(t *testing.T) {
	col := &ColumnSpec{Params: map[string]any{"max_len": 255}}
	if !parseInto(col, "name", "(char_length((name)::text) <= 100)") {
		t.Fatalf("expected parsed")
	}
	if got, _ := col.Params["max_len"].(int); got != 100 {
		t.Fatalf("max_len=%v, want 100", got)
	}
}
