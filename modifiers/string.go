package modifiers

import (
	"fmt"
	"strings"
)

// Prefix — добавить префикс к строке, если она не пустая.
//
// Пример:
//
//	{fio|prefix:`гражданин `} → "гражданин Иванов Иван Иванович"
func Prefix(s, p string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return p + s
}

// UniqPrefix — добавить префикс только если строка непустая и ещё не начинается с него.
//
// Пример:
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

// Postfix — добавить постфикс к строке, если она не пустая.
//
// Пример:
//
//	{sum|postfix:` руб.`} → "100 руб."
func Postfix(s, p string) string {
	if strings.TrimSpace(s) == "" {
		return ""
	}
	return s + p
}

// UniqPostfix — добавить постфикс только если строка непустая и ещё не оканчивается на него.
//
// Пример:
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

// DefaultValue — вернуть значение по умолчанию, если строка пустая.
//
// Пример:
//
//	{position|default:`сотрудник`} → "сотрудник"
func DefaultValue(s, def string) string {
	if strings.TrimSpace(s) == "" {
		return def
	}
	return s
}

// Filled — вернуть указанное значение, если строка непустая; иначе пусто.
//
// Пример:
//
//	{passport|filled:`—`} → "—"
func Filled(val any, out string) string {
	// nil → пусто
	if val == nil {
		return ""
	}
	// для строки проверим пустую
	if s, ok := val.(string); ok {
		if s == "" {
			return ""
		}
		return out
	}
	// для всего остального — просто считаем, что "есть"
	return out
}

// Replace — заменить все вхождения подстроки.
//
// Пример:
//
//	{address|replace:`Москва`:`Санкт-Петербург`}
func Replace(s, old, new string) string {
	return strings.ReplaceAll(s, old, new)
}

// Truncate — обрезать строку до n символов, при необходимости добавить хвост (например, "…").
//
// Пример:
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

// WordReverse — меняет порядок слов в строке на обратный.
//
// Примеры:
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

// ConcatFactory возвращает функцию concat, которая склеивает строку с тегами или текстом.
// Последний параметр всегда считается разделителем.
//
// Примеры:
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

			if v, ok := data[val]; ok { // если совпадает с тегом
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
