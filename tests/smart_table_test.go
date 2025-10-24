package tests

import (
	"docxgen"
	"strings"
	"testing"
)

func TestRenderSmartTable_MixNamedAndPositional(t *testing.T) {
	table := `<w:tbl>` +
		`<w:tr><w:tc><w:p><w:t>HEADER</w:t></w:p></w:tc></w:tr>` +
		`<w:tr><w:tc><w:p><w:t>{title|abbr}</w:t></w:p></w:tc></w:tr>` +
		`<w:tr><w:tc><w:p><w:t>{fio}</w:t></w:p></w:tc><w:tc><w:p><w:t>{pos}</w:t></w:p></w:tc></w:tr>` +
		`<w:tr><w:tc><w:p><w:t>%[1]s</w:t></w:p></w:tc><w:tc><w:p><w:t>%[2]s</w:t></w:p></w:tc></w:tr>` +
		`<w:tr><w:tc><w:p><w:t>FOOTER</w:t></w:p></w:tc></w:tr>` +
		`</w:tbl>`

	items := []any{
		map[string]any{"title_row": map[string]any{"title": "Отдел продаж"}},
		map[string]any{"employee": map[string]any{"fio": "Иванов И.И.", "pos": "Инженер"}},
		map[string]any{"employee": map[string]any{"fio": "Петров М.С.", "pos": "Директор"}},
		map[string]any{"employee": map[string]any{"fio": "Сидоров Н.Д.", "pos": "Бухгалтер"}},
		map[string]any{"contacts": []any{"AAA", "BBB"}},
	}

	got, err := docxgen.RenderSmartTable(table, items)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	want := `<w:tbl>` +
		`<w:tr><w:tc><w:p><w:t>HEADER</w:t></w:p></w:tc></w:tr>` +
		`<w:tr><w:tc><w:p><w:t>{ ` + "`Отдел продаж`" + ` | abbr }</w:t></w:p></w:tc></w:tr>` +
		`<w:tr><w:tc><w:p><w:t>Иванов И.И.</w:t></w:p></w:tc><w:tc><w:p><w:t>Инженер</w:t></w:p></w:tc></w:tr>` +
		`<w:tr><w:tc><w:p><w:t>Петров М.С.</w:t></w:p></w:tc><w:tc><w:p><w:t>Директор</w:t></w:p></w:tc></w:tr>` +
		`<w:tr><w:tc><w:p><w:t>Сидоров Н.Д.</w:t></w:p></w:tc><w:tc><w:p><w:t>Бухгалтер</w:t></w:p></w:tc></w:tr>` +
		`<w:tr><w:tc><w:p><w:t>AAA</w:t></w:p></w:tc><w:tc><w:p><w:t>BBB</w:t></w:p></w:tc></w:tr>` +
		`<w:tr><w:tc><w:p><w:t>FOOTER</w:t></w:p></w:tc></w:tr>` +
		`</w:tbl>`

	compact := func(s string) string {
		return strings.ReplaceAll(strings.ReplaceAll(strings.TrimSpace(s), "\n", ""), " ", "")
	}

	if compact(got) != compact(want) {
		t.Fatalf("mismatch:\n got: %s\nwant: %s", got, want)
	}
}

func TestRenderSmartTable_LeavesUnknownTags(t *testing.T) {
	table := `
<w:tbl>
  <w:tr><w:tc><w:p><w:t>{company}</w:t></w:p></w:tc></w:tr>
  <w:tr><w:tc><w:p><w:t>{fio}</w:t></w:p></w:tc></w:tr>
</w:tbl>`

	items := []any{
		map[string]any{"item": map[string]any{"fio": "Иванов"}},
	}

	got, err := docxgen.RenderSmartTable(table, items)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if !strings.Contains(got, "{company}") {
		t.Fatalf("unknown tag {company} should be preserved: %s", got)
	}
	if !strings.Contains(got, "Иванов") {
		t.Fatalf("fio should be substituted: %s", got)
	}
}

func TestRenderSmartTable_PositionalInsideBackticks(t *testing.T) {
	table := `
<w:tbl>
  <w:tr><w:tc><w:p><w:t>%[1]s</w:t></w:p></w:tc><w:tc><w:p><w:t>%[2]s</w:t></w:p></w:tc></w:tr>
  <w:tr><w:tc><w:p><w:t>{` + "`%[1]s`" + `|abbr}</w:t></w:p></w:tc></w:tr>
</w:tbl>`

	items := []any{
		map[string]any{"b": []any{"AAA", "ООО Техпром, г. Москва"}},
		map[string]any{"a": []any{"BBB"}},
	}

	got, err := docxgen.RenderSmartTable(table, items)
	if err != nil {
		t.Fatalf("RenderSmartTable error: %v", err)
	}

	if !strings.Contains(got, "{{ `BBB` | abbr }}") {
		t.Fatalf("backticked placeholder should be substituted using next slice item: %s", got)
	}

	if !strings.Contains(got, ">AAA<") {
		t.Fatalf("positional row should be substituted: %s", got)
	}

	if !strings.Contains(got, "ООО Техпром, г. Москва") {
		t.Fatalf("positional row should be substituted: %s", got)
	}
}

func TestRenderSmartTable_NoMatchItemIgnored(t *testing.T) {
	table := `
<w:tbl>
  <w:tr><w:tc><w:p><w:t>{fio}</w:t></w:p></w:tc></w:tr>
</w:tbl>`

	items := []any{
		map[string]any{"x": map[string]any{"unknownfield": "???"}},
	}

	got, err := docxgen.RenderSmartTable(table, items)
	if err != nil {
		t.Fatalf("error: %v", err)
	}

	if strings.Contains(got, "???") {
		t.Fatalf("item without tags should be skipped: %s", got)
	}
}
