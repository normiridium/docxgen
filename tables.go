package docxgen

import (
	"fmt"
	"strings"
)

// TableTemplateEngine - Table generator from the DOCX template
type TableTemplateEngine struct {
	HeaderPart       string // part of the table before template strings
	RowTemplate      string // main string template
	SubRowTemplate   string // substring template
	TitleRowTemplate string // header string template
	FooterPart       string // part of the table after template strings
	Rows             []string
}

// TableTemplateConfig - Template string index configuration
type TableTemplateConfig struct {
	RowIndex    int // required index of the template string
	SubRowIndex int // substring (if not, -1)
	TitleIndex  int // header string (if not, -1)
}

// NewTableTemplate — creates a table generator based on the config
func NewTableTemplate(tableXML string, cfg TableTemplateConfig) (*TableTemplateEngine, error) {
	parts := strings.Split(tableXML, TableRowClosingTag)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty table")
	}

	engine := &TableTemplateEngine{}
	var rows []string
	for _, p := range parts {
		if strings.Contains(p, TableRowOpeningTag) {
			rows = append(rows, p+TableRowClosingTag)
		}
	}

	// required RowTemplate
	if cfg.RowIndex < 0 || cfg.RowIndex >= len(rows) {
		return nil, fmt.Errorf("row index %d out of range", cfg.RowIndex)
	}
	engine.RowTemplate = rows[cfg.RowIndex]
	engine.HeaderPart = strings.Join(rows[:cfg.RowIndex], "")
	if cfg.RowIndex+1 < len(rows) {
		engine.FooterPart = strings.Join(rows[cfg.RowIndex+1:], "")
	}

	// SubRowTemplate
	if cfg.SubRowIndex >= 0 && cfg.SubRowIndex < len(rows) {
		engine.SubRowTemplate = rows[cfg.SubRowIndex]
	}

	// TitleRowTemplate
	if cfg.TitleIndex >= 0 && cfg.TitleIndex < len(rows) {
		engine.TitleRowTemplate = rows[cfg.TitleIndex]
	}

	return engine, nil
}

// AddRow — add a regular row
func (t *TableTemplateEngine) AddRow(values ...string) {
	row := t.RowTemplate
	for i, v := range values {
		placeholder := fmt.Sprintf("%%%d", i+1)
		row = strings.ReplaceAll(row, placeholder, v)
	}
	t.Rows = append(t.Rows, row)
}

// AddSubRow — Add substring
func (t *TableTemplateEngine) AddSubRow(values ...string) {
	if t.SubRowTemplate == "" {
		return
	}
	row := t.SubRowTemplate
	for i, v := range values {
		ph := fmt.Sprintf("%%%d", i+1)
		row = strings.ReplaceAll(row, ph, v)
	}
	t.Rows = append(t.Rows, row)
}

// AddTitleRow — Add a header bar
func (t *TableTemplateEngine) AddTitleRow(values ...string) {
	if t.TitleRowTemplate == "" {
		return
	}
	row := t.TitleRowTemplate
	for i, v := range values {
		ph := fmt.Sprintf("%%%d", i+1)
		row = strings.ReplaceAll(row, ph, v)
	}
	t.Rows = append(t.Rows, row)
}

// Render — collect the final table
func (t *TableTemplateEngine) Render() string {
	return TableOpeningTag +
		t.HeaderPart +
		strings.Join(t.Rows, "") +
		t.FooterPart +
		TableEndingTag
}

// FindTemplateRows is a utility that searches for index scripts of template rows.
// Returns rowIdx, subRowIdx, titleIdx.
func FindTemplateRows(tableXML string) (int, int, int, error) {
	rows := strings.Split(tableXML, TableRowClosingTag)
	if len(rows) == 0 {
		return -1, -1, -1, fmt.Errorf("empty table")
	}

	var rowIdx, subRowIdx, titleIdx = -1, -1, -1

	for i, r := range rows {
		if strings.Contains(r, "%1") {
			if rowIdx == -1 {
				rowIdx = i
			} else if subRowIdx == -1 {
				subRowIdx = i
			}
		}
		if strings.Contains(strings.ToLower(r), "итого") {
			titleIdx = i
		}
	}

	return rowIdx, subRowIdx, titleIdx, nil
}
