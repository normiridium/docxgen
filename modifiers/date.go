package modifiers

import (
	"fmt"
	"strings"
	"time"
)

// DateFormat - Formats the date according to the Go pattern (for example, "01/02/2006").
//
// Examples:
//
//	{project.deadline|date_format:`02.01.2006`} → "01.03.2026"
//	{created_at|date_format:`02.01.2006 15:04`} → "14.10.2025 08:30"
func DateFormat(val any, layout string) string {
	if val == nil {
		return ""
	}

	var t time.Time

	switch v := val.(type) {
	case time.Time:
		t = v

	case *time.Time:
		if v != nil {
			t = *v
		}

	case string:
		s := strings.TrimSpace(v)
		if s == "" {
			return ""
		}

		// Порядок попыток разбора строковых форматов
		tryFormats := []string{
			time.RFC3339,          // 2025-10-14T22:15:00Z
			"2006-01-02",          // 2025-10-14
			"02.01.2006",          // 14.10.2025
			"2006/01/02",          // 2025/10/14
			"02.01.2006 15:04",    // 14.10.2025 08:30
			"2006-01-02 15:04:05", // 2025-10-14 22:15:00
			time.ANSIC,            // Mon Jan _2 15:04:05 2006
		}
		for _, f := range tryFormats {
			if parsed, err := time.Parse(f, s); err == nil {
				t = parsed
				break
			}
		}
		if t.IsZero() {
			// если не смогли распознать — вернём исходное
			return s
		}

	case int64:
		t = time.Unix(v, 0)

	case float64:
		t = time.Unix(int64(v), 0)

	default:
		s := strings.TrimSpace(fmt.Sprint(v))
		if s == "" {
			return ""
		}
		if parsed, err := time.Parse(time.RFC3339, s); err == nil {
			t = parsed
		} else {
			return s
		}
	}

	if t.IsZero() {
		return ""
	}

	return t.Format(layout)
}
