package modifiers

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/normiridium/rusnum"
)

// -------- Numeral --------

// Numeral is a numerical modifier with morphology (gender, case, variant 8, variant zero).
//
// Examples:
//
//	{count|numeral} → "один"
//	{count|numeral:`предложный`} → "одном"
//	{count|numeral:`женский`:`предложный`} → "одной"
//	{count|numeral:`предложный`:`ноль`} → "ноле"
//	{count|numeral:`женский`:`творительный`:`восемью`:`нуль`} → "восемью"
//	{35147|numeral:`дательный`} → "тридцати пяти тысячам ста сорока семи"
func Numeral(v any, opts ...string) string {
	n, ok := parseInt(v)
	if !ok {
		return ""
	}

	// defaults
	g := rusnum.Masc
	c := rusnum.Nom
	nullStyle := rusnum.ZeroNul
	alt8 := false

	// Parameter parsing
	for _, p := range opts {
		p = strings.ToLower(strings.TrimSpace(p))
		if p == "" {
			continue
		}

		// genus
		switch p {
		case "м", "муж", "мужской", "masc", "m", "masculine":
			g = rusnum.Masc
			continue
		case "ж", "жен", "женский", "fem", "f", "feminine":
			g = rusnum.Fem
			continue
		case "ср", "сред", "средний", "neut", "n", "neutral":
			g = rusnum.Neut
			continue
		}

		// maturity
		switch p {
		case "им", "именительный", "nom", "nominative":
			c = rusnum.Nom
			continue
		case "род", "родительный", "gen", "genitive":
			c = rusnum.Gen
			continue
		case "дат", "дательный", "dat", "dative":
			c = rusnum.Dat
			continue
		case "вин", "винительный", "acc", "accusative":
			c = rusnum.Acc
			continue
		case "тв", "творительный", "ins", "instrumental":
			c = rusnum.Ins
			continue
		case "пред", "предложный", "prep", "prepositional":
			c = rusnum.Prep
			continue
		}

		// Figure-eight shapes
		switch p {
		case "восемью", "альт8", "альтернативная8", "alt8":
			alt8 = true
			continue
		case "восьмью", "стандартная8", "std8":
			alt8 = false
			continue
		}

		// Zero Style
		switch p {
		case "нуль", "nul", "zero-nul":
			nullStyle = rusnum.ZeroNul
			continue
		case "ноль", "nol", "zero-nol":
			nullStyle = rusnum.ZeroNol
			continue
		}
	}

	return rusnum.ToWords(
		n,
		rusnum.WithGender(g),
		rusnum.WithCase(c),
		rusnum.WithNullStyle(nullStyle),
		rusnum.WithInsEightAlt(alt8),
	)
}

// -------- Plural --------

// Plural is the declension of nouns by number.
// v is a value (a number or a string with a number),
// forms — three forms of the word: ["employee", "employee", "employees"].
//
// Examples:
//
//	{count|plural:`день`:`дня`:`дней`}        → "дня"
//	{files|plural:`файл`:`файла`:`файлов`}   → "файлов"
//	{users|plural}                           → "сотрудников"
func Plural(v any, forms ...string) string {
	n, ok := parseInt(v)
	if !ok {
		return ""
	}

	// формы по умолчанию
	if len(forms) == 0 {
		forms = []string{"сотрудник", "сотрудника", "сотрудников"}
	}

	// если указано только две формы — расширяем до трёх
	if len(forms) == 2 {
		forms = []string{forms[0], forms[1], forms[1]}
	}

	var idx int
	if n%10 == 1 && n%100 != 11 {
		idx = 0 // один
	} else if n%10 >= 2 && n%10 <= 4 && (n%100 < 10 || n%100 >= 20) {
		idx = 1 // два–четыре
	} else {
		idx = 2 // остальные
	}

	return forms[idx]
}

// -------- Sign --------

// Sign - Adds a "+" sign to positive numbers.
//
// Examples:
//
//	{delta|sign} → "+5"
//	{-3|sign} → "-3"
//	{0|sign} → "0"
func Sign(v any) string {
	f, ok := parseFloat(v)
	if !ok {
		return fmt.Sprint(v)
	}
	if f > 0 {
		return fmt.Sprintf("+%v", f)
	}
	return fmt.Sprintf("%v", f)
}

// -------- PadLeft --------

// PadLeft - Completes the string on the left with a character up to a specified length.
//
// Example:
//
//	{num|pad_left:`5`:`0`} → "00042"
func PadLeft(v any, length int, char string) string {
	s := fmt.Sprint(v)
	for len(s) < length {
		s = char + s
	}
	return s
}

// -------- PadRight --------

// PadRight - Completes the string on the right with a character up to a specified length.
//
// Example:
//
//	{num|pad_right:`3`:`0`} → "420"
func PadRight(v any, length int, char string) string {
	s := fmt.Sprint(v)
	for len(s) < length {
		s += char
	}
	return s
}

// -------- Money --------

// Money - Formats a number as a monetary value separated by thousands of spaces.
// Supports the "int" / "целое" flag to hide the fractional part,
// as well as custom format via FMT template. Sprintf.
//
// Examples:
//
//	{sum|money}                    → "1 234,56"
//	{sum|money:`int`}              → "1 234"
//	{sum|money:`целое`}            → "1 234"
//	{sum|money:`%s рублей`}        → "1 234 рублей"
//	{sum|money:`%s рублей %02d копеек`} → "1 234 рублей 56 копеек"
//
// Если шаблон содержит только один плейсхолдер (%s), дробная часть будет опущена.
// Некорректный формат не вызывает паники — просто игнорируется.
func Money(v any, opts ...string) string {
	f, ok := parseFloat(v)
	if !ok {
		return fmt.Sprint(v)
	}

	intPart := int64(f)
	fracPart := int64(math.Round((f - float64(intPart)) * 100))
	intStr := fmt.Sprintf("%d", intPart)

	var parts []string
	for len(intStr) > 3 {
		parts = append([]string{intStr[len(intStr)-3:]}, parts...)
		intStr = intStr[:len(intStr)-3]
	}
	parts = append([]string{intStr}, parts...)
	main := strings.Join(parts, " ")

	if len(opts) > 0 {
		format := strings.TrimSpace(opts[0])
		lower := strings.ToLower(format)

		switch lower {
		case "int", "целое":
			return main
		default:
			if strings.Contains(format, "%") {
				defer func() { _ = recover() }() // безопасно подавляем форматные ошибки

				if strings.Count(format, "%") == 1 {
					return fmt.Sprintf(format, main)
				}
				return fmt.Sprintf(format, main, fracPart)
			}
		}
	}

	return fmt.Sprintf("%s,%02d", main, fracPart)
}

// -------- Roman --------

// Roman - Converts the number into Roman numerals.
//
// Example:
//
//	{page|roman} → "XIV"
func Roman(v any) string {
	n, ok := parseInt(v)
	if !ok || n <= 0 {
		return ""
	}
	vals := []int{1000, 900, 500, 400, 100, 90, 50, 40, 10, 9, 5, 4, 1}
	syms := []string{"M", "CM", "D", "CD", "C", "XC", "L", "XL", "X", "IX", "V", "IV", "I"}

	var b strings.Builder
	for i := 0; i < len(vals); i++ {
		for n >= vals[i] {
			n -= vals[i]
			b.WriteString(syms[i])
		}
	}
	return b.String()
}

// -------------------- helpers --------------------

func parseInt(v any) (int, bool) {
	switch x := v.(type) {
	case int:
		return x, true
	case int64:
		return int(x), true
	case float64:
		return int(x), true
	case string:
		n, err := strconv.Atoi(strings.TrimSpace(x))
		return n, err == nil
	default:
		s := fmt.Sprint(x)
		n, err := strconv.Atoi(strings.TrimSpace(s))
		return n, err == nil
	}
}

func parseFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case int:
		return float64(x), true
	case string:
		f, err := strconv.ParseFloat(strings.TrimSpace(strings.ReplaceAll(x, ",", ".")), 64)
		return f, err == nil
	default:
		s := fmt.Sprint(x)
		f, err := strconv.ParseFloat(strings.TrimSpace(strings.ReplaceAll(s, ",", ".")), 64)
		return f, err == nil
	}
}
