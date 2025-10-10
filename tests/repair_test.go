package tests

import (
	"strings"
	"testing"

	"docxgen"
)

// TestRepairTags проверяет, что Word-разбитый тег собирается в цельный
func TestRepairTags(t *testing.T) {
	input := `<w:p><w:r><w:t>{f</w:t></w:r><w:r><w:t>io}</w:t></w:r></w:p>`
	want := `<w:p><w:r><w:t>{fio}</w:t></w:r></w:p>`

	d := &docxgen.Docx{}
	got, err := d.RepairTags(input)
	if err != nil {
		t.Fatalf("RepairTags error: %v", err)
	}
	if got != want {
		t.Errorf("RepairTags:\n got  %q\n want %q", got, want)
	}
}

// TestRepairTags_WithModifiers проверяет, что тег с модификаторами собирается правильно
func TestRepairTags_WithModifiers(t *testing.T) {
	input := `<w:p><w:r><w:t>{ti</w:t></w:r>` +
		`<w:r><w:t>tle|truncate:15:` + "`...`" + `}</w:t></w:r></w:p>`

	want := `<w:p><w:r><w:t>{title|truncate:15:` + "`...`" + `}</w:t></w:r></w:p>`

	d := &docxgen.Docx{}
	got, err := d.RepairTags(input)
	if err != nil {
		t.Fatalf("RepairTags error: %v", err)
	}
	if got != want {
		t.Errorf("RepairTags_WithModifiers:\n got  %q\n want %q", got, want)
	}
}

// TestRepairTags_WithRussian проверяет, что кириллица и спецсимволы внутри тега сохраняются
func TestRepairTags_WithRussian(t *testing.T) {
	input := `<w:p><w:r><w:t>{fi</w:t></w:r>` +
		`<w:r><w:t>o|declension:` + "`genitive`" + `:` + "`фамилия </w:t></w:r><w:r><w:t>имя отчество`" + `}</w:t></w:r></w:p>`

	want := `<w:p><w:r><w:t>{fio|declension:` + "`genitive`:`фамилия имя отчество`" + `}</w:t></w:r></w:p>`

	d := &docxgen.Docx{}
	got, err := d.RepairTags(input)
	if err != nil {
		t.Fatalf("RepairTags error: %v", err)
	}
	if got != want {
		t.Errorf("RepairTags_WithRussian:\n got  %q\n want %q", got, want)
	}
}

// TestProcessStarTags проверяет преобразование {*tag*} → {tag}
func TestProcessStarTags(t *testing.T) {
	input := `<w:p><w:r><w:t>{*clients*}</w:t></w:r></w:p>`
	want := `{clients}`

	d := &docxgen.Docx{}
	got := d.ProcessUnWrapParagraphTags(input)

	if !strings.Contains(got, want) {
		t.Errorf("ProcessStarTags:\n got  %q\n want to contain %q", got, want)
	}
}
