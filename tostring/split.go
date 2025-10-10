package tostring

import (
	"docxgen/metrics"
	"fmt"
	"strings"
)

// widthByUnderscore считает ширину строки из n подчёркиваний
func widthByUnderscore(fs metrics.FontMeasurer, style metrics.Style, sizePt float64, n int) (float64, error) {
	unders := strings.Repeat("_", n)
	return fs.Measure(unders, style, sizePt)
}

// SplitParagraphByUnderscore разбивает текст по «линейке» из подчёркиваний
// first – сколько "_" помещается в первой строке
// other – сколько "_" помещается в остальных
func SplitParagraphByUnderscore(
	text string,
	fs metrics.FontMeasurer, // вместо *metrics.FontSet
	style metrics.Style,
	sizePt float64,
	first, other int,
) ([]string, error) {
	firstWidth, err := widthByUnderscore(fs, style, sizePt, first)
	if err != nil {
		return nil, fmt.Errorf("firstWidth: %w", err)
	}
	otherWidth, err := widthByUnderscore(fs, style, sizePt, other)
	if err != nil {
		return nil, fmt.Errorf("otherWidth: %w", err)
	}

	words := strings.Fields(text)
	var lines []string
	var curLine strings.Builder
	curWidth := 0.0
	limit := firstWidth

	for _, w := range words {
		wordWidth, err := fs.Measure(w, style, sizePt)
		if err != nil {
			return nil, fmt.Errorf("measure word: %w", err)
		}
		spaceWidth, err := fs.Measure(" ", style, sizePt)
		if err != nil {
			return nil, fmt.Errorf("measure space: %w", err)
		}

		add := wordWidth
		if curLine.Len() > 0 {
			add += spaceWidth
		}

		if curWidth+add > limit && curLine.Len() > 0 {
			// перенос строки
			lines = append(lines, curLine.String())
			curLine.Reset()
			curLine.WriteString(w)
			curWidth = wordWidth
			limit = otherWidth
		} else {
			if curLine.Len() > 0 {
				curLine.WriteString(" ")
				curWidth += spaceWidth
			}
			curLine.WriteString(w)
			curWidth += wordWidth
		}
	}
	if curLine.Len() > 0 {
		lines = append(lines, curLine.String())
	}
	return lines, nil
}
