package docxgen

import (
	"strconv"
	"strings"
)

// transformTag получает строку вида {fio|declension:`genitive`:`ф: и }о`}
// и конвертирует её в {.fio | declension "genitive" "ф: и }о"}
func transformTag(tag string) string {
	tag = strings.TrimSuffix(strings.TrimPrefix(tag, "{"), "}")

	var parts []string
	var buf strings.Builder
	inQuote := false

	for _, r := range tag {
		switch r {
		case '`':
			if inQuote {
				// закрыли литерал
				parts = append(parts, `"`+buf.String()+`"`)
				buf.Reset()
				inQuote = false
			} else {
				// открыли литерал
				inQuote = true
				buf.Reset()
			}
		case '|', ':':
			if inQuote {
				buf.WriteRune(r)
			} else {
				if buf.Len() > 0 {
					parts = append(parts, buf.String())
					buf.Reset()
				}
			}
		default:
			buf.WriteRune(r)
		}
	}
	if buf.Len() > 0 {
		if inQuote {
			parts = append(parts, `"`+buf.String()+`"`)
		} else {
			parts = append(parts, buf.String())
		}
	}

	if len(parts) == 0 {
		return "{}"
	}

	out := new(strings.Builder)
	out.WriteString("{.")
	out.WriteString(strings.TrimSpace(parts[0]))

	if len(parts) > 1 {
		out.WriteString(" | ")
		out.WriteString(strings.TrimSpace(parts[1]))
		for _, arg := range parts[2:] {
			// если уже строка (мы её так пометили выше) → вставляем как есть
			if strings.HasPrefix(arg, `"`) && strings.HasSuffix(arg, `"`) {
				out.WriteString(" ")
				out.WriteString(arg)
				continue
			}
			// если число → оставляем как есть
			if _, err := strconv.ParseFloat(arg, 64); err == nil {
				out.WriteString(" ")
				out.WriteString(arg)
				continue
			}
			// всё остальное → строка
			out.WriteString(" ")
			out.WriteString(`"`)
			out.WriteString(arg)
			out.WriteString(`"`)
		}
	}
	out.WriteString("}")
	return out.String()
}

// TransformTemplate обходит весь текст документа и конвертит старые {tag|mod:arg}
// в валидный синтаксис Go-шаблонов. Уже готовые Go-теги ({.fio ...}, {if ...} и т.п.)
// оставляет без изменений.
func TransformTemplate(input string) string {
	var out strings.Builder
	var token strings.Builder
	inTag := false
	inQuote := false

	for _, r := range input {
		switch r {
		case '{':
			if inTag {
				token.WriteRune(r)
			} else {
				inTag = true
				token.Reset()
				token.WriteRune(r)
			}
		case '`':
			if inTag {
				// переключаем флаг кавычек
				inQuote = !inQuote
			}
			token.WriteRune(r)
		case '}':
			if inTag && !inQuote {
				// закончили тег
				token.WriteRune(r)
				tok := token.String()
				if looksLikeOldStyle(tok) {
					out.WriteString(transformTag(tok))
				} else {
					out.WriteString(tok)
				}
				inTag = false
			} else {
				// либо обычный текст, либо внутри кавычек
				if inTag {
					token.WriteRune(r)
				} else {
					out.WriteRune(r)
				}
			}
		default:
			if inTag {
				token.WriteRune(r)
			} else {
				out.WriteRune(r)
			}
		}
	}

	if inTag {
		out.WriteString(token.String())
	}

	return out.String()
}

// looksLikeOldStyle определяет, нужно ли прогонять тег через transformTag
func looksLikeOldStyle(tag string) bool {
	t := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(tag, "{"), "}"))
	if strings.HasPrefix(t, ".") {
		return false
	}
	// Уже Go-выражения — НЕ трогаем
	if strings.HasPrefix(t, ".") ||
		strings.HasPrefix(t, "`") ||
		strings.HasPrefix(t, "\"") {
		return false
	}
	if strings.HasPrefix(t, "if ") || strings.HasPrefix(t, "else") ||
		strings.HasPrefix(t, "end") || strings.HasPrefix(t, "range ") ||
		strings.HasPrefix(t, "with ") {
		return false
	}
	// старый стиль: либо простой тег без точки,
	// либо с модификатором через |
	return !strings.HasPrefix(t, ".")
}
