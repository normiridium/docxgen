package tests

import (
	"docxgen"
	"testing"
)

func TestReplaceTagWithParagraph(t *testing.T) {
	body := `<w:body>` +
		`<w:p><w:r><w:t>AAA {tag} BBB</w:t></w:r></w:p>` +
		`</w:body>`

	out := docxgen.ReplaceTagWithParagraph(body, "{tag}", "CONTENT")

	if out != "<w:body>CONTENT</w:body>" {
		t.Fatalf("should not unwrap when tag is not the only run: %s", out)
	}
}

func TestReplaceTagWithBadParagraph(t *testing.T) {
	body := `<w:body>` +
		`<w:p><w:r><w:t>{tag}</w:t><w:t>AAA</w:t></w:r><w:r><w:t>BBB</w:t><w:t>CCC</w:t></w:r></w:p>` +
		`</w:body>`

	out := docxgen.ReplaceTagWithParagraph(body, "{tag}", "CONTENT")

	if out != body {
		t.Fatalf("should not unwrap when tag is not the only run: %s", out)
	}
}
