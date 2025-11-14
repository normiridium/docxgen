package docxgen

import (
	"strconv"
	"strings"
)

// transformTag gets a string like {fio|declension:`genitive`:`ф: и }о`}
// and converts it to {.fio | declension "genitive" "ф: и }о"}
func transformTag(tag string) string {
	tag = strings.TrimSuffix(strings.TrimPrefix(tag, "{"), "}")

	var parts []string
	var buf strings.Builder
	inQuote := false

	for _, r := range tag {
		switch r {
		case '`':
			if inQuote {
				// closed literal
				parts = append(parts, `"`+buf.String()+`"`)
				buf.Reset()
				inQuote = false
			} else {
				// opened a letter
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
			// If there is already a line (we marked it this way above) → insert it as it is.
			if strings.HasPrefix(arg, `"`) && strings.HasSuffix(arg, `"`) {
				out.WriteString(" ")
				out.WriteString(arg)
				continue
			}
			// if the number leave as it is
			if _, err := strconv.ParseFloat(arg, 64); err == nil {
				out.WriteString(" ")
				out.WriteString(arg)
				continue
			}
			// Everything else → line
			out.WriteString(" ")
			out.WriteString(`"`)
			out.WriteString(arg)
			out.WriteString(`"`)
		}
	}
	out.WriteString("}")
	return out.String()
}

// TransformTemplate bypasses all the text of the document and converts the old {tag|mod:arg}
// into the valid syntax of Go templates. Ready-made Go tags ({.fio ...}, {if ...}, etc.)
// leaves unchanged.
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
				// toggle the quotation mark flag
				inQuote = !inQuote
			}
			token.WriteRune(r)
		case '}':
			if inTag && !inQuote {
				// finished tag
				token.WriteRune(r)
				tok := token.String()
				if looksLikeOldStyle(tok) {
					out.WriteString(transformTag(tok))
				} else {
					out.WriteString(tok)
				}
				inTag = false
			} else {
				// either plain text or inside quotation marks
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

// looksLikeOldStyle determines whether to run the tag through the transformTag
func looksLikeOldStyle(tag string) bool {
	t := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(tag, "{"), "}"))
	if strings.HasPrefix(t, ".") {
		return false
	}
	// Do NOT touch Go-expressions
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
	// simplified style: either a simple tag without a period,
	// or with a modifier in |
	return !strings.HasPrefix(t, ".")
}
