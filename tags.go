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
// Topic: Processing Word XML Paragraphs and Bullets
// ============================================================================

// ReplaceTagWithParagraph - Replaces the paragraph containing {tag} with an XML fragment.
// If the tag was the only content, the paragraph is simply cut out.
// If the paragraph had text before or after the tag, they turn into separate <w:p>.
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

		// тег НЕ найден: просто копируем как есть
		if !strings.Contains(text, tag) {
			out.WriteString(body[pos:end])
			pos = end
			continue
		}

		// тег найден - обрабатываем
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

// extractParagraphText — Quickly pulls text out of <w:t> without full XML parsing.
func extractParagraphText(p string) string {
	var b strings.Builder

	for {
		idx1 := strings.Index(p, "<w:t>")
		idx2 := strings.Index(p, "<w:t ")

		// Select the nearest one
		startTag := -1
		if idx1 >= 0 && idx2 >= 0 {
			if idx1 < idx2 {
				startTag = idx1
			} else {
				startTag = idx2
			}
		} else if idx1 >= 0 {
			startTag = idx1
		} else if idx2 >= 0 {
			startTag = idx2
		} else {
			break
		}

		// Looking for closing the '>' tag
		tagClose := strings.Index(p[startTag:], ">")
		if tagClose < 0 {
			break
		}
		tagClose += startTag

		// Looking for </w:t>
		endTag := strings.Index(p[tagClose:], "</w:t>")
		if endTag < 0 {
			break
		}
		endTag += tagClose

		// text between tags
		text := p[tagClose+1 : endTag]
		b.WriteString(text)

		// moving on
		p = p[endTag+len("</w:t>"):]
	}

	return b.String()
}

// xmlEscape — escapes &, <, >, and quotes to insert into XML.
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
// Section: Unwrap and Repair Broken Tags
// ============================================================================

// ProcessUnWrapParagraphTags - Looks for {*tag*} and turns it into block {tags}.
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

// RepairTags — restores {tag} and [include] after Word tore them at <w:t>.
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

//============================================================================
// Раздел: [included/..]
//
// Поддержка вставок дочерних docs-фрагментов:
// [include/file.docs], [include/file.docs/table/2], [include/file.docs/p/3]
//============================================================================

func (d *Docx) ResolveIncludes(body string, data map[string]any) string {
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
		spec, err := ParseBracketIncludeTag(raw, data)
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
	doc, err := child.ContentPart("document")
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

// --- extracting fragments ---

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

// GetTableN — get the n table numbered with 1
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

// --- description structure include ---

type BracketIncludeSpec struct {
	RawTag   string
	File     string
	Fragment string
	Index    int
}

// ParseBracketIncludeTag — parses a string like "[include/file.docx/table/2]" without regexp.
func ParseBracketIncludeTag(tag string, data map[string]any) (BracketIncludeSpec, error) {
	spec := BracketIncludeSpec{
		RawTag:   tag,
		Fragment: "body",
		Index:    1,
	}

	// local spoofing var inside the include path
	for k, v := range data {
		placeholder := "%" + k + "%"
		if strings.Contains(tag, placeholder) {
			tag = strings.ReplaceAll(tag, placeholder, fmt.Sprint(v))
		}
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

	// looking for a segment with docx
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

	// assembling the path to the file
	fileSegments := parts[1 : idxDocx+1]
	filePath := path.Clean(path.Join(fileSegments...))
	if filePath == "." || filePath == "" {
		return spec, fmt.Errorf("include: empty file path")
	}
	spec.File = filePath

	// Dismantling the remainder (fragment)
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
// ProcessTrimTags - Cleans up {~} and {-} with neighboring <w:t> and valid spaces
// ============================================================================

// ProcessTrimTags — removes spaces, tabs, and hyphenation around {~}/{-} without breaking the structure of Word.
func (d *Docx) ProcessTrimTags(body string) string {
	// 1. substitution of special tags with symbols
	body = strings.ReplaceAll(body, "<w:tab/>", "<w:t>\t</w:t>")
	body = strings.ReplaceAll(body, "<w:br/>", "<w:t>\n</w:t>")

	// 2. breaking it down into paragraphs
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

		// filter: clean only if the paragraph contains {~ or {-
		if !strings.Contains(content, "{~") && !strings.Contains(content, "{-") &&
			!strings.Contains(content, "~}") && !strings.Contains(content, "-}") {
			continue
		}

		// 3. Work inside the paragraph as before — line by line according to <w:r>
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

			// 4. We restore special tags without breaking the XML structure
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

// cleanTrimTags — removes spaces, tabs, and hyphens around {~}/{-} by correcting spaces.
func cleanTrimTags(s string) string {
	// {~...~} — eats everything
	s = regexp.MustCompile(`[\s]*\{~`).ReplaceAllString(s, "{")
	s = regexp.MustCompile(`~}[\s]*`).ReplaceAllString(s, "}")
	// {-...-} — eats only spaces and tabs
	s = regexp.MustCompile(`[ \t]*\{-`).ReplaceAllString(s, "{")
	s = regexp.MustCompile(`-}[ \t]*`).ReplaceAllString(s, "}")
	// Removing markers
	s = strings.ReplaceAll(s, "{~", "{")
	s = strings.ReplaceAll(s, "~}", "}")
	s = strings.ReplaceAll(s, "{-", "{")
	s = strings.ReplaceAll(s, "-}", "}")
	// Restore the space before the tag
	s = regexp.MustCompile(`([A-Za-zА-Яа-яЁё])\{`).ReplaceAllString(s, `$1 {`)
	return s
}

// extractText — Pulls out the text from <w:t ...>...</w:t>.
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

func expandVars(s string, data map[string]any) string {
	// Look for %var% and substitute the value from the data
	for k, v := range data {
		placeholder := "%" + k + "%"
		if strings.Contains(s, placeholder) {
			s = strings.ReplaceAll(s, placeholder, fmt.Sprint(v))
		}
	}
	return s
}
