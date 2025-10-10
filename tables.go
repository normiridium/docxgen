package docxgen

import (
	"fmt"
	"strings"
)

// TableTemplateEngine — генератор таблиц из шаблона DOCX
type TableTemplateEngine struct {
	HeaderPart       string // часть таблицы до шаблонных строк
	RowTemplate      string // шаблон основной строки
	SubRowTemplate   string // шаблон подстроки
	TitleRowTemplate string // шаблон заголовочной строки
	FooterPart       string // часть таблицы после шаблонных строк
	Rows             []string
}

// TableTemplateConfig — конфигурация индексов строк-шаблонов
type TableTemplateConfig struct {
	RowIndex    int // обязательный индекс строки-шаблона
	SubRowIndex int // подстрока (если нет, то -1)
	TitleIndex  int // заголовочная строка (если нет, то -1)
}

// NewTableTemplate — создаёт генератор таблицы по конфигу
func NewTableTemplate(tableXML string, cfg TableTemplateConfig) (*TableTemplateEngine, error) {
	parts := strings.Split(tableXML, TableRowClosingTag)
	if len(parts) == 0 {
		return nil, fmt.Errorf("empty table")
	}

	engine := &TableTemplateEngine{}
	rows := []string{}
	for _, p := range parts {
		if strings.Contains(p, TableRowOpeningTag) {
			rows = append(rows, p+TableRowClosingTag)
		}
	}

	// обязательный RowTemplate
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

// AddRow — добавить обычную строку
func (t *TableTemplateEngine) AddRow(values ...string) {
	row := t.RowTemplate
	for i, v := range values {
		placeholder := fmt.Sprintf("%%%d", i+1)
		row = strings.ReplaceAll(row, placeholder, v)
	}
	t.Rows = append(t.Rows, row)
}

// AddSubRow — добавить подстроку
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

// AddTitleRow — добавить заголовочную строку
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

// Render — собрать итоговую таблицу
func (t *TableTemplateEngine) Render() string {
	return TableOpeningTag +
		t.HeaderPart +
		strings.Join(t.Rows, "") +
		t.FooterPart +
		TableEndingTag
}

// FindTemplateRows — утилита: ищет индексы строк-шаблонов.
// Возвращает rowIdx, subRowIdx, titleIdx.
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
