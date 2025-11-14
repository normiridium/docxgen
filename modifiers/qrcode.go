package modifiers

var QrCodeFunc func(string, ...string) RawXML

// QrCode — inserts a QR code at a specified value directly into the document.
//
// Example of use:
//
// {project.code|qrcode:`right`:`top`:`8%`:`5/5`:`border`}
//
// Format:
//
// {value|qrcode:[mode]:[align]:[valign]:[crop%]:[margins]:[border]}
//
// Parameters (all optional, the order is not important):
//
//   - mode — "anchor" (default) or "inline"
//     Insertion mode: floating (anchor) or embedded in text (inline).
//
//   - align — "left", "center", "right"
//     Horizontal alignment for anchor mode (default is "right").
//
//   - valign — "top", "middle", "bottom"
//     Vertical alignment (default "top").
//     "middle" is a synonym for "center".
//
// - <N>mm—QR code size in millimeters (32 mm by default).
//
// - <N>% – crop (crop the white margins around the QR code), 4% by default.
//
//   - margins — indents from the text, in millimeters.
//     Formats:
//     "5/5" — top/bottom = 5 mm, left/right = 5 mm;
//     "5/3/5/3" - top/right/bottom/left separately;
//     "5/3/7" - top, side, bottom.
//
// - border — a flag that adds a thin black border (≈ 0.5 pt) around the QR code.
//
// Returns:
//
// Inserted XML fragment <w:drawing> with the generated QR image.
//
// Compatible with Microsoft Word, LibreOffice, OnlyOffice.
func QrCode(value string, opts ...string) RawXML {
	if QrCodeFunc == nil {
		return ""
	}
	return QrCodeFunc(value, opts...)
}
