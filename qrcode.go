package docxgen

import (
	"docxgen/modifiers"
	"fmt"
	"strconv"
	"strings"

	"github.com/skip2/go-qrcode"
)

// QrCode — финальная версия
func (d *Docx) QrCode(value string, opts ...string) modifiers.RawXML {
	const emuPerMM = 36000

	if value == "" {
		return ""
	}

	// -------- дефолтные значения, как в примере ----------
	mode := "anchor"
	sizeMM := 32.0
	crop := 4.0
	align := "right"
	valign := "top"
	distT, distB, distL, distR := 0, 0, 0, 0
	hasBorder := false

	// -------- парсим параметры ----------
	for _, token := range opts {
		token = strings.TrimSpace(token)
		switch {
		case token == "anchor" || token == "inline":
			mode = token
		case strings.HasSuffix(token, "%"):
			crop, _ = strconv.ParseFloat(strings.TrimSuffix(token, "%"), 64)
		case strings.Contains(token, "/"):
			parts := strings.Split(token, "/")
			switch len(parts) {
			case 2:
				// top/bottom = parts[0], left/right = parts[1]
				if v, err := strconv.ParseFloat(parts[0], 64); err == nil {
					distT = int(v * emuPerMM)
					distB = distT
				}
				if v, err := strconv.ParseFloat(parts[1], 64); err == nil {
					distL = int(v * emuPerMM)
					distR = distL
				}
			case 3:
				// top, left/right, bottom
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
				// top, right, bottom, left
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
		case token == "left" || token == "center" || token == "right":
			align = token
		case token == "top" || token == "middle" || token == "bottom":
			if token == "middle" {
				token = "center"
			}
			valign = token
		case token == "border":
			hasBorder = true
		default:
			if v, err := strconv.ParseFloat(strings.TrimSuffix(token, "mm"), 64); err == nil {
				sizeMM = v
			}
		}
	}

	// -------- генерируем QR --------
	sizePx := int(sizeMM / 25.4 * 96)
	data, err := qrcode.Encode(value, qrcode.Medium, sizePx)
	if err != nil {
		return modifiers.RawXML(fmt.Sprintf("<w:p><w:t>QR error: %v</w:t></w:p>", err))
	}

	rId, base := d.AddImageRel(data)

	// -------- перевод в EMU --------

	cx := int(sizeMM * emuPerMM)
	cy := cx
	cropVal := int(crop * 1000)

	// -------- crop section --------
	cropXML := ""
	if crop > 0 {
		cropXML = fmt.Sprintf(`<a:srcRect l="%d" t="%d" r="%d" b="%d"/>`, cropVal, cropVal, cropVal, cropVal)
	}

	borderXML := ""
	if hasBorder {
		borderXML = `<a:ln w="12700"><a:solidFill><a:srgbClr val="000000"/></a:solidFill></a:ln>`
	}

	// -------- общий кусок <pic:pic> --------
	pic := fmt.Sprintf(`
<pic:pic xmlns:pic="http://schemas.openxmlformats.org/drawingml/2006/picture">
  <pic:nvPicPr>
    <pic:cNvPr id="1" name="%s"/>
    <pic:cNvPicPr><a:picLocks noChangeAspect="1" noChangeArrowheads="1"/></pic:cNvPicPr>
  </pic:nvPicPr>
  <pic:blipFill>
    <a:blip r:embed="%s" cstate="print"/>
    %s
    <a:stretch><a:fillRect/></a:stretch>
  </pic:blipFill>
  <pic:spPr bwMode="auto">
    <a:xfrm><a:off x="0" y="0"/><a:ext cx="%d" cy="%d"/></a:xfrm>
    <a:prstGeom prst="rect"><a:avLst/></a:prstGeom>
    <a:noFill/>%s
  </pic:spPr>
</pic:pic>`, base, rId, cropXML, cx, cy, borderXML)

	// -------- ветка inline / anchor --------
	var drawing string

	if mode == "inline" {
		drawing = fmt.Sprintf(`
<w:drawing xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <wp:inline distT="0" distB="0" distL="0" distR="0">
    <wp:extent cx="%d" cy="%d"/>
    <wp:effectExtent l="0" t="0" r="0" b="0"/>
    <wp:docPr id="1" name="%s"/>
    <wp:cNvGraphicFramePr>
      <a:graphicFrameLocks xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" noChangeAspect="1"/>
    </wp:cNvGraphicFramePr>
    <a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
      <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">%s</a:graphicData>
    </a:graphic>
  </wp:inline>
</w:drawing>`, cx, cy, base, pic)
	} else { // anchor (по умолчанию)
		drawing = fmt.Sprintf(`
<w:drawing xmlns:r="http://schemas.openxmlformats.org/officeDocument/2006/relationships">
  <wp:anchor behindDoc="0" distT="%d" distB="%d" distL="%d" distR="%d" 
	simplePos="0" locked="0" layoutInCell="0" allowOverlap="1" relativeHeight="2">
	<wp:simplePos x="0" y="0"/>
    <wp:positionH relativeFrom="column"><wp:align>%s</wp:align></wp:positionH>
    <wp:positionV relativeFrom="paragraph"><wp:align>%s</wp:align></wp:positionV>
    <wp:extent cx="%d" cy="%d"/>
    <wp:effectExtent l="0" t="0" r="0" b="0"/>
    <wp:wrapSquare wrapText="bothSides"/>
    <wp:docPr id="1" name="%s"/>
    <wp:cNvGraphicFramePr>
      <a:graphicFrameLocks xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main" noChangeAspect="1"/>
    </wp:cNvGraphicFramePr>
    <a:graphic xmlns:a="http://schemas.openxmlformats.org/drawingml/2006/main">
      <a:graphicData uri="http://schemas.openxmlformats.org/drawingml/2006/picture">%s</a:graphicData>
    </a:graphic>
  </wp:anchor>
</w:drawing>`, distT, distB, distL, distR, align, valign, cx, cy, base, pic)
	}

	// -------- выходим из параграфа  --------
	xml := fmt.Sprintf("</w:t></w:r><w:r>%s</w:r><w:r><w:t>", drawing)

	return modifiers.RawXML(xml)
}
