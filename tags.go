package docxgen

import (
	"encoding/xml"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	securejoin "github.com/cyphar/filepath-securejoin"
)

// -----------------------------
// БАЗОВЫЕ XML-КОНСТРУКЦИИ
// -----------------------------

// paragraphStruct — минимальная структура для проверки параграфа
type paragraphStruct struct {
	XMLName xml.Name `xml:"p"`
	R       struct {
		T string `xml:"t"`
	} `xml:"r"`
}

// RepairTags — аккуратно чинит теги, если их порвал Word/LibreOffice на несколько <w:t>.
// ВАЖНО: мы трогаем только содержимое между { и }, вырезая там служебные <w:t ...> и </w:t>,
// остальной текст документа (включая кириллицу) не меняем.
func (d *Docx) RepairTags(body string) (string, error) {
	var b strings.Builder
	inTag := false
	i := 0
	for i < len(body) {
		if !inTag {
			// Ожидаем открывающую скобку — начинаем режим «внутри тега»
			if body[i] == '{' {
				inTag = true
				b.WriteByte('{')
				i++
				continue
			}
			b.WriteByte(body[i])
			i++
			continue
		}

		// Внутри тега: выпиливаем границы ран-ов Word
		if strings.HasPrefix(body[i:], "<w:t") {
			// пропускаем до '>'
			j := strings.IndexByte(body[i:], '>')
			if j < 0 {
				// поломанный XML — просто дописываем хвост и выходим
				b.WriteString(body[i:])
				break
			}
			i += j + 1
			continue
		}
		if strings.HasPrefix(body[i:], "</w:t>") {
			i += len("</w:t>")
			continue
		}

		// (опционально можно выкидывать и другие w:теги внутри фигурных,
		// если вдруг офис порвёт сильнее)
		if strings.HasPrefix(body[i:], "<w:") {
			j := strings.IndexByte(body[i:], '>')
			if j < 0 {
				b.WriteString(body[i:])
				break
			}
			i += j + 1
			continue
		}

		// NB: специально перечисляем <w:t>, <w:r>, <w:rPr> —
		// если убрать и схлопывать всё подряд "<w:", Word/LibreOffice
		// могут порвать скобки так, что полетят лишние куски XML.
		// Поэтому правила дублируют друг друга, но это осознанно.

		if strings.HasPrefix(body[i:], "<w:r") || strings.HasPrefix(body[i:], "</w:r>") {
			j := strings.IndexByte(body[i:], '>')
			if j < 0 {
				b.WriteString(body[i:])
				break
			}
			i += j + 1
			continue
		}

		if strings.HasPrefix(body[i:], "<w:rPr") || strings.HasPrefix(body[i:], "</w:rPr>") {
			j := strings.IndexByte(body[i:], '>')
			if j < 0 {
				b.WriteString(body[i:])
				break
			}
			i += j + 1
			continue
		}

		// Закрывающая фигурная — выходим из режима
		if body[i] == '}' {
			inTag = false
			b.WriteByte('}')
			i++
			continue
		}

		// Обычный символ внутри тега
		b.WriteByte(body[i])
		i++
	}

	return b.String(), nil
}

// ReplaceTagWithParagraph — удаляет параграф с тегом и возвращает обновлённый контент.
// Используется как «unwrap»-механизм: параграф, содержащий тег, заменяется непосредственно контентом.
func ReplaceTagWithParagraph(body, tag, content string) string {
	paragraphs := strings.Split(body, ParagraphPartTag)
	for i, paragraph := range paragraphs {
		if strings.Contains(paragraph, tag) {
			p := new(paragraphStruct)
			_ = xml.Unmarshal([]byte(ParagraphOpeningTag+paragraph+ParagraphPartTag), p)
			if strings.Contains(p.R.T, tag) {
				// заменяем параграф на "якорь"
				paragraphs[i] = tag + ClosingPartTag
			}
		}
	}

	filtered := strings.Join(paragraphs, ParagraphPartTag)
	replaced := strings.ReplaceAll(
		filtered,
		ParagraphOpeningTag+tag+ParagraphClosingTag,
		content,
	)
	return replaced
}

// ProcessUnWrapParagraphTags — ищет все теги вида {*tag*}, вырезает параграф и превращает их в блочные {tag}.
func (d *Docx) ProcessUnWrapParagraphTags(body string) string {
	for {
		start := strings.Index(body, "{*")
		if start == -1 {
			return body // больше нет звёздочных тегов
		}
		endRel := strings.Index(body[start:], "*}")
		if endRel == -1 {
			return body // незакрытый тег
		}

		starTag := body[start : start+endRel+2] // "{*tag*}"
		name := strings.TrimSpace(body[start+2 : start+endRel])
		normalized := "{" + name + "}"

		body = ReplaceTagWithParagraph(body, starTag, normalized)
	}
}

// ResolveIncludes — находит все [include/...] и подставляет нужный фрагмент.
// Выполняется ДО TransformTemplate/ExecuteTemplate.
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
		end = start + end + 1

		raw := body[start:end]
		spec, perr := ParseBracketIncludeTag(raw)
		if perr != nil {
			// удаляем маркер целиком
			body = body[:start] + body[end:]
			continue
		}

		xmlFrag, _, ferr := d.getIncludeXML(spec)
		if ferr != nil {
			// удаляем маркер целиком
			body = body[:start] + body[end:]
			continue
		}

		// успешная замена
		body = ReplaceTagWithParagraph(body, spec.RawTag, xmlFrag)
	}

	return body
}

// getIncludeXML — открыть дочерний docx и извлечь фрагмент
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

// openFragmentDoc — открыть вложенный DOCX из includeBase (если задан) или из каталога исходного файла
func (d *Docx) openFragmentDoc(rel string) (*Docx, error) {
	// защита: разрешаем только .docx и .dotx
	ext := strings.ToLower(filepath.Ext(rel))
	switch ext {
	case ".docx", ".dotx":
		// ok
	default:
		return nil, fmt.Errorf("unsupported include extension (allowed: .docx, .dotx): %s", rel)
	}

	// собираем безопасно
	base := filepath.Dir(d.filePath)
	full, err := securejoin.SecureJoin(base, rel)
	if err != nil {
		return nil, fmt.Errorf("forbidden include path: %w", err)
	}

	// простая проверка существования
	if _, err := os.Stat(full); err != nil {
		return nil, err
	}
	return Open(full)
}

// GetBodyFragment — вытащить содержимое <w:body>...</w:body> без самих тегов
func GetBodyFragment(content string) (string, error) {
	parts := strings.Split(content, BodyOpeningTag)
	if len(parts) < 2 {
		return "", fmt.Errorf("include: body open not found")
	}
	rest := strings.Split(parts[1], BodyClosingTag)
	if len(rest) < 2 {
		return "", fmt.Errorf("include: body close not found")
	}
	return rest[0], nil
}

// GetTableN — получить n-ую таблицу (нумерация с 1)
func GetTableN(content string, n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("include: bad table index")
	}
	parts := strings.Split(content, TablePartTag)
	// таблицы — нечётные позиции: 1,3,5,... (если считать с 0)
	idx := n*2 - 1
	if idx < 0 || idx >= len(parts) {
		return "", fmt.Errorf("include: table %d not found", n)
	}
	return TableOpeningTag + parts[idx] + TablePartTag, nil
}

// GetParagraphN — получить n-й параграф (нумерация с 1) целиком <w:p>...</w:p>
func GetParagraphN(content string, n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("include: bad paragraph index")
	}
	parts := strings.Split(content, ParagraphPartTag)
	// параграфы — действительно нечётные куски (между w:p>)
	idx := n*2 - 1
	if idx < 0 || idx >= len(parts) {
		return "", fmt.Errorf("include: paragraph %d not found", n)
	}
	return ParagraphOpeningTag + parts[idx] + ParagraphPartTag, nil
}

type BracketIncludeSpec struct {
	RawTag   string // исходный маркер, например: [include/blocks/x.docx/table/2]
	File     string // blocks/x.docx
	Fragment string // body | table | p
	Index    int    // 1..N (для table/p), по умолчанию 1
}

// ParseBracketIncludeTag — парсит строку вида "[include/blocks/x.docx/table/2]" без регулярок.
// Поддерживает:
//
//	[include/file.docx]
//	[include/file.docx/body]
//	[include/file.docx/table]
//	[include/file.docx/table/3]
//	[include/file.docx/p/2] (синоним paragraph)
//
// Возвращает spec с нормализованными полями или ошибку.
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

	// Ищем сегмент с .docx
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

	// Собираем путь к файлу (на случай подкаталогов: blocks/nested/persons.docx)
	fileSegments := parts[1 : idxDocx+1]
	filePath := path.Clean(path.Join(fileSegments...))
	if filePath == "." || filePath == "" {
		return spec, fmt.Errorf("include: empty file path")
	}

	spec.File = filePath

	// Остальные сегменты — описание фрагмента
	rest := parts[idxDocx+1:] // []string{} либо ["table"] либо ["table","2"] ...
	if len(rest) == 0 {
		// по умолчанию body/1
		return spec, nil
	}

	switch strings.ToLower(strings.TrimSpace(rest[0])) {
	case "body":
		spec.Fragment = "body"
	case "table":
		spec.Fragment = "table"
		if len(rest) >= 2 && strings.TrimSpace(rest[1]) != "" {
			n, err := strconv.Atoi(strings.TrimSpace(rest[1]))
			if err != nil || n <= 0 {
				return spec, fmt.Errorf("include: bad table index")
			}
			spec.Index = n
		}
	case "p", "paragraph":
		spec.Fragment = "p"
		if len(rest) >= 2 && strings.TrimSpace(rest[1]) != "" {
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
