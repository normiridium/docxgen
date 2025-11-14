package modifiers

import (
	"fmt"
	"strings"
)

var (
	// NewLineInText - Line wrap within a paragraph
	NewLineInText = "</w:t><w:br/><w:t>"
)

// NewLine — Add a hyphen to a line.
func NewLine(s string) RawXML {
	return RawXML(s + NewLineInText)
}

// Prefix - Add a prefix to the string if it is not empty.
//
// Example:
//
//	{fio|prefix:`гражданин `} → "гражданин Иванов Иван Иванович"
func Prefix(s, p string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return p + s
}

// UniqPrefix - add a prefix only if the string is not empty and does not start with it yet.
//
// Example:
//
//	{org|uniqprefix:`ООО `} → "ООО Ромашка"
func UniqPrefix(s, p string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(s)), strings.ToLower(strings.TrimSpace(p))) {
		return s
	}
	return p + s
}

// Postfix — add a postfix to the string if it is not empty.
//
// Example:
//
//	{sum|postfix:` руб.`} → "100 руб."
func Postfix(s, p string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return s + p
}

// UniqPostfix — add a postfix only if the line is not empty and does not end with it yet.
//
// Example:
//
//	{city|uniqpostfix:` г.`} → "Москва г."
func UniqPostfix(s, p string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	if strings.HasSuffix(strings.ToLower(strings.TrimSpace(s)), strings.ToLower(strings.TrimSpace(p))) {
		return s
	}
	return s + p
}

// DefaultValue - Return the default value if the string is empty.
//
// Example:
//
//	{position|default:`сотрудник`} → "сотрудник"
func DefaultValue(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// Filled — return the specified value if the string is not empty; otherwise it is empty.
//
// Example:
//
//	{passport|filled:`—`} → "—"
func Filled(val any, out string) string {
	// nil → empty
	if val == nil {
		return ""
	}
	// For the line, check the empty
	if s, ok := val.(string); ok {
		if s == "" {
			return ""
		}
		return out
	}
	// for everything else, we just believe that "is"
	return out
}

// Replace - Replace all occurrences of the substring.
//
// Example:
//
//	{address|replace:`Москва`:`Санкт-Петербург`}
func Replace(s, old, new string) string {
	return strings.ReplaceAll(s, old, new)
}

// Truncate - Trim the string to n characters, add a tail if necessary (e.g. "...").
//
// Example:
//
//	{title|truncate:`20`:`…`} → "Очень длинное заг…"
func Truncate(s string, n int, suffix string) string {
	if n <= 0 {
		return ""
	}
	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + suffix
}

// WordReverse - Changes the order of words in a string to reverse.
//
// Examples:
//
//	{fio|word_reverse}
//	  "Фамилия Имя Отчество" → "Отчество Имя Фамилия"
//
//	{phrase|word_reverse}
//	  "раз два три четыре" → "четыре три два раз"
func WordReverse(s string) string {
	parts := strings.Fields(s)
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}
	return strings.Join(parts, " ")
}

// ConcatFactory returns a concat function that glues a string with tags or text.
// The last parameter is always considered a separator.
//
// Examples:
//
//	{own_org_name|concat:`own_org_address`:`position`:`department`:`, `}
//	→ "ООО Рога, Москва, Отдел продаж"
//
//	{abc_tag|concat:`не тег`:`def_tag`:` разделитель `}
//	→ "A разделитель не тег разделитель B"
//
//	{fio|concat:`prefix`:`postfix`:`, `}
//	→ "Иванов Иван Иванович, prefix, postfix"
func ConcatFactory(data map[string]any) func(base string, parts ...string) string {
	return func(base string, parts ...string) string {
		if len(parts) == 0 {
			return base
		}

		sep := parts[len(parts)-1]
		chunks := make([]string, 0, len(parts)+1)

		if strings.TrimSpace(base) != "" {
			chunks = append(chunks, base)
		}

		for _, p := range parts[:len(parts)-1] {
			val := strings.TrimSpace(p)

			if v, ok := data[val]; ok { // if it matches tag
				if str, ok := v.(string); ok {
					val = str
				} else {
					val = fmt.Sprint(v)
				}
			}

			if strings.TrimSpace(val) != "" {
				chunks = append(chunks, val)
			}
		}

		return strings.Join(chunks, sep)
	}
}
