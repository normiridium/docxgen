package modifiers

import (
	"fmt"
	"strings"

	"docxgen/petrovich"
)

// Declension — склоняет ФИО в указанный падеж и формат, используя petrovich-go.
// Если приходит строка "Фамилия Имя Отчество" — делает автоматическое склонение.
// Если приходит map[string]string с готовыми формами — выбирает нужную.
//
// Примеры:
//
//	{user_fio|declension:`предложный`:`ф и о`} = "Иванову Ивану Ивановичу"
//	{user_fio|declension:`дательный`:`фамилия и.о.`} = "Сидорову И.П."
//	{user_fio|declension:`винительный`:`фамилия`} = "Петрова"
//	{user_fio|decl:`п`:`и.о. ф`} = "И.И. Сидорову"
func Declension(v any, opts ...string) string {
	src := strings.TrimSpace(fmt.Sprint(v))
	if src == "" {
		return ""
	}

	// параметры: падеж и формат
	caseName := "родительный"
	format := ""
	if len(opts) >= 1 && strings.TrimSpace(opts[0]) != "" {
		caseName = strings.ToLower(strings.TrimSpace(opts[0]))
	}
	if len(opts) >= 2 && strings.TrimSpace(opts[1]) != "" {
		format = strings.ToLower(strings.TrimSpace(opts[1]))
	}

	// если дали готовые формы
	if m, ok := v.(map[string]string); ok {
		first, last, middle := pickPrepared(m, caseName)
		return formatFIO(first, last, middle, format)
	}

	// иначе используем petrovich
	p, _ := petrovich.LoadRules()
	parts := strings.Fields(src)
	if len(parts) == 0 {
		return src
	}

	// определяем род по отчеству
	gender := petrovich.Androgynous
	if len(parts) == 3 {
		if strings.HasSuffix(parts[2], "ич") {
			gender = petrovich.Male
		}
		if strings.HasSuffix(parts[2], "на") {
			gender = petrovich.Female
		}
	} else if len(parts) >= 1 {
		// если отчества нет — пробуем по фамилии
		last := strings.ToLower(parts[0])
		switch {
		case strings.HasSuffix(last, "ов"),
			strings.HasSuffix(last, "ев"),
			strings.HasSuffix(last, "ин"),
			strings.HasSuffix(last, "ский"),
			strings.HasSuffix(last, "цкий"):
			gender = petrovich.Male

		case strings.HasSuffix(last, "ова"),
			strings.HasSuffix(last, "ева"),
			strings.HasSuffix(last, "ина"),
			strings.HasSuffix(last, "ая"),
			strings.HasSuffix(last, "ская"):
			gender = petrovich.Female
		}
	}

	// склоняем каждую часть
	last, first, middle := "", "", ""
	switch len(parts) {
	case 1:
		last = p.InfLastname(parts[0], petrovichCase(caseName), gender)
	case 2:
		last = p.InfLastname(parts[0], petrovichCase(caseName), gender)
		first = p.InfFirstname(parts[1], petrovichCase(caseName), gender)
	case 3:
		last = p.InfLastname(parts[0], petrovichCase(caseName), gender)
		first = p.InfFirstname(parts[1], petrovichCase(caseName), gender)
		middle = p.InfMiddlename(parts[2], petrovichCase(caseName), gender)
	}

	return formatFIO(first, last, middle, format)
}

// petrovichCase — маппинг наших падежных сокращений на библиотечные константы.
func petrovichCase(c string) petrovich.Case {
	switch strings.ToLower(strings.TrimSpace(c)) {
	case "род", "родительный", "gen", "р":
		return petrovich.Genitive
	case "дат", "дательный", "dat", "д":
		return petrovich.Dative
	case "вин", "винительный", "acc", "в":
		return petrovich.Accusative
	case "тв", "творительный", "ins", "т":
		return petrovich.Instrumental
	case "пред", "предложный", "prep", "п":
		return petrovich.Prepositional
	default:
		return petrovich.Genitive
	}
}

// форматирование по токенам
func formatFIO(first, last, middle, format string) string {
	trim := func(s string) string { return strings.TrimSpace(s) }

	// если формат пустой — по умолчанию "ф и о"
	if strings.TrimSpace(format) == "" {
		out := strings.TrimSpace(strings.Join([]string{trim(last), trim(first), trim(middle)}, " "))
		out = strings.Join(strings.Fields(out), " ")
		return out
	}

	initial := func(s string) string {
		if s == "" {
			return ""
		}
		r := []rune(strings.TrimSpace(s))
		return string(r[0]) + "."
	}

	tokens := strings.Fields(format)
	res := make([]string, 0, len(tokens))

	for _, t := range tokens {
		switch t {
		case "ф", "фамилия":
			res = append(res, trim(last))
		case "и", "имя":
			res = append(res, trim(first))
		case "о", "отчество":
			res = append(res, trim(middle))
		case "и.":
			res = append(res, initial(first))
		case "о.":
			res = append(res, initial(middle))
		case "и.о.":
			res = append(res, initial(first)+initial(middle))
		default:
			// любой произвольный токен — как есть
			res = append(res, t)
		}
	}

	out := strings.TrimSpace(strings.Join(res, " "))
	out = strings.Join(strings.Fields(out), " ")
	return out
}

func pickPrepared(m map[string]string, caseName string) (first, last, middle string) {
	c := normalizeCase(caseName)
	// ключи вида first_gen, last_dat, middle_ins и т.п.
	first = coalesce(m["first_"+c], m["first_nom"], m["first"])
	last = coalesce(m["last_"+c], m["last_nom"], m["last"], m["surname_"+c], m["surname"])
	middle = coalesce(m["middle_"+c], m["middle_nom"], m["middle"], m["patronymic_"+c], m["patronymic"])
	return
}

func coalesce(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func normalizeCase(c string) string {
	c = strings.ToLower(strings.TrimSpace(c))
	switch c {
	case "им", "именительный", "nom", "nominative":
		return "nom"
	case "род", "родительный", "gen", "genitive", "р":
		return "gen"
	case "дат", "дательный", "dat", "dative", "д":
		return "dat"
	case "вин", "винительный", "acc", "accusative", "в":
		return "acc"
	case "тв", "творительный", "ins", "instrumental", "т":
		return "ins"
	case "пред", "предложный", "prep", "prepositional", "п":
		return "prep"
	default:
		return "gen" // умолчание — родительный
	}
}
