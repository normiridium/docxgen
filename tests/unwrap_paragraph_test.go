package tests

import (
	"docxgen"
	"strings"
	"testing"
)

// normalize — убирает лишние пробелы и переносы, чтобы не путали тесты.
func normalize(s string) string {
	s = strings.ReplaceAll(s, "\r", "")
	s = strings.ReplaceAll(s, "\n", "")
	s = strings.Join(strings.Fields(s), " ")
	return s
}

// --- тесты ReplaceTagWithParagraph -----------------------------------

func TestReplaceTagWithParagraph(t *testing.T) {
	type tc struct {
		name   string
		input  string
		tag    string
		output string
	}

	tests := []tc{
		{
			name: "тег единственный в параграфе",
			input: `<w:body>` +
				`<w:p><w:r><w:t>{tag}</w:t></w:r></w:p>` +
				`</w:body>`,
			tag:    "{tag}",
			output: `<w:body>CONTENT</w:body>`,
		},
		{
			name: "тег внутри текста одного <w:t>",
			input: `<w:body>` +
				`<w:p><w:r><w:t>AAA {tag} BBB</w:t></w:r></w:p>` +
				`</w:body>`,
			tag: "{tag}",
			output: `<w:body>` +
				`<w:p><w:r><w:t xml:space="preserve">AAA</w:t></w:r></w:p>` +
				`CONTENT` +
				`<w:p><w:r><w:t xml:space="preserve">BBB</w:t></w:r></w:p>` +
				`</w:body>`,
		},
		{
			name: "тег отдельным <w:t> в середине run",
			input: `<w:body>` +
				`<w:p><w:r><w:t>AAA</w:t><w:t>{tag}</w:t><w:t>BBB</w:t></w:r></w:p>` +
				`</w:body>`,
			tag: "{tag}",
			output: `<w:body>` +
				`<w:p><w:r><w:t xml:space="preserve">AAA</w:t></w:r></w:p>` +
				`CONTENT` +
				`<w:p><w:r><w:t xml:space="preserve">BBB</w:t></w:r></w:p>` +
				`</w:body>`,
		},
		{
			name: "тег в начале текста",
			input: `<w:body>` +
				`<w:p><w:r><w:t>{tag} BBB</w:t></w:r></w:p>` +
				`</w:body>`,
			tag: "{tag}",
			output: `<w:body>` +
				`CONTENT` +
				`<w:p><w:r><w:t xml:space="preserve">BBB</w:t></w:r></w:p>` +
				`</w:body>`,
		},
		{
			name: "тег в конце текста",
			input: `<w:body>` +
				`<w:p><w:r><w:t>AAA {tag}</w:t></w:r></w:p>` +
				`</w:body>`,
			tag: "{tag}",
			output: `<w:body>` +
				`<w:p><w:r><w:t xml:space="preserve">AAA</w:t></w:r></w:p>` +
				`CONTENT` +
				`</w:body>`,
		},
		{
			name: "несколько параграфов, только один с тегом",
			input: `<w:body>` +
				`<w:p><w:r><w:t>AAA</w:t></w:r></w:p>` +
				`<w:p><w:r><w:t>{tag}</w:t></w:r></w:p>` +
				`<w:p><w:r><w:t>CCC</w:t></w:r></w:p>` +
				`</w:body>`,
			tag: "{tag}",
			output: `<w:body>` +
				`<w:p><w:r><w:t>AAA</w:t></w:r></w:p>` +
				`CONTENT` +
				`<w:p><w:r><w:t>CCC</w:t></w:r></w:p>` +
				`</w:body>`,
		},
		{
			name: "нет параграфов с тегом",
			input: `<w:body>` +
				`<w:p><w:r><w:t>AAA BBB CCC</w:t></w:r></w:p>` +
				`</w:body>`,
			tag:    "{tag}",
			output: `<w:body><w:p><w:r><w:t>AAA BBB CCC</w:t></w:r></w:p></w:body>`,
		},
		{
			name: "повреждённый параграф (Unmarshal fail)",
			input: `<w:body>` +
				`<w:p><w:r><w:t>AAA {tag BBB</w:t></w:r></w:p>` +
				`</w:body>`,
			tag:    "{tag}",
			output: `<w:body><w:p><w:r><w:t>AAA {tag BBB</w:t></w:r></w:p></w:body>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := docxgen.ReplaceTagWithParagraph(tt.input, tt.tag, "CONTENT")

			if normalize(out) != normalize(tt.output) {
				t.Errorf("unexpected output:\n got:  %s\n want: %s", out, tt.output)
			}
		})
	}
}
