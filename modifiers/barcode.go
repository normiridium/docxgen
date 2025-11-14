package modifiers

var BarCodeFunc func(string, ...string) RawXML

// BarCode - Inserts a barcode at a preset value directly into the document.
//
// Example of use:
//
// {product.code|barcode:`code128`:`anchor`:`right`:`top`:`50mm*15mm`:`10%`:`2/5`:`border`}
//
// Format:
//
// {value|barcode:[type]:[mode]:[align]:[valign]:[size]:[crop%]:[margins]:[border]}
//
// Parameters (all optional, the order is not important):
//
//   - type—barcode type.
//     Supported: "code128" (default), "ean13".
//     If not specified, "code128" is used.
//
//   - mode — "anchor" (default) or "inline".
//     "anchor" — floating placement relative to the text (like an image),
//     "inline" is an inline line element.
//
//   - align — "left", "center", "right".
//     Horizontal alignment for anchor mode (default is "right").
//
//   - valign — "top", "middle", "bottom".
//     Vertical alignment (default "top").
//     "middle" is automatically converted to "center".
//
// - size — barcode dimensions:
//
// - <N>"mm" – width (height is calculated as 1/3 of the width, aspect ratio 3:1);
//
// - "<W>mm*<H>mm" — both sides are explicitly specified;
//
// - "<N>%" — width as a percentage of the page width;
//
// - <W>"%*<H>mm" or vice versa - combined sizes (percent + millimeters).
//
//   - <N>% – crop (trimming the white margins around the barcode).
//     The value is set by the number of percentages (0 by default).
//
//   - margins — indents from the text (for anchor mode), millimeters.
//     Formats:
//     "5/5" — top/bottom = 5 mm, left/right = 5 mm;
//     "5/3/5/3" - top/right/bottom/left separately;
//     "5/3/7" - top, side, bottom.
//
// - border — a flag that adds a thin black border (≈ 0.5 pt) around the barcode.
//
// Features:
//
// - Barcode scales proportionally or to specified sizes.
//   - Dimensions can be set as absolute (mm) or relative (% of page).
//   - "Inline" and "anchor" modes are supported, similar to a QR code.
//   - Cropping of white margins and setting of external paddings are supported.
//   - When used in pipelines (e.g. {code|compact|barcode})
//     The barcode gets a value after all the previous filters.
//
// Returns:
//
// An XML fragment <w:drawing> with an image of the barcode.
func BarCode(value string, opts ...string) RawXML {
	if BarCodeFunc == nil {
		return ""
	}
	return BarCodeFunc(value, opts...)
}
