package docxgen

import (
	"encoding/xml"
	"strings"
)

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

// replaceTagWithParagraph — удаляет параграф с тегом и возвращает обновлённый контент
func replaceTagWithParagraph(body, tag, content string) string {
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

		body = replaceTagWithParagraph(body, starTag, normalized)
	}
}
