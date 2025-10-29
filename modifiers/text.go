package modifiers

import (
	"docxgen/metrics"
	"docxgen/tostring"
	"regexp"
	"strconv"
	"strings"
)

// Неразрывные пробелы (обычный и узкий)
const (
	NBSP  = "\u00A0" // обычный неразрывный пробел
	NNBSP = "\u202F" // узкий неразрывный пробел
)

// Nowrap - заменяет все обычные пробелы на неразрывные.
// Используется для коротких кодов, индексов, номеров.
//
// Пример:
//
//	{case_index|nowrap} → "Дело № 15"
func Nowrap(s string) string {
	return strings.ReplaceAll(s, " ", NBSP)
}

// Compact - заменяет все пробелы на узкие неразрывные.
// Подходит для телефонов, номеров документов и таблиц.
//
// Пример:
//
//	{user_phone|compact} → "+7 (4912) 572-466"
func Compact(s string) string {
	return strings.ReplaceAll(s, " ", NNBSP)
}

// Abbr - делает сокращения, инициалы и короткие слова неразрывными с последующим словом.
// Это предотвращает разрыв “г.”, “ул.”, “ООО”, “И.” и т. п. на концах строк.
//
// Примеры:
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

// RuPhone - форматирует российские номера телефонов по шаблонам.
// Если номер не распознан, возвращает исходную строку.
//
// Примеры:
//
//	{user_phone|phone} → "+7 (4912) 572-466"
//	{user_phone|phone:`тел.: +7 ($2) $3-$4-$5`:`тел.: +7 ($2) $3-$4`}
//
// Параметры:
//   - без параметров — используется стандартный формат;
//   - 1 параметр — шаблон для мобильного телефона;
//   - 2 параметра — шаблон для мобильного и регионального телефона.
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
		// --- региональные ---
		regexp.MustCompile(`[+]?([78])[-\s]?\(?([1-7]\d{3})\)?[-\s]?(\d{3})[-\s]?(\d{3})`),
		regexp.MustCompile(`[+]?([78])[-\s]?([1-7]\d{3})[-\s]?(\d{3})[-\s]?(\d{3})`),

		// --- мобильные ---
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

			// если после совпадения идёт цифра — пропускаем
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
