package tests

import (
	"bytes"
	"docxgen"
	"strconv"
	"strings"
	"testing"
)

func TestProcessTrimTags_Simplified(t *testing.T) {
	doc := &docxgen.Docx{}

	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "простое {~..~}",
			in:   `<w:p><w:r><w:t>{~fio~}</w:t></w:r></w:p>`,
			want: `<w:p><w:r><w:t>{fio}</w:t></w:r></w:p>`,
		},
		{
			name: "пробелы вокруг {~..~}",
			in:   `<w:p><w:r><w:t> </w:t><w:t>{~fio~}</w:t><w:t> </w:t></w:r></w:p>`,
			want: `<w:p><w:r><w:t>{fio}</w:t></w:r></w:p>`,
		},
		{
			name: "табы вокруг {-..-}",
			in:   `<w:p><w:r><w:tab/><w:t>{-fio-}</w:t><w:tab/></w:r></w:p>`,
			want: `<w:p><w:r><w:t>{fio}</w:t></w:r></w:p>`,
		},
		{
			name: "переносы вокруг {~..~}",
			in:   `<w:p><w:r><w:br/><w:t>{~fio~}</w:t><w:br/></w:r></w:p>`,
			want: `<w:p><w:r><w:t>{fio}</w:t></w:r></w:p>`,
		},
		{
			name: "очень много пробелов",
			in:   `<w:p><w:r><w:t>        </w:t><w:t>{-fio-}</w:t><w:br/></w:r></w:p>`,
			want: `<w:p><w:r><w:t>{fio}</w:t><w:br/></w:r></w:p>`,
		},
		{
			name: "в середине предложения",
			in:   `<w:p><w:r><w:t>Уважаемый {~fio~}, благодарим.</w:t></w:r></w:p>`,
			want: `<w:p><w:r><w:t>Уважаемый {fio}, благодарим.</w:t></w:r></w:p>`,
		},
		{
			name: "направленные {~...} и {...-}",
			in:   `<w:p><w:r><w:t> </w:t><w:t>{~fio}</w:t><w:tab/><w:t>{fio-}</w:t><w:t> </w:t></w:r></w:p>`,
			want: `<w:p><w:r><w:t>{fio}</w:t><w:tab/><w:t>{fio}</w:t></w:r></w:p>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := doc.ProcessTrimTags(tt.in)

			gN := normalizeXML(got)
			wN := normalizeXML(tt.want)

			if gN != wN {
				var buf bytes.Buffer
				buf.WriteString("mismatch:\n got: ")
				buf.WriteString(got)
				buf.WriteString("\nwant: ")
				buf.WriteString(tt.want)
				t.Error(buf.String())
			}
		})
	}
}

// -----------------------------------------
// главный матричный тест
// -----------------------------------------

func TestProcessTrimTags_Matrix(t *testing.T) {
	d := docxgen.Docx{}

	for b, before := range xmlBefore {
		for a, after := range xmlAfter {
			for tt, tag := range tagForms {
				name := "b_" + itoa(b) + "_a_" + itoa(a) + "_t_" + itoa(tt)
				t.Run(name, func(t *testing.T) {
					src := xmlPrefix + before + `<w:t xml:space="preserve">` + tag + `</w:t>` + after + xmlSuffix
					out := d.ProcessTrimTags(src)

					assertXMLBalanced(t, out)
					isTilde := strings.Contains(tag, "{~") || strings.Contains(tag, "~}")
					assertTrimBehavior(t, src, out, isTilde)
				})
			}
		}
	}
}

// простая утилита для индексов
func itoa(i int) string { return strconv.Itoa(i) }

// normalizeXML — убирает форматирование и схлопывает пробелы.
func normalizeXML(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, "\t", "")
	return strings.Join(strings.Fields(s), " ")
}

// -----------------------------------------
// вспомогательные XML-заготовки
// -----------------------------------------
var (
	xmlPrefix = `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>` +
		`<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">` +
		`<w:body><w:p><w:r>`

	xmlSuffix = `</w:r></w:p></w:body></w:document>`

	xmlBefore = []string{
		`<w:t>Начало предложения, </w:t>`,
		``,
		`<w:t xml:space="preserve"> </w:t>`,
		`<w:tab/><w:tab/>`,
		`<w:br/><w:br/>`,
		`<w:br/><w:t xml:space="preserve"> </w:t>`,
	}

	xmlAfter = []string{
		`<w:t> конец строки.</w:t>`,
		``,
		`<w:t xml:space="preserve"> </w:t>`,
		`<w:tab/><w:tab/>`,
		`<w:br/><w:br/>`,
		`<w:t xml:space="preserve"> </w:t><w:br/>`,
	}

	tagForms = []string{
		`{~demo|x:` + "`a`:`b`" + `~}`,
		`{~demo|x:` + "`a`:`b`" + `}`,
		`{demo|x:` + "`a`:`b`" + `~}`,
		`{-demo|x:` + "`a`:`b`" + `-}`,
		`{-demo|x:` + "`a`:`b`" + `}`,
		`{demo|x:` + "`a`:`b`" + `-}`,
	}
)

// -----------------------------------------
// проверки целостности XML
// -----------------------------------------
// assertXMLBalanced — проверяет парность ключевых тегов, игнорируя самозакрывающиеся.
func assertXMLBalanced(t *testing.T, s string) {
	// очищаем самозакрывающиеся теги (<w:tab/>, <w:br/>, <w:sym/>, и т.п.)
	clean := s
	clean = strings.ReplaceAll(clean, "<w:tab/>", "")
	clean = strings.ReplaceAll(clean, "<w:br/>", "")
	clean = strings.ReplaceAll(clean, "<w:drawing/>", "")
	clean = strings.ReplaceAll(clean, "<w:noBreakHyphen/>", "")
	clean = strings.ReplaceAll(clean, "<w:softHyphen/>", "")

	// теперь считаем только настоящие парные
	for _, tag := range []string{"w:t>", "w:r>", "w:p>"} {
		openTag := strings.Count(clean, "<"+tag[:len(tag)-1]) // "<w:t", "<w:r", "<w:p"
		closeTag := strings.Count(clean, "</"+tag)
		if openTag != closeTag {
			t.Fatalf("❌ нарушена парность <%s>: %d ≠ %d\n\n%s", tag[:len(tag)-1], openTag, closeTag, s)
		}
	}
}

// -----------------------------------------
// проверка логики удаления пробелов/переносов
// -----------------------------------------
// assertTrimBehavior — проверяет корректность чистки по типу {~} или {-}.
func assertTrimBehavior(t *testing.T, src, out string, isTilde bool) {
	hasBreak := strings.Contains(src, "<w:br/>")

	if isTilde {
		// --- переносы ---
		// если тильда открывает блок ({~...), проверяем только левую сторону
		if strings.Contains(src, "{~") {
			if strings.Contains(src, "<w:br/><w:t xml:space=\"preserve\">{~") {
				parts := strings.SplitN(out, "{~", 2)
				if len(parts) == 2 && strings.Contains(parts[0], "<w:br/>") {
					t.Errorf("{~} не удалил левый перенос\nSRC:\n%s\nOUT:\n%s", src, out)
				}
			}
		}
		// если тильда закрывает блок (...~}), проверяем только правую сторону
		if strings.Contains(src, "~}") {
			if strings.Contains(src, "~}</w:t><w:br/>") {
				parts := strings.SplitN(out, "~}</w:t>", 2)
				if len(parts) == 2 && strings.Contains(parts[1], "<w:br/>") {
					t.Errorf("{~} не удалил правый перенос\nSRC:\n%s\nOUT:\n%s", src, out)
				}
			}
		}

		// --- табуляция ---
		// проверяем только табуляции, стоящие непосредственно после закрывающего тега
		if strings.Contains(src, "~}</w:t><w:tab/>") {
			parts := strings.SplitN(out, "~}</w:t>", 2)
			if len(parts) == 2 && strings.Contains(parts[1], "<w:tab/>") {
				t.Errorf("{~} не удалил правую табуляцию\nSRC:\n%s\nOUT:\n%s", src, out)
			}
		}
		// и только те, что прямо перед открывающим
		if strings.Contains(src, "<w:tab/><w:t xml:space=\"preserve\">{~") {
			parts := strings.SplitN(out, "{~", 2)
			if len(parts) == 2 && strings.Contains(parts[0], "<w:tab/>") {
				t.Errorf("{~} не удалил левую табуляцию\nSRC:\n%s\nOUT:\n%s", src, out)
			}
		}

		// --- пробелы ---
		leftSpace := `> </w:t><w:t xml:space="preserve">{~`
		rightSpace := `}</w:t><w:t xml:space="preserve"> </w:t>`
		if strings.Contains(src, leftSpace) && strings.Contains(out, `> </w:t><w:t xml:space="preserve">{demo}`) {
			t.Errorf("{~} не удалил левый пробел\nSRC:\n%s\nOUT:\n%s", src, out)
		}
		if strings.Contains(src, rightSpace) && strings.Contains(out, `{demo}</w:t><w:t xml:space="preserve"> </w:t>`) {
			t.Errorf("{~} не удалил правый пробел\nSRC:\n%s\nOUT:\n%s", src, out)
		}

	} else {
		// {-} не должен удалять переносы
		if hasBreak && !strings.Contains(out, "<w:br/>") {
			t.Errorf("{-} удалил перенос (что неправильно)\nSRC:\n%s\nOUT:\n%s", src, out)
		}
	}
}
