package docxgen

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
)

// ============================================================================
//  Раздел: Обработка параграфов и маркеров Word XML
// ============================================================================

// ReplaceTagWithParagraph — заменяет параграф, содержащий {tag}, на XML-фрагмент.
// Если тег был единственным содержимым — параграф просто вырезается.
// Если в параграфе был текст до или после тега, они превращаются в отдельные <w:p>.
func ReplaceTagWithParagraph(body, tag, content string) string {
	const (
		openTag  = "<w:p>"
		closeTag = "</w:p>"
	)

	var out strings.Builder
	pos := 0

	for {
		start := strings.Index(body[pos:], openTag)
		if start < 0 {
			out.WriteString(body[pos:])
			break
		}
		start += pos
		end := strings.Index(body[start:], closeTag)
		if end < 0 {
			out.WriteString(body[pos:])
			break
		}
		end += start + len(closeTag)

		paragraph := body[start:end]
		text := extractParagraphText(paragraph)

		// если тег не найден — просто копируем параграф
		if !strings.Contains(text, tag) {
			out.WriteString(body[pos:end])
			pos = end
			continue
		}

		// если в параграфе только тег — вставляем контент без обертки
		// (никаких <w:p> вокруг)
		if strings.TrimSpace(text) == tag {
			out.WriteString(body[pos:start])
			out.WriteString(content)
			pos = end
			continue
		}

		// иначе делим текст на before/after и строим параграфы
		before, after, _ := strings.Cut(text, tag)
		out.WriteString(body[pos:start])

		if strings.TrimSpace(before) != "" {
			out.WriteString(`<w:p><w:r><w:t xml:space="preserve">`)
			out.WriteString(xmlEscape(strings.TrimSpace(before)))
			out.WriteString(`</w:t></w:r></w:p>`)
		}

		out.WriteString(content)

		if strings.TrimSpace(after) != "" {
			out.WriteString(`<w:p><w:r><w:t xml:space="preserve">`)
			out.WriteString(xmlEscape(strings.TrimSpace(after)))
			out.WriteString(`</w:t></w:r></w:p>`)
		}

		pos = end
	}

	return out.String()
}

// extractParagraphText — быстро вытаскивает текст из <w:t> без полных XML-разборов.
func extractParagraphText(p string) string {
	var b strings.Builder
	for {
		start := strings.Index(p, "<w:t>")
		if start < 0 {
			break
		}
		start += len("<w:t>")
		end := strings.Index(p[start:], "</w:t>")
		if end < 0 {
			break
		}
		b.WriteString(p[start : start+end])
		p = p[start+end+len("</w:t>"):]
	}
	return b.String()
}

// xmlEscape — экранирует &, <, > и кавычки для вставки в XML.
func xmlEscape(s string) string {
	return strings.NewReplacer(
		"&", "&amp;",
		"<", "&lt;",
		">", "&gt;",
		"\"", "&quot;",
		"'", "&apos;",
	).Replace(s)
}

// ============================================================================
//  Раздел: Unwrap и восстановление разорванных тегов
// ============================================================================

// ProcessUnWrapParagraphTags — ищет {*tag*} и превращает в блочные {tag}.
func (d *Docx) ProcessUnWrapParagraphTags(body string) string {
	for {
		start := strings.Index(body, "{*")
		if start == -1 {
			return body
		}
		endRel := strings.Index(body[start:], "*}")
		if endRel == -1 {
			return body
		}

		starTag := body[start : start+endRel+2] // "{*tag*}"
		name := strings.TrimSpace(body[start+2 : start+endRel])
		body = ReplaceTagWithParagraph(body, starTag, "{"+name+"}")
	}
}

// RepairTags — восстанавливает {tag} и [include] после того, как Word порвал их на <w:t>.
func (d *Docx) RepairTags(body string) (string, error) {
	var b strings.Builder
	inCurly, inSquare := false, false
	i := 0

	for i < len(body) {
		switch {
		case !inCurly && !inSquare:
			if body[i] == '{' {
				inCurly = true
			} else if body[i] == '[' {
				inSquare = true
			}
			b.WriteByte(body[i])
			i++
		default:
			switch {
			case strings.HasPrefix(body[i:], "<w:t"),
				strings.HasPrefix(body[i:], "</w:t>"),
				strings.HasPrefix(body[i:], "<w:r"),
				strings.HasPrefix(body[i:], "</w:r>"),
				strings.HasPrefix(body[i:], "<w:rPr"),
				strings.HasPrefix(body[i:], "</w:rPr>"),
				strings.HasPrefix(body[i:], "<w:"):
				if j := strings.IndexByte(body[i:], '>'); j >= 0 {
					i += j + 1
					continue
				}
			}
			if (inCurly && body[i] == '}') || (inSquare && body[i] == ']') {
				if inCurly {
					inCurly = false
				} else {
					inSquare = false
				}
				b.WriteByte(body[i])
				i++
				continue
			}
			b.WriteByte(body[i])
			i++
		}
	}
	return b.String(), nil
}

// ============================================================================
//  Раздел: [include/...]
//
//  Поддержка вставок дочерних DOCX-фрагментов:
//  [include/file.docx], [include/file.docx/table/2], [include/file.docx/p/3]
// ============================================================================

func (d *Docx) ResolveIncludes(body string) string {
	for {
		start := strings.Index(body, "[include/")
		if start < 0 {
			break
		}
		end := strings.Index(body[start:], "]")
		if end < 0 {
			break
		}
		end += start + 1

		raw := body[start:end]
		spec, err := ParseBracketIncludeTag(raw)
		if err != nil {
			body = body[:start] + body[end:]
			continue
		}
		xmlFrag, _, err := d.getIncludeXML(spec)
		if err != nil {
			body = body[:start] + body[end:]
			continue
		}
		body = ReplaceTagWithParagraph(body, spec.RawTag, xmlFrag)
	}
	return body
}

// --- helpers include ---

func (d *Docx) getIncludeXML(spec BracketIncludeSpec) (xmlFragment string, isTable bool, err error) {
	child, err := d.openFragmentDoc(spec.File)
	if err != nil {
		return "", false, fmt.Errorf("include open %q: %w", spec.File, err)
	}
	doc, err := child.Content()
	if err != nil {
		return "", false, fmt.Errorf("include %q: document.xml not found", spec.File)
	}

	switch spec.Fragment {
	case "body":
		xmlFragment, err = GetBodyFragment(doc)
		return xmlFragment, false, err
	case "table":
		xmlFragment, err = GetTableN(doc, spec.Index)
		return xmlFragment, true, err
	case "p":
		xmlFragment, err = GetParagraphN(doc, spec.Index)
		return xmlFragment, false, err
	default:
		return "", false, fmt.Errorf("unknown fragment")
	}
}

func (d *Docx) openFragmentDoc(rel string) (*Docx, error) {
	ext := strings.ToLower(filepath.Ext(rel))
	if ext != ".docx" && ext != ".dotx" {
		return nil, fmt.Errorf("unsupported include extension: %s", rel)
	}
	base := filepath.Dir(d.sourcePath)
	full, err := securejoin.SecureJoin(base, rel)
	if err != nil {
		return nil, fmt.Errorf("forbidden include path: %w", err)
	}
	if _, err := os.Stat(full); err != nil {
		return nil, err
	}
	return Open(full)
}

// --- извлечение фрагментов ---
func GetBodyFragment(content string) (string, error) {
	a := strings.Split(content, BodyOpeningTag)
	if len(a) < 2 {
		return "", fmt.Errorf("include: body open not found")
	}
	b := strings.Split(a[1], BodyClosingTag)
	if len(b) < 2 {
		return "", fmt.Errorf("include: body close not found")
	}
	return b[0], nil
}

// GetTableN — получить n-ую таблицу (нумерация с 1)
func GetTableN(content string, n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("include: bad table index")
	}

	parts := strings.Split(content, TablePartTag)
	idx := n*2 - 1
	if idx < 0 || idx >= len(parts) {
		return "", fmt.Errorf("include: table %d not found", n)
	}

	frag := TableOpeningTag + parts[idx] + TablePartTag

	// ⚠️ обрезаем всё после конца таблицы (чтобы не захватывать sectPr)
	if pos := strings.Index(frag, "</w:tbl>"); pos != -1 {
		frag = frag[:pos+len("</w:tbl>")]
	}
	return frag, nil
}

func GetParagraphN(content string, n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("include: bad paragraph index")
	}
	parts := strings.Split(content, ParagraphPartTag)
	idx := n*2 - 1
	if idx < 0 || idx >= len(parts) {
		return "", fmt.Errorf("include: paragraph %d not found", n)
	}
	return ParagraphOpeningTag + parts[idx] + ParagraphPartTag, nil
}

// --- структура описания include ---
type BracketIncludeSpec struct {
	RawTag   string
	File     string
	Fragment string
	Index    int
}

// ParseBracketIncludeTag — парсит строку вида "[include/file.docx/table/2]" без регулярок.
func ParseBracketIncludeTag(tag string) (BracketIncludeSpec, error) {
	spec := BracketIncludeSpec{
		RawTag:   tag,
		Fragment: "body",
		Index:    1,
	}

	tag = strings.TrimSpace(tag)
	if !strings.HasPrefix(tag, "[include/") || !strings.HasSuffix(tag, "]") {
		return spec, fmt.Errorf("not an include marker")
	}

	inner := strings.TrimSuffix(strings.TrimPrefix(tag, "["), "]") // include/...
	parts := strings.Split(inner, "/")
	if len(parts) < 2 || parts[0] != "include" {
		return spec, fmt.Errorf("invalid include marker")
	}

	// ищем сегмент с .docx
	idxDocx := -1
	for i := 1; i < len(parts); i++ {
		if strings.HasSuffix(strings.ToLower(parts[i]), ".docx") {
			idxDocx = i
			break
		}
	}
	if idxDocx == -1 {
		return spec, fmt.Errorf("include: .docx not found")
	}

	// собираем путь к файлу
	fileSegments := parts[1 : idxDocx+1]
	filePath := path.Clean(path.Join(fileSegments...))
	if filePath == "." || filePath == "" {
		return spec, fmt.Errorf("include: empty file path")
	}
	spec.File = filePath

	// разбираем остаток (фрагмент)
	rest := parts[idxDocx+1:]
	if len(rest) == 0 {
		return spec, nil
	}

	switch strings.ToLower(strings.TrimSpace(rest[0])) {
	case "body":
		spec.Fragment = "body"
	case "table":
		spec.Fragment = "table"
		if len(rest) >= 2 {
			n, err := strconv.Atoi(strings.TrimSpace(rest[1]))
			if err != nil || n <= 0 {
				return spec, fmt.Errorf("include: bad table index")
			}
			spec.Index = n
		}
	case "p", "paragraph":
		spec.Fragment = "p"
		if len(rest) >= 2 {
			n, err := strconv.Atoi(strings.TrimSpace(rest[1]))
			if err != nil || n <= 0 {
				return spec, fmt.Errorf("include: bad paragraph index")
			}
			spec.Index = n
		}
	default:
		return spec, fmt.Errorf("include: unknown fragment %q", rest[0])
	}

	return spec, nil
}

// ============================================================================
// ProcessTrimTags — чистит {~} и {-} с учётом соседних <w:t> и корректных пробелов
// ============================================================================

// ProcessTrimTags — убирает пробелы, табы и переносы вокруг {~}/{-}, не ломая структуру Word.
func (d *Docx) ProcessTrimTags(body string) string {
	// 1. Подмена спец-тегов на символы
	body = strings.ReplaceAll(body, "<w:tab/>", "<w:t>\t</w:t>")
	body = strings.ReplaceAll(body, "<w:br/>", "<w:t>\n</w:t>")

	// 2. Разбиваем на параграфы
	parts := strings.Split(body, "<w:p>")
	if len(parts) == 1 {
		return body // нет параграфов
	}

	for i := 1; i < len(parts); i++ {
		p := parts[i]
		end := strings.Index(p, "</w:p>")
		if end == -1 {
			continue
		}
		content := p[:end]

		// фильтр: чистим только если в параграфе есть {~ или {-
		if !strings.Contains(content, "{~") && !strings.Contains(content, "{-") &&
			!strings.Contains(content, "~}") && !strings.Contains(content, "-}") {
			continue
		}

		// 3. Работаем внутри параграфа как раньше — построчно по <w:r>
		reRun := regexp.MustCompile(`(?s)<w:r>.*?</w:r>`)
		content = reRun.ReplaceAllStringFunc(content, func(run string) string {
			reT := regexp.MustCompile(`(?s)<w:t[^>]*>.*?</w:t>`)
			partsT := reT.FindAllString(run, -1)
			if len(partsT) == 0 {
				return run
			}

			var buf strings.Builder
			for _, p := range partsT {
				buf.WriteString(extractText(p))
			}
			clean := cleanTrimTags(buf.String())

			// 4. Восстанавливаем спец-теги, не ломая структуру XML
			var out strings.Builder
			out.WriteString("<w:r>")
			out.WriteString("<w:t>")

			open := true
			for i, r := range clean {
				switch r {
				case '\t':
					if open {
						out.WriteString("</w:t>")
						open = false
					}
					out.WriteString("<w:tab/>")
					if i < len(clean)-1 {
						out.WriteString("<w:t>")
						open = true
					}
				case '\n':
					if open {
						out.WriteString("</w:t>")
						open = false
					}
					out.WriteString("<w:br/>")
					if i < len(clean)-1 {
						out.WriteString("<w:t>")
						open = true
					}
				default:
					if !open {
						out.WriteString("<w:t>")
						open = true
					}
					out.WriteRune(r)
				}
			}
			if open {
				out.WriteString("</w:t>")
			}
			out.WriteString("</w:r>")
			return out.String()
		})

		parts[i] = content + p[end:]
	}

	return strings.Join(parts, "<w:p>")
}

// cleanTrimTags — удаляет пробелы, табы и переносы вокруг {~}/{-}, корректируя пробелы.
func cleanTrimTags(s string) string {
	// {~...~} — ест всё
	s = regexp.MustCompile(`[\s]*\{~`).ReplaceAllString(s, "{")
	s = regexp.MustCompile(`~}[\s]*`).ReplaceAllString(s, "}")
	// {-...-} — ест только пробелы и табы
	s = regexp.MustCompile(`[ \t]*\{-`).ReplaceAllString(s, "{")
	s = regexp.MustCompile(`-}[ \t]*`).ReplaceAllString(s, "}")
	// убираем маркеры
	s = strings.ReplaceAll(s, "{~", "{")
	s = strings.ReplaceAll(s, "~}", "}")
	s = strings.ReplaceAll(s, "{-", "{")
	s = strings.ReplaceAll(s, "-}", "}")
	// восстанавливаем пробел перед тегом
	s = regexp.MustCompile(`([A-Za-zА-Яа-яЁё])\{`).ReplaceAllString(s, `$1 {`)
	return s
}

// extractText — достаёт текст из <w:t ...>...</w:t>.
func extractText(xml string) string {
	start := strings.Index(xml, ">")
	if start == -1 {
		return ""
	}
	end := strings.Index(xml[start:], "</w:t>")
	if end == -1 {
		return ""
	}
	end += start
	return xml[start+1 : end]
}
