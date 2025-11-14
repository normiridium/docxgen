package docxgen

// Common XML tokens and stubs for DOCX
const (
	// Paragraphs
	ParagraphOpeningTag = "<w:p>"
	ParagraphPartTag    = "w:p>"
	ParagraphClosingTag = "</w:p>"

	// Table
	TableOpeningTag    = "<w:tbl>"
	TablePartTag       = "w:tbl>"
	TableEndingTag     = "</w:tbl>"
	TableRowOpeningTag = "<w:tr>"
	TableRowPartTag    = "w:tr>"
	TableRowClosingTag = "</w:tr>"

	// Document body
	BodyOpeningTag = "<w:body>"
	BodyClosingTag = "</w:body>"

	// Common Closing Marker
	ClosingPartTag = "</"

	// Empty paragraph
	EmptyParagraph = "<w:p></w:p>"

	// Page break
	PageBreak = `<w:p><w:pPr><w:pStyle w:val="Normal"/><w:ind w:hanging="0" w:left="0" w:right="0"/><w:jc w:val="left"/><w:rPr><w:sz w:val="4"/><w:szCs w:val="4"/></w:rPr></w:pPr><w:r><w:rPr><w:sz w:val="4"/><w:szCs w:val="4"/></w:rPr><w:br w:type="page"/></w:r></w:p>`

	// NilParagraph is a minimal paragraph, often needed for table headers
	NilParagraph = `<w:p><w:pPr><w:pStyle w:val="Normal"/><w:spacing w:lineRule="exact" w:line="14" w:before="0" w:after="0"/><w:rPr><w:sz w:val="2"/><w:szCs w:val="2"/></w:rPr></w:pPr><w:r><w:rPr><w:sz w:val="2"/><w:szCs w:val="2"/></w:rPr></w:r></w:p>`

	// Normal paragraph format (with text)
	FormatNormalParagraph = `<w:p><w:pPr><w:pStyle w:val="Normal"/><w:bidi w:val="0"/><w:jc w:val="left"/><w:rPr></w:rPr></w:pPr><w:r><w:rPr></w:rPr><w:t>%s</w:t></w:r></w:p>`

	// Mark to remove wrapping with xml tag
	unwrapOpen  = "<unwrap>"
	unwrapClose = "</unwrap>"
)
