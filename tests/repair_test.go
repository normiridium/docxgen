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

// TestRepairTags_DontTouchJson проверяет, что JSON не ломается
func TestRepairTags_DontTouchJson(t *testing.T) {
	input := `<w:p><w:r><w:t>{"key": "value", "num": 123}</w:t></w:r></w:p>`
	want := input

	d := &docxgen.Docx{}
	got, err := d.RepairTags(input)
	if err != nil {
		t.Fatalf("RepairTags_DontTouchJson error: %v", err)
	}
	if got != want {
		t.Errorf("RepairTags_DontTouchJson:\n got  %q\n want %q", got, want)
	}
}

// TestRepairTags_DontTouchMath проверяет, что формулы с фигурными не трогаются
func TestRepairTags_DontTouchMath(t *testing.T) {
	input := `<w:p><w:r><w:t>f(x) = {x^2 + 1}</w:t></w:r></w:p>`
	want := input

	d := &docxgen.Docx{}
	got, err := d.RepairTags(input)
	if err != nil {
		t.Fatalf("RepairTags_DontTouchMath error: %v", err)
	}
	if got != want {
		t.Errorf("RepairTags_DontTouchMath:\n got  %q\n want %q", got, want)
	}
}

// TestRepairTags_DontTouchMarkdown проверяет, что markdown-фигурные не трогаются
func TestRepairTags_DontTouchMarkdown(t *testing.T) {
	input := `<w:p><w:r><w:t>текст {в фигурных скобках} не тег</w:t></w:r></w:p>`
	want := input

	d := &docxgen.Docx{}
	got, err := d.RepairTags(input)
	if err != nil {
		t.Fatalf("RepairTags_DontTouchMarkdown error: %v", err)
	}
	if got != want {
		t.Errorf("RepairTags_DontTouchMarkdown:\n got  %q\n want %q", got, want)
	}
}

// TestRepairTags_TableTag проверяет, что [table/users] чинится корректно
func TestRepairTags_TableTag(t *testing.T) {
	input := `<w:p><w:r><w:t>[</w:t></w:r><w:r><w:t>table/users</w:t></w:r><w:r><w:t>]</w:t></w:r></w:p>`
	want := `<w:p><w:r><w:t>[table/users]</w:t></w:r></w:p>`

	d := &docxgen.Docx{}
	got, err := d.RepairTags(input)
	if err != nil {
		t.Fatalf("RepairTags_TableTag error: %v", err)
	}
	if got != want {
		t.Errorf("RepairTags_TableTag:\n got  %q\n want %q", got, want)
	}
}

// TestRepairTags_IncludeTag проверяет, что [include/x.docx/table/2] чинится корректно
func TestRepairTags_IncludeTag(t *testing.T) {
	input := `<w:p><w:r><w:t>[</w:t></w:r><w:r><w:t>include/x.docx/table/2</w:t></w:r><w:r><w:t>]</w:t></w:r></w:p>`
	want := `<w:p><w:r><w:t>[include/x.docx/table/2]</w:t></w:r></w:p>`

	d := &docxgen.Docx{}
	got, err := d.RepairTags(input)
	if err != nil {
		t.Fatalf("RepairTags_IncludeTag error: %v", err)
	}
	if got != want {
		t.Errorf("RepairTags_IncludeTag:\n got  %q\n want %q", got, want)
	}
}

// TestRepairTags_TableCloseTag проверяет, что [/table] чинится корректно
func TestRepairTags_TableCloseTag(t *testing.T) {
	input := `<w:p><w:r><w:t>[</w:t></w:r><w:r><w:t>/table</w:t></w:r><w:r><w:t>]</w:t></w:r></w:p>`
	want := `<w:p><w:r><w:t>[/table]</w:t></w:r></w:p>`

	d := &docxgen.Docx{}
	got, err := d.RepairTags(input)
	if err != nil {
		t.Fatalf("RepairTags_TableCloseTag error: %v", err)
	}
	if got != want {
		t.Errorf("RepairTags_TableCloseTag:\n got  %q\n want %q", got, want)
	}
}

// TestRepairTags_DontTouchNumberRefs проверяет, что [1][2][3] не трогаются
func TestRepairTags_DontTouchNumberRefs(t *testing.T) {
	input := `<w:p><w:r><w:t>тому если исследования[1][2][3]</w:t></w:r></w:p>`
	want := input // должно остаться без изменений

	d := &docxgen.Docx{}
	got, err := d.RepairTags(input)
	if err != nil {
		t.Fatalf("RepairTags_DontTouchNumberRefs error: %v", err)
	}
	if got != want {
		t.Errorf("RepairTags_DontTouchNumberRefs:\n got  %q\n want %q", got, want)
	}
}

// TestRepairTags_DontTouchLinkTag проверяет, что [<link>1</link>] не трогается
func TestRepairTags_DontTouchLinkTag(t *testing.T) {
	input := `<w:p><w:r><w:t>[<link>1</link>]</w:t></w:r></w:p>`
	want := input

	d := &docxgen.Docx{}
	got, err := d.RepairTags(input)
	if err != nil {
		t.Fatalf("RepairTags_DontTouchLinkTag error: %v", err)
	}
	if got != want {
		t.Errorf("RepairTags_DontTouchLinkTag:\n got  %q\n want %q", got, want)
	}
}

// TestRepairTags_MixedCurlyAndSquare проверяет, что фигурные и квадратные теги обрабатываются независимо
func TestRepairTags_MixedCurlyAndSquare(t *testing.T) {
	input := `<w:p><w:r><w:t>{fio}</w:t></w:r><w:r><w:t>[</w:t></w:r><w:r><w:t>table/users</w:t></w:r><w:r><w:t>]</w:t></w:r></w:p>`
	want := `<w:p><w:r><w:t>{fio}</w:t></w:r><w:r><w:t>[table/users]</w:t></w:r></w:p>`

	d := &docxgen.Docx{}
	got, err := d.RepairTags(input)
	if err != nil {
		t.Fatalf("RepairTags_MixedCurlyAndSquare error: %v", err)
	}
	if got != want {
		t.Errorf("RepairTags_MixedCurlyAndSquare:\n got  %q\n want %q", got, want)
	}
}
