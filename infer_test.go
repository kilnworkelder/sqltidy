package sqltidy

import (
	"encoding/json"
	"strings"
	"testing"
)

func newReader(s string) *strings.Reader { return strings.NewReader(s) }

func TestInferColumnTypes(t *testing.T) {
	tests := []struct {
		name   string
		values []any
		want   columnType
	}{
		{"integers", []any{"1", "2", "-3"}, typeInteger},
		{"leading zeros", []any{"001", "002"}, typeText},
		{"mixed int and real", []any{"1", "2.5"}, typeReal},
		{"booleans", []any{"true", "false"}, typeInteger},
		{"int and text", []any{"1", "abc"}, typeText},
		{"empty ignored", []any{"", nil, "7"}, typeInteger},
		{"all empty", []any{"", nil}, typeUnknown},
		{"json integer", []any{json.Number("42")}, typeInteger},
		{"json float", []any{json.Number("4.2")}, typeReal},
		{"json large exponent", []any{json.Number("1e3")}, typeReal},
		{"object", []any{map[string]any{"a": 1}}, typeText},
		{"array", []any{[]any{1, 2}}, typeText},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rows := make([][]any, len(tt.values))
			for i, v := range tt.values {
				rows[i] = []any{v}
			}
			if got := inferColumnTypes(rows, 1)[0]; got != tt.want {
				t.Fatalf("infer = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestIsIntString(t *testing.T) {
	for _, s := range []string{"0", "12", "-7", "+9", "9223372036854775807"} {
		if !isIntString(s) {
			t.Errorf("isIntString(%q) = false, want true", s)
		}
	}
	for _, s := range []string{"", "007", "1.0", "1e3", "12a", "9223372036854775808", " 1 2 "} {
		if isIntString(s) {
			t.Errorf("isIntString(%q) = true, want false", s)
		}
	}
}
