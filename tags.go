package docxgen

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
)

// ============================================================================
//  Раздел: Обработка параграфов и маркеров Word XML
// ============================================================================

// ReplaceTagWithParagraph — заменяет параграф, содержащий {tag}, на произвольный XML-фрагмент.
// Используется для "unwrap"-механизма и вставки таблиц/фрагментов без лишнего <w:p>.
func ReplaceTagWithParagraph(body, tag, content string) string {
	var out strings.Builder
	pos := 0

	for {
		start := strings.Index(body[pos:], "<w:p>")
		if start < 0 {
			out.WriteString(body[pos:])
			break
		}
		start += pos

		end := strings.Index(body[start:], "</w:p>")
		if end < 0 {
			out.WriteString(body[pos:])
			break
		}
		end += start + len("</w:p>")

		paragraph := body[start:end]
		text := extractParagraphText(paragraph)

		if !strings.Contains(text, tag) {
			out.WriteString(body[pos:end])
			pos = end
			continue
		}

		// тег единственный в параграфе
		if strings.TrimSpace(text) == tag {
			out.WriteString(body[pos:start])
			out.WriteString(content)
			pos = end
			continue
		}

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
	base := filepath.Dir(d.filePath)
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

func GetTableN(content string, n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("include: bad table index")
	}
	parts := strings.Split(content, TablePartTag)
	idx := n*2 - 1
	if idx < 0 || idx >= len(parts) {
		return "", fmt.Errorf("include: table %d not found", n)
	}
	return TableOpeningTag + parts[idx] + TablePartTag, nil
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
//  Раздел: TrimTags — структурное удаление пробелов по маркерам {~}, {-}
// ============================================================================

// --- классификация пробелов -------------------------------------------------

type wsKind int

const (
	wsNone       wsKind = iota // не пробельный узел
	wsSpacesTabs               // только пробелы/табы
	wsHasNewline               // есть переносы строк
)

// classifyWS — определяет тип пробельного узла <w:t>.
func classifyWS(s string) wsKind {
	if s == "\t" {
		return wsSpacesTabs
	}
	if s == "\n" {
		return wsHasNewline
	}
	// только пробелы и табы
	if strings.IndexFunc(s, func(r rune) bool { return !(r == ' ' || r == '\t') }) == -1 {
		return wsSpacesTabs
	}
	// только пробелы/табы/переносы
	if strings.IndexFunc(s, func(r rune) bool { return !(r == ' ' || r == '\t' || r == '\n') }) == -1 {
		if strings.ContainsRune(s, '\n') {
			return wsHasNewline
		}
		return wsSpacesTabs
	}
	return wsNone
}

// --- маски обрезки ----------------------------------------------------------

type trimSide int

const (
	trimNone trimSide = iota
	trimST            // удалять пробелы и табы
	trimSTN           // удалять пробелы, табы и переносы строк
)

// computeMasks — определяет маску удаления слева/справа по содержимому {тега}.
func computeMasks(tagText string) (left, right trimSide) {
	if strings.Contains(tagText, "{~") {
		left = trimSTN
	} else if strings.Contains(tagText, "{-") {
		left = trimST
	}
	if strings.Contains(tagText, "~}") {
		right = trimSTN
	} else if strings.Contains(tagText, "-}") {
		right = trimST
	}
	return
}

// canEat — можно ли удалить пробельный узел с учётом маски.
func canEat(kind wsKind, side trimSide) bool {
	if side == trimNone || kind == wsNone {
		return false
	}
	if side == trimST {
		return kind == wsSpacesTabs
	}
	return kind == wsSpacesTabs || kind == wsHasNewline
}

// stripMarkersInPlace — превращает {~...~}, {-...-}, {~...}, {...~}, {-...}, {...-} → {...}.
func stripMarkersInPlace(s string) string {
	s = strings.ReplaceAll(s, "{~", "{")
	s = strings.ReplaceAll(s, "{-", "{")
	s = strings.ReplaceAll(s, "~}", "}")
	s = strings.ReplaceAll(s, "-}", "}")
	return s
}

// --- структуры Word XML -----------------------------------------------------

type textStruct struct {
	XMLName xml.Name `xml:"t"`
	Text    string   `xml:",chardata"`
}

type runStruct struct {
	XMLName xml.Name     `xml:"r"`
	Texts   []textStruct `xml:"t"`
	Tabs    []struct{}   `xml:"tab"`
	Breaks  []struct{}   `xml:"br"`
}

type paragraphStruct struct {
	XMLName xml.Name    `xml:"p"`
	Runs    []runStruct `xml:"r"`
}

// MarshalXML — сериализация <w:p> с сохранением префикса w:.
func (p paragraphStruct) MarshalXML(e *xml.Encoder, _ xml.StartElement) error {
	start := xml.StartElement{Name: xml.Name{Local: "w:p"}}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, r := range p.Runs {
		if err := encodeRun(e, r); err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// encodeRun — вспомогательная сериализация <w:r>.
func encodeRun(e *xml.Encoder, r runStruct) error {
	start := xml.StartElement{Name: xml.Name{Local: "w:r"}}
	if err := e.EncodeToken(start); err != nil {
		return err
	}
	for _, t := range r.Texts {
		err := e.EncodeToken(xml.StartElement{Name: xml.Name{Local: "w:t"}})
		if err != nil {
			return err
		}
		err = e.EncodeToken(xml.CharData([]byte(t.Text)))
		if err != nil {
			return err
		}
		err = e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "w:t"}})
		if err != nil {
			return err
		}
	}
	for range r.Tabs {
		err := e.EncodeToken(xml.StartElement{Name: xml.Name{Local: "w:tab"}})
		if err != nil {
			return err
		}
		err = e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "w:tab"}})
		if err != nil {
			return err
		}
	}
	for range r.Breaks {
		err := e.EncodeToken(xml.StartElement{Name: xml.Name{Local: "w:br"}})
		if err != nil {
			return err
		}
		err = e.EncodeToken(xml.EndElement{Name: xml.Name{Local: "w:br"}})
		if err != nil {
			return err
		}
	}
	return e.EncodeToken(xml.EndElement{Name: start.Name})
}

// dropTextAt — удаляет i-й элемент из списка текстов.
func dropTextAt(texts []textStruct, i int) []textStruct {
	copy(texts[i:], texts[i+1:])
	return texts[:len(texts)-1]
}

// --- основная логика --------------------------------------------------------

// ProcessTrimTags — проход по абзацам и удаление пробелов вокруг {~}/{-}.
//
// Алгоритм:
//  1. заворачивает <w:tab/> и <w:br/> в текстовые узлы \t и \n,
//  2. парсит <w:p> в структуру,
//  3. для каждого <w:r> удаляет пробельные <w:t> вокруг тега,
//  4. очищает сами теги от маркеров,
//  5. собирает всё обратно в XML.
func (d *Docx) ProcessTrimTags(body string) string {
	parts := strings.Split(body, ParagraphPartTag)
	for i := 0; i < len(parts); i++ {
		paraXML := ParagraphOpeningTag + parts[i] + ParagraphPartTag

		// пропускаем, если нет маркеров
		if !hasAnyTrimMarkers(paraXML) {
			continue
		}

		// локально оборачиваем табы/переносы в текст
		wrapped := wrapTabsBreaksInParagraph(paraXML)

		var p paragraphStruct
		if err := xml.Unmarshal([]byte(wrapped), &p); err != nil {
			continue
		}

		for ri := range p.Runs {
			r := &p.Runs[ri]
			if len(r.Texts) == 0 {
				continue
			}

			ti := 0
			for ti < len(r.Texts) {
				txt := r.Texts[ti].Text

				if !(strings.Contains(txt, "{~") ||
					strings.Contains(txt, "~}") ||
					strings.Contains(txt, "{-") ||
					strings.Contains(txt, "-}")) {
					ti++
					continue
				}

				leftMask, rightMask := computeMasks(txt)

				// влево
				li := ti - 1
				for li >= 0 {
					if !canEat(classifyWS(r.Texts[li].Text), leftMask) {
						break
					}
					r.Texts = dropTextAt(r.Texts, li)
					ti--
					li--
				}

				// вправо
				ri2 := ti + 1
				for ri2 < len(r.Texts) {
					if !canEat(classifyWS(r.Texts[ri2].Text), rightMask) {
						break
					}
					r.Texts = dropTextAt(r.Texts, ri2)
				}

				r.Texts[ti].Text = stripMarkersInPlace(r.Texts[ti].Text)
				ti++
			}
		}

		var buf bytes.Buffer
		enc := xml.NewEncoder(&buf)
		_ = enc.Encode(&p)
		out := strings.TrimSpace(strings.TrimPrefix(buf.String(), xml.Header))
		out = unwrapTabsBreaksInXML(out)

		if strings.HasPrefix(out, ParagraphOpeningTag) && strings.HasSuffix(out, ParagraphPartTag) {
			out = strings.TrimPrefix(out, ParagraphOpeningTag)
			out = strings.TrimSuffix(out, ParagraphPartTag)
			parts[i] = out
		} else {
			parts[i] = strings.TrimPrefix(wrapped, ParagraphOpeningTag)
			parts[i] = strings.TrimSuffix(parts[i], ParagraphPartTag)
		}
	}
	return strings.Join(parts, ParagraphPartTag)
}

// ============================================================================
//  Раздел: вспомогательные утилиты XML
// ============================================================================

// hasAnyTrimMarkers — быстрый фильтр: стоит ли вообще разбирать параграф.
func hasAnyTrimMarkers(s string) bool {
	return strings.Contains(s, "{~") ||
		strings.Contains(s, "~}") ||
		strings.Contains(s, "{-") ||
		strings.Contains(s, "-}")
}

// wrapTabsBreaksInParagraph — оборачивает <w:tab/> и <w:br/> в текст,
// чтобы не потерять их при Unmarshal.
func wrapTabsBreaksInParagraph(pXML string) string {
	pXML = strings.ReplaceAll(pXML, "<w:tab/>", "<w:t>\t</w:t>")
	pXML = strings.ReplaceAll(pXML, "<w:br/>", "<w:t>\n</w:t>")
	return pXML
}

// unwrapTabsBreaksInXML — обратная операция после marshal:
// возвращает \t → <w:tab/>, \n → <w:br/>.
func unwrapTabsBreaksInXML(s string) string {
	s = strings.ReplaceAll(s, `<w:t>`+"\t"+`</w:t>`, "<w:tab/>")
	s = strings.ReplaceAll(s, `<w:t>`+"\n"+`</w:t>`, "<w:br/>")
	s = strings.ReplaceAll(s, `<w:t>&#x9;</w:t>`, "<w:tab/>")
	s = strings.ReplaceAll(s, `<w:t>&#xA;</w:t>`, "<w:br/>")
	return s
}
