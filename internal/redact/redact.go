// Package redact normalizes and masks untrusted portal fields before they reach
// one-record-per-line output.
package redact

import (
	"fmt"
	"strings"
	"time"
	"unicode"
)

// Normalize collapses control characters and whitespace runs into single
// spaces, so a field can never start a new output record.
func Normalize(value string) string {
	cleaned := strings.Map(func(char rune) rune {
		if unicode.IsControl(char) || unicode.IsSpace(char) {
			return ' '
		}
		return char
	}, value)
	return strings.Join(strings.Fields(cleaned), " ")
}

// Field returns the normalized value, or fallback when it is empty or the
// formatted form of a missing JSON value.
func Field(value, fallback string) string {
	value = Normalize(value)
	if value == "" || value == "<nil>" {
		return fallback
	}
	return value
}

// Text keeps the first and last rune of a name and masks the rest.
func Text(value string) string {
	runes := []rune(Normalize(value))
	switch len(runes) {
	case 0:
		return "unknown"
	case 1:
		return "*"
	case 2:
		return string(runes[:1]) + "*"
	}
	return string(runes[:1]) + strings.Repeat("*", len(runes)-2) + string(runes[len(runes)-1:])
}

// Account keeps the first and last three runes of an account identifier.
func Account(value string) string {
	value = Normalize(value)
	runes := []rune(value)
	if len(runes) <= 6 {
		return Text(value)
	}
	return string(runes[:3]) + "***" + string(runes[len(runes)-3:])
}

// MAC keeps the first two octets and the last octet of a colon-separated MAC.
func MAC(value string) string {
	value = Normalize(value)
	parts := strings.Split(value, ":")
	if len(parts) != 6 {
		if value == "" {
			return "unknown"
		}
		return "redacted"
	}
	return strings.Join([]string{parts[0], parts[1], "xx", "xx", "xx", parts[5]}, ":")
}

// Unix formats a positive Unix timestamp in local time.
func Unix(value int64) string {
	if value <= 0 {
		return "unknown"
	}
	return time.Unix(value, 0).Format("2006-01-02 15:04:05")
}

// AnyUnix formats a timestamp decoded from untyped JSON.
func AnyUnix(value any) string {
	switch cast := value.(type) {
	case float64:
		return Unix(int64(cast))
	case int64:
		return Unix(cast)
	case int:
		return Unix(int64(cast))
	default:
		return "unknown"
	}
}

// Presence reports whether an untyped JSON value is present without revealing it.
func Presence(value any) string {
	text := strings.TrimSpace(fmt.Sprint(value))
	if text == "" || text == "<nil>" {
		return "none"
	}
	return "present"
}
