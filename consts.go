package docxgen

// Общие XML-маркеры и заготовки для DOCX
const (
	// Параграфы
	ParagraphOpeningTag = "<w:p>"
	ParagraphPartTag    = "w:p>"
	ParagraphClosingTag = "</w:p>"

	// Таблицы
	TableOpeningTag = "<w:tbl>"
	TablePartTag    = "w:tbl>"
	TableEndingTag  = "</w:tbl>"

	TableRowOpeningTag = "<w:tr>"
	TableRowPartTag    = "w:tr>"
	TableRowClosingTag = "</w:tr>"

	// Тело документа
	BodyOpeningTag = "<w:body>"
	BodyClosingTag = "</w:body>"

	// Общий закрывающий маркер
	ClosingPartTag = "</"

	// Перенос строки внутри параграфа
	NewLineInText = "</w:t><w:br/><w:t>"

	// Пустой параграф
	EmptyParagraph = "<w:p></w:p>"

	// Разрыв страницы
	PageBreak = `<w:p><w:pPr><w:pStyle w:val="Normal"/><w:ind w:hanging="0" w:left="0" w:right="0"/><w:jc w:val="left"/><w:rPr><w:sz w:val="4"/><w:szCs w:val="4"/></w:rPr></w:pPr><w:r><w:rPr><w:sz w:val="4"/><w:szCs w:val="4"/></w:rPr><w:br w:type="page"/></w:r></w:p>`

	// NilParagraph — минимальный параграф, часто нужен для шапок таблиц
	NilParagraph = `<w:p><w:pPr><w:pStyle w:val="Normal"/><w:spacing w:lineRule="exact" w:line="14" w:before="0" w:after="0"/><w:rPr><w:sz w:val="2"/><w:szCs w:val="2"/></w:rPr></w:pPr><w:r><w:rPr><w:sz w:val="2"/><w:szCs w:val="2"/></w:rPr></w:r></w:p>`

	// Формат нормального параграфа (с текстом)
	FormatNormalParagraph = `<w:p><w:pPr><w:pStyle w:val="Normal"/><w:bidi w:val="0"/><w:jc w:val="left"/><w:rPr></w:rPr></w:pPr><w:r><w:rPr></w:rPr><w:t>%s</w:t></w:r></w:p>`

	// Пометка к снятию оборачивания xml-тегом
	unwrapOpen  = "<unwrap>"
	unwrapClose = "</unwrap>"
)
