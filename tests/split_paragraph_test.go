package tests

import (
	"docxgen/metrics"
	"docxgen/tostring"
	"fmt"
	"testing"
)

// локальный мок для FontSet
type mockFontSet struct{}

func (m *mockFontSet) Measure(s string, _ metrics.Style, _ float64) (float64, error) {
	if s == "err" {
		return 0, fmt.Errorf("mock error")
	}
	return float64(len(s)), nil
}

func newMockFontSet() *mockFontSet { return &mockFontSet{} }

func TestSplitParagraphByUnderscore_Simple(t *testing.T) {
	fs := newMockFontSet()
	text := "один два три"
	lines, err := tostring.SplitParagraphByUnderscore(text, fs, metrics.Regular, 12, 20, 20)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 2 {
		t.Errorf("expected 1 line, got %d", len(lines))
	}
	if lines[0] != "один два" {
		t.Errorf("unexpected content: %q", lines[0])
	}
}

func TestSplitParagraphByUnderscore_Wrap(t *testing.T) {
	fs := newMockFontSet()
	text := "один два три"
	lines, err := tostring.SplitParagraphByUnderscore(text, fs, metrics.Regular, 12, 5, 10)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) < 2 {
		t.Errorf("expected wrap into 2+ lines, got %v", lines)
	}
}

func TestSplitParagraphByUnderscore_LongWord(t *testing.T) {
	fs := newMockFontSet()
	text := "суперкалифрагилистик"
	lines, err := tostring.SplitParagraphByUnderscore(text, fs, metrics.Regular, 12, 5, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 1 || lines[0] != text {
		t.Errorf("expected single long word intact, got %v", lines)
	}
}

func TestSplitParagraphByUnderscore_Empty(t *testing.T) {
	fs := newMockFontSet()
	lines, err := tostring.SplitParagraphByUnderscore("", fs, metrics.Regular, 12, 5, 5)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lines) != 0 {
		t.Errorf("expected no lines, got %v", lines)
	}
}

func TestSplitParagraphByUnderscore_ErrorMeasure(t *testing.T) {
	fs := newMockFontSet()
	_, err := tostring.SplitParagraphByUnderscore("err", fs, metrics.Regular, 12, 5, 5)
	if err == nil {
		t.Error("expected error, got nil")
	}
}
