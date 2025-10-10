package modifiers

import (
	"fmt"
	"strconv"
	"strings"

	"docxgen/metrics"
	"docxgen/tostring"
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
	if len(s) <= n {
		return s
	}
	return s[:n] + suffix
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

			if val != "" {
				chunks = append(chunks, val)
			}
		}

		return strings.Join(chunks, sep)
	}
}

// ------- p_split -------

// MakePSplit — модификатор p_split, реализует переносы строк как в офисных редакторах,
// но только в таблицах, где текст нужно раскидать равномерно по ячейкам.
//
// В шаблоне дизайнер руками проставляет строки из подчёркиваний "_".
// По их количеству мы узнаем сколько символов «влезет» в строку.
// Далее заводится тег с модификатором p_split вместо этих подчеркиваний.
// При написании модификатора p_split количество подчеркиваний указывается в параметрах.
// Программа сама пересчитает количество подчеркиваний в pt исходя из размера текста, начертания.
//
// Алгоритм: текст делится на слова и раскладывается по строкам,
// чтобы слова не рвались и каждая строка укладывалась в заданное число символов.
//
// Параметры (через двоеточие):
//
//	{tag|p_split:<indentCount>:<lineCount>:<nLine>[:<style>:<fontSize>]}
//	• indentCount — количество подчёркиваний в первой строке (если есть абзацный отступ);
//	                если отступа нет, indentCount = lineCount;
//	• lineCount   — количество подчёркиваний во всех последующих строках;
//	• nLine       — номер строки, которую нужно взять;
//	                если указано с плюсом (например, +2), берутся все строки начиная с этой;
//	• style       — (необязательно) стиль шрифта: regular, bold, italic, bolditalic;
//	• fontSize    — (необязательно) размер шрифта (10, 12, 14 и т. д.).
//
// Примеры:
//
//	{address|p_split:20:65:1}        → первая строка адреса (20 подчёркиваний, затем 65)
//	{address|p_split:20:65:2}        → вторая строка адреса
//	{address|p_split:20:65:+2}       → вторая и все последующие строки
//	{fio|p_split:15:60:1:`bold`:12}    → первая строка, жирный 12 pt
func MakePSplit(fonts *metrics.FontSet) func(text string, firstUnders, otherUnders, nLine any, extra ...any) string {
	return func(text string, firstUnders, otherUnders, nLine any, extra ...any) string {
		// парсинг чисел
		parseInt := func(a any) (int, bool) {
			switch v := a.(type) {
			case int:
				return v, true
			case int64:
				return int(v), true
			case float64:
				return int(v), true
			case string:
				if v == "" {
					return 0, false
				}
				n, err := strconv.Atoi(v)
				return n, err == nil
			default:
				return 0, false
			}
		}
		fi, ok1 := parseInt(firstUnders)
		oi, ok2 := parseInt(otherUnders)
		if !ok1 || !ok2 || fi <= 0 || oi <= 0 {
			return ""
		}

		// nLine: может быть "+N" (join от N до конца)
		isMore := false
		nIdx := 1
		switch v := nLine.(type) {
		case string:
			s := strings.TrimSpace(v)
			if strings.HasPrefix(s, "+") {
				isMore = true
				s = strings.TrimPrefix(s, "+")
			}
			if s == "" {
				return ""
			}
			n, err := strconv.Atoi(s)
			if err != nil || n <= 0 {
				return ""
			}
			nIdx = n
		default:
			n, ok := parseInt(v)
			if !ok || n <= 0 {
				return ""
			}
			nIdx = n
		}

		// дефолты
		style := metrics.Regular
		sizePt := 12.0

		// extra: [sizePt?, style?]
		if len(extra) > 0 {
			switch vv := extra[0].(type) {
			case int:
				sizePt = float64(vv)
			case float64:
				sizePt = vv
			case string:
				if n, err := strconv.Atoi(vv); err == nil {
					sizePt = float64(n)
				}
			}
		}
		if len(extra) > 1 {
			style = parseStyle(extra[1])
		}

		// разбиваем
		lines, err := tostring.SplitParagraphByUnderscore(
			strings.TrimSpace(text),
			fonts,
			style,
			sizePt,
			fi,
			oi,
		)
		if err != nil || len(lines) == 0 {
			return ""
		}

		idx := nIdx - 1
		if idx < 0 || idx >= len(lines) {
			return ""
		}
		if isMore {
			return strings.Join(lines[idx:], " ")
		}
		return lines[idx]
	}
}

func parseStyle(v any) metrics.Style {
	asStr := ""
	switch t := v.(type) {
	case string:
		asStr = strings.ToLower(strings.TrimSpace(t))
	default:
		asStr = strings.ToLower(strings.TrimSpace(strings.Trim(fmtAny(v), "\"")))
	}

	switch asStr {
	case "regular", "r", "reg", "обычный", "о", "регулярный":
		return metrics.Regular
	case "bold", "b", "жирный", "ж":
		return metrics.Bold
	case "italic", "i", "курсив", "к":
		return metrics.Italic
	case "bolditalic", "bold-italic", "bi", "жирныйкурсив", "жк", "жирныйкурсивный":
		return metrics.BoldItalic
	default:
		return metrics.Regular
	}
}

func fmtAny(v any) string {
	switch x := v.(type) {
	case string:
		return x
	default:
		return ""
	}
}
