package sqltidy

import (
	"encoding/json"
	"math"
	"strconv"
	"strings"
)

// columnType is the SQLite storage class sqltidy infers for a column.
type columnType string

const (
	typeUnknown columnType = ""
	typeInteger columnType = "INTEGER"
	typeReal    columnType = "REAL"
	typeText    columnType = "TEXT"
)

func (t columnType) sql() string {
	if t == typeUnknown {
		return string(typeText)
	}
	return string(t)
}

// inferColumnTypes walks the sample and resolves each column to the widest
// type required by its values: INTEGER < REAL < TEXT.
func inferColumnTypes(rows [][]any, ncols int) []columnType {
	types := make([]columnType, ncols)
	for _, row := range rows {
		for i := 0; i < ncols && i < len(row); i++ {
			types[i] = mergeType(types[i], row[i])
		}
	}
	return types
}

func mergeType(cur columnType, v any) columnType {
	if cur == typeText {
		return typeText
	}
	switch x := v.(type) {
	case nil:
		return cur
	case bool:
		if cur == typeReal {
			return typeReal
		}
		return typeInteger
	case int, int8, int16, int32, int64, uint, uint8, uint16, uint32, uint64:
		if cur == typeReal {
			return typeReal
		}
		return typeInteger
	case float32, float64:
		return typeReal
	case json.Number:
		s := string(x)
		if !strings.ContainsAny(s, ".eE") {
			if _, err := strconv.ParseInt(s, 10, 64); err == nil {
				if cur == typeReal {
					return typeReal
				}
				return typeInteger
			}
		}
		return typeReal
	case string:
		return mergeString(cur, x)
	case []byte:
		return mergeString(cur, string(x))
	default:
		// Objects and arrays are stored as JSON text.
		return typeText
	}
}

func mergeString(cur columnType, s string) columnType {
	s = strings.TrimSpace(s)
	if s == "" {
		return cur
	}
	if isBoolString(s) {
		if cur == typeReal {
			return typeReal
		}
		return typeInteger
	}
	if isIntString(s) {
		if cur == typeReal {
			return typeReal
		}
		return typeInteger
	}
	if isFloatString(s) {
		return typeReal
	}
	return typeText
}

func isBoolString(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "true", "false":
		return true
	}
	return false
}

// isIntString reports whether s is a base-10 integer that can be stored
// without losing information. Leading zeros are treated as text so values
// such as ZIP codes and phone numbers survive a round trip.
func isIntString(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	body := s
	if body[0] == '+' || body[0] == '-' {
		body = body[1:]
	}
	if body == "" {
		return false
	}
	if len(body) > 1 && body[0] == '0' {
		return false
	}
	for i := 0; i < len(body); i++ {
		if body[i] < '0' || body[i] > '9' {
			return false
		}
	}
	_, err := strconv.ParseInt(s, 10, 64)
	return err == nil
}

func isFloatString(s string) bool {
	s = strings.TrimSpace(s)
	if !strings.ContainsAny(s, ".eE") {
		return false
	}
	f, err := strconv.ParseFloat(s, 64)
	if err != nil || math.IsNaN(f) || math.IsInf(f, 0) {
		return false
	}
	return true
}

func boolInt(s string) int64 {
	if strings.EqualFold(strings.TrimSpace(s), "true") {
		return 1
	}
	return 0
}
