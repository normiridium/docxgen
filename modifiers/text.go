package modifiers

import (
	"docxgen/metrics"
	"docxgen/tostring"
	"regexp"
	"strconv"
	"strings"
)

// Non-breaking spaces (normal and narrow)
const (
	NBSP  = "\u00A0" // Normal Unbroken Space
	NNBSP = "\u202F" // Narrow continuous space
)

// Nowrap - replaces all regular spaces with non-breaking spaces.
// Used for short codes, zip codes, numbers.
//
// Example:
//
//	{case_index|nowrap} → "Дело № 15"
func Nowrap(s string) string {
	return strings.ReplaceAll(s, " ", NBSP)
}

// Compact - replaces all spaces with narrow non-breaking spaces.
// Suitable for phones, document numbers, and spreadsheets.
//
// Example:
//
//	{user_phone|compact} → "+7 (4912) 572-466"
func Compact(s string) string {
	return strings.ReplaceAll(s, " ", NNBSP)
}

// Abbr - makes abbreviations, initials, and short words inseparable from the subsequent word.
// This prevents the breakage of "g.", "st.", "LLC", "I.", etc. at the ends of the lines.
//
// Examples:
//
//	"г. Москва" → "г. Москва"
//	"И. И. Иванов" → "И. И. Иванов"
//	"ООО Центр" → "ООО Центр"
func Abbr(s string) string {
	// поддержка и кириллицы, и латиницы
	re := regexp.MustCompile(`(?i)((?:(?:^|\s)[a-zа-яё.-]{1,5}\.?){1,2})\s+`)
	return re.ReplaceAllStringFunc(s, func(m string) string {
		return strings.ReplaceAll(m, " ", NBSP)
	})
}

// RuPhone formats Russian phone numbers according to templates.
// If the number is not recognized, returns the original string.
//
// Examples:
//
//	{user_phone|phone} → "+7 (4912) 572-466"
//	{user_phone|phone:`тел.: +7 ($2) $3-$4-$5`:`тел.: +7 ($2) $3-$4`}
//
// Options:
//   - No parameters — the standard format is used;
//   - 1 parameter — template for a mobile phone;
//   - 2 parameters – template for mobile and regional phones.
func RuPhone(s string, formats ...string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}

	template := "+7 ($2) $3-$4-$5"
	regional := "+7 ($2) $3-$4"

	if len(formats) == 1 {
		template = formats[0]
	} else if len(formats) >= 2 {
		template = formats[0]
		regional = formats[1]
	}

	patterns := []*regexp.Regexp{
		// --- regional ---
		regexp.MustCompile(`[+]?([78])[-\s]?\(?([1-7]\d{3})\)?[-\s]?(\d{3})[-\s]?(\d{3})`),
		regexp.MustCompile(`[+]?([78])[-\s]?([1-7]\d{3})[-\s]?(\d{3})[-\s]?(\d{3})`),

		// --- mobile ---
		regexp.MustCompile(`[+]?([78])[-\s]?\(?(\d{3})\)?[-\s]?(\d{3})[-\s]?(\d{2})[-\s]?(\d{2})`),
		regexp.MustCompile(`[+]?([78])[-\s]?(\d{3})[-\s]?(\d{3})[-\s]?(\d{2})[-\s]?(\d{2})`),
	}

	replacements := []string{
		regional,
		regional,
		template,
		template,
	}

	var buf strings.Builder
	for i, p := range patterns {
		repl := replacements[i]

		matches := p.FindAllStringIndex(s, -1)
		lastEnd := 0
		buf.Reset()

		for _, idx := range matches {
			start, end := idx[0], idx[1]

			// If there is a number after the match, skip it
			if end < len(s) && s[end] >= '0' && s[end] <= '9' {
				continue
			}

			// добавить текст до совпадения
			buf.WriteString(s[lastEnd:start])

			match := s[start:end]
			replaced := p.ReplaceAllString(match, repl)
			buf.WriteString(replaced)

			lastEnd = end
		}

		buf.WriteString(s[lastEnd:])
		s = buf.String()
	}

	return s
}

// ------- p_split -------

// MakePSplit is a p_split modifier that implements line breaks like in office editors,
// But only in tables, where the text needs to be scattered evenly across the cells.
//
// In the template, the designer puts lines of underscores with "_" by hand.
// By their number, we find out how many characters will "fit" into the line.
// Next, a tag with a p_split modifier is created instead of these underscores.
// When you write a modifier p_split the number of underlines is specified in the parameters.
// The program itself will recalculate the number of underlines in pt based on the size of the text and style.
//
// Algorithm: the text is divided into words and laid out into lines,
// so that the words do not break and each line fits into a given number of characters.
//
// Parameters (separated by a colon):
//
// {tag|p_split:<indentCount>:<lineCount>:<nLine>[:<style>:<fontSize>]}
//   - indentCount — the number of underscores in the first line (if there is a paragraph indent);
//     if there is no indent indent, indentCount = lineCount;
//   - lineCount — the number of underscores in all subsequent lines;
//   - nLine — the number of the line to be taken;
//     if indicated with a plus (for example, +2), all lines starting with this one are taken;
//   - style — (optional) font style: regular, bold, italic, bolditalic;
//   - fontSize—(optional) font size (10, 12, 14, etc.).
//
// Examples:
//
//	{address|p_split:20:65:1}        → первая строка адреса (20 подчёркиваний, затем 65)
//	{address|p_split:20:65:2}        → вторая строка адреса
//	{address|p_split:20:65:+2}       → вторая и все последующие строки
//	{fio|p_split:15:60:1:`bold`:12}    → первая строка, жирный 12 pt
func MakePSplit(fonts *metrics.FontSet) func(text string, firstUnders, otherUnders, nLine any, extra ...any) string {
	return func(text string, firstUnders, otherUnders, nLine any, extra ...any) string {
		// Number parsing
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

		// nLine: can be "+N" (join from N to end)
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

		// defaults
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

		// split
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
