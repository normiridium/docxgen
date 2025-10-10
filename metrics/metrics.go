package metrics

import (
	"fmt"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
	"io/ioutil"
)

type Style int

const (
	Regular Style = iota
	Bold
	Italic
	BoldItalic
)

// FontSet хранит набор TTF Times New Roman (обычный, жирный, курсив, жирный курсив).
// Поля экспортируемые, чтобы их можно было использовать за пределами пакета.
type FontSet struct {
	Regular    *sfnt.Font
	Bold       *sfnt.Font
	Italic     *sfnt.Font
	BoldItalic *sfnt.Font
}

// FontMeasurer — общий интерфейс для всего, что умеет измерять строки.
type FontMeasurer interface {
	Measure(s string, style Style, sizePt float64) (float64, error)
}

// LoadFonts загружает шрифты из файловой системы.
func LoadFonts(pathRegular, pathBold, pathItalic, pathBoldItalic string) (*FontSet, error) {
	paths := []string{pathRegular, pathBold, pathItalic, pathBoldItalic}
	var fonts [4]*sfnt.Font

	for i, path := range paths {
		data, err := ioutil.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("read font %s: %w", path, err)
		}
		font, err := sfnt.Parse(data)
		if err != nil {
			return nil, fmt.Errorf("parse font %s: %w", path, err)
		}
		fonts[i] = font
	}

	return &FontSet{
		Regular:    fonts[0],
		Bold:       fonts[1],
		Italic:     fonts[2],
		BoldItalic: fonts[3],
	}, nil
}

// Measure возвращает ширину строки в "пунктах" (pt) для заданного размера и стиля
func (fs *FontSet) Measure(text string, style Style, sizePt float64) (float64, error) {
	var fontFace *sfnt.Font
	switch style {
	case Regular:
		fontFace = fs.Regular
	case Bold:
		fontFace = fs.Bold
	case Italic:
		fontFace = fs.Italic
	case BoldItalic:
		fontFace = fs.BoldItalic
	default:
		return 0, fmt.Errorf("unknown style")
	}

	unitsPerEm := fontFace.UnitsPerEm()

	// предполагаем DPI = 72 → 1 pt = 1 px
	// если нужен другой DPI, сюда можно добавить параметр
	ppem := fixed.Int26_6(sizePt * 64)

	buf := &sfnt.Buffer{}
	total := 0.0
	for _, r := range text {
		gid, err := fontFace.GlyphIndex(buf, r)
		if err != nil {
			return 0, fmt.Errorf("glyphIndex: %w", err)
		}
		adv, err := fontFace.GlyphAdvance(buf, gid, ppem, font.HintingNone)
		if err != nil {
			return 0, fmt.Errorf("glyphAdvance: %w", err)
		}
		total += float64(adv) / 64.0 // adv в fixed.Int26_6 → делим на 64
	}

	// пересчёт из font units в pt (масштабируем по UnitsPerEm)
	return total * (sizePt / float64(unitsPerEm)), nil
}
