package docxgen

import (
	"bytes"
	"docxgen/modifiers"
	"fmt"
	"image"
	"image/png"
	"strconv"
	"strings"

	"github.com/boombuler/barcode"
	"github.com/boombuler/barcode/code128"
	"github.com/boombuler/barcode/ean"
)

// Barcode - Inserts a barcode (Code128, EAN13) into a document.
// Supports crop (%), margins (x/y), inline/anchor, and relative sizes (% of page).
func (d *Docx) Barcode(value string, opts ...string) modifiers.RawXML {
	if value == "" {
		return ""
	}

	const emuPerMM = 36000

	// ---------- Default parameters ----------
	codeType := "code128"
	mode := "anchor"
	align := "right"
	valign := "top"
	sizeWMM := 40.0
	sizeHMM := 0.0 // if 0, count 1:3
	crop := 0.0
	hasBorder := false
	distT, distB, distL, distR := 0, 0, 0, 0

	// ---------- Page Dimensions (for % Calculations) ----------
	pageW, pageH := d.GetPageSizeEMU()

	// ---------- Parsing options ----------
	for _, token := range opts {
		token = strings.TrimSpace(token)
		switch {
		case token == "anchor" || token == "inline":
			mode = token

		case strings.EqualFold(token, "left"),
			strings.EqualFold(token, "center"),
			strings.EqualFold(token, "right"):
			align = token

		case strings.EqualFold(token, "top"),
			strings.EqualFold(token, "middle"),
			strings.EqualFold(token, "bottom"):
			if token == "middle" {
				token = "center"
			}
			valign = token

		case strings.HasSuffix(token, "%"):
			// crop or relative dimensions
			if strings.Contains(token, "*") {
				break
			}
			if v, err := strconv.ParseFloat(strings.TrimSuffix(token, "%"), 64); err == nil {
				crop = v
			}

		case strings.Contains(token, "/"): // padding
			parts := strings.Split(token, "/")
			switch len(parts) {
			case 2:
				if v, err := strconv.ParseFloat(parts[0], 64); err == nil {
					distT = int(v * emuPerMM)
					distB = distT
				}
				if v, err := strconv.ParseFloat(parts[1], 64); err == nil {
					distL = int(v * emuPerMM)
					distR = distL
				}
			case 3:
				if v, err := strconv.ParseFloat(parts[0], 64); err == nil {
					distT = int(v * emuPerMM)
				}
				if v, err := strconv.ParseFloat(parts[1], 64); err == nil {
					distL = int(v * emuPerMM)
					distR = distL
				}
				if v, err := strconv.ParseFloat(parts[2], 64); err == nil {
					distB = int(v * emuPerMM)
				}
			case 4:
				if v, err := strconv.ParseFloat(parts[0], 64); err == nil {
					distT = int(v * emuPerMM)
				}
				if v, err := strconv.ParseFloat(parts[1], 64); err == nil {
					distR = int(v * emuPerMM)
				}
				if v, err := strconv.ParseFloat(parts[2], 64); err == nil {
					distB = int(v * emuPerMM)
				}
				if v, err := strconv.ParseFloat(parts[3], 64); err == nil {
					distL = int(v * emuPerMM)
				}
			}

		case strings.HasSuffix(token, "mm"):
			// Dimensions (possibly A*B)
			if strings.Contains(token, "*") {
				parts := strings.Split(token, "*")
				if len(parts) == 2 {
					sizeWMM = parseMMorPercent(parts[0], pageW)
					sizeHMM = parseMMorPercent(parts[1], pageH)
				}
			} else if v, err := strconv.ParseFloat(strings.TrimSuffix(token, "mm"), 64); err == nil {
				sizeWMM = v
			}

		case strings.Contains(token, "*") && (strings.HasSuffix(token, "%")):
			// Option with percentages (e.g. 80%*10mm)
			parts := strings.Split(token, "*")
			if len(parts) == 2 {
				sizeWMM = parseMMorPercent(parts[0], pageW)
				sizeHMM = parseMMorPercent(parts[1], pageH)
			}

		case token == "border":
			hasBorder = true

		case token != "":
			codeType = strings.ToLower(token)
		}
	}

	// ---------- Generating an image ----------
	var img barcode.Barcode
	var err error
	switch codeType {
	case "ean13":
		img, err = ean.Encode(value)
	default:
		img, err = code128.Encode(value)
	}
	if err != nil {
		return modifiers.RawXML(fmt.Sprintf("<w:p><w:t>barcode error: %v</w:t></w:p>", err))
	}

	// ---------- scalable ----------
	if sizeHMM <= 0 {
		sizeHMM = sizeWMM / 3
		img, _ = barcode.Scale(img, int(sizeWMM*12), int(sizeHMM*12))
	} else {
		// if it is set explicitly, leave the original barcode,
		// to maintain clarity and not break the aspect ratio
		img, _ = barcode.Scale(img, img.Bounds().Dx(), img.Bounds().Dy())
	}
	buf, _ := encodePNG(img)
	rId, base := d.AddImageRel(buf)

	// ---------- XML ----------
	cx := int(sizeWMM * emuPerMM)
	cy := int(sizeHMM * emuPerMM)
	cropVal := int(crop * 1000)

	cropXML := ""
	if crop > 0 {
		cropXML = fmt.Sprintf(`<a:srcRect l="%d" t="%d" r="%d" b="%d"/>`, cropVal, cropVal, cropVal, cropVal)
	}

	borderXML := ""
	if hasBorder {
		borderXML = `<a:ln w="12700"><a:solidFill><a:srgbClr val="000000"/></a:solidFill></a:ln>`
	}

	pic := fmt.Sprintf(`
<pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
  <pic:nvPicPr><pic:cNvPr id="1" name="%s"/><pic:cNvPicPr/></pic:nvPicPr>
  <pic:blipFill><a:blip r:embed="%s" cstate="print"/>%s<a:stretch><a:fillRect/></a:stretch></pic:blipFill>
  <pic:spPr><a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm>
  <a:prstGeom prst="rect"><a:avLst/></a:prstGeom><a:noFill/>%s</pic:spPr>
</pic:pic>`, base, rId, cropXML, cx, cy, borderXML)

	var xml string
	if mode == "inline" {
		xml = fmt.Sprintf(`
<w:drawing xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <wp:inline distT="0" distB="0" distL="0" distR="0">
    <wp:extent cx="%d" cy="%d"/>
    <wp:docPr id="1" name="%s"/>
    <a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
      <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">%s</a:graphicData>
    </a:graphic>
  </wp:inline>
</w:drawing>`, cx, cy, base, pic)
	} else {
		xml = fmt.Sprintf(`
<w:drawing>
  <wp:anchor behindDoc="0" distT="%d" distB="%d" distL="%d" distR="%d"
    simplePos="0" locked="0" layoutInCell="0" allowOverlap="1" relativeHeight="2">
    <wp:simplePos x="0" y="0"/>
    <wp:positionH relativeFrom="column"><wp:align>%s</wp:align></wp:positionH>
    <wp:positionV relativeFrom="paragraph"><wp:align>%s</wp:align></wp:positionV>
    <wp:extent cx="%d" cy="%d"/>
    <wp:wrapSquare wrapText="bothSides"/>
    <wp:docPr id="1" name="%s"/>
    <a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
      <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">%s</a:graphicData>
    </a:graphic>
  </wp:anchor>
</w:drawing>`, distT, distB, distL, distR, align, valign, cx, cy, base, pic)
	}

	return modifiers.RawXML("</w:t></w:r><w:r>" + xml + "</w:r><w:r><w:t>")
}

// parseMMorPercent — parses a string like "40mm" or "80%" in millimeters,
// using page sizes in the EMU to calculate percentages.
func parseMMorPercent(token string, pageSizeEMU int) float64 {
	token = strings.TrimSpace(token)
	switch {
	case strings.HasSuffix(token, "mm"):
		v, _ := strconv.ParseFloat(strings.TrimSuffix(token, "mm"), 64)
		return v
	case strings.HasSuffix(token, "%"):
		v, _ := strconv.ParseFloat(strings.TrimSuffix(token, "%"), 64)
		// EMU → mm (1 mm = 36000 EMU)
		pageMM := float64(pageSizeEMU) / 36000
		return pageMM * v / 100
	default:
		return 0
	}
}

// GetPageSizeEMU — gets page sizes from document.xml in EMU.
func (d *Docx) GetPageSizeEMU() (width, height int) {
	data, ok := d.files["word/document.xml"]
	if !ok {
		// A4 Default: 210×297mm
		return 210 * 36000, 297 * 36000
	}
	str := string(data)
	w := extractAttrInt(str, `w:pgSz`, `w:w`)
	h := extractAttrInt(str, `w:pgSz`, `w:h`)
	if w == 0 || h == 0 {
		return 210 * 36000, 297 * 36000
	}
	// Values in twips (1/20 pt), 1 pt = 12700 EMU, 1 twip = 635 EMU
	return w * 635, h * 635
}

func extractAttrInt(xml, tag, attr string) int {
	start := strings.Index(xml, "<"+tag)
	if start == -1 {
		return 0
	}
	part := xml[start:]
	attrStart := strings.Index(part, attr+`="`)
	if attrStart == -1 {
		return 0
	}
	attrStart += len(attr) + 2
	end := strings.Index(part[attrStart:], `"`)
	val, _ := strconv.Atoi(part[attrStart : attrStart+end])
	return val
}

// encodePNG encodes image.Image to PNG and returns []byte.
func encodePNG(img image.Image) ([]byte, error) {
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
