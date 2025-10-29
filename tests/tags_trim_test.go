package tests

import (
	"bytes"
	"docxgen"
	"encoding/xml"
	"strings"
	"testing"
)

func normalizeXML(s string) string {
	// убираем xml.Header и лишние пробелы
	s = strings.TrimPrefix(s, xml.Header)
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.ReplaceAll(s, " ", "")
	return s
}

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
