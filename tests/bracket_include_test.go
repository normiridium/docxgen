package tests

import (
	"docxgen"
	"testing"
)

func TestParseBracketIncludeTag_OK(t *testing.T) {
	tests := map[string]struct {
		tag      string
		file     string
		fragment string
		index    int
	}{
		"[include/a.docx]":         {"include/a.docx", "a.docx", "body", 1},
		"[include/a.docx/body]":    {"include/a.docx/body", "a.docx", "body", 1},
		"[include/a.docx/table]":   {"include/a.docx/table", "a.docx", "table", 1},
		"[include/a.docx/table/3]": {"include/a.docx/table/3", "a.docx", "table", 3},
		"[include/x/y/z.docx/p/2]": {"include/x/y/z.docx/p/2", "x/y/z.docx", "p", 2},
	}

	for raw, want := range tests {
		spec, err := docxgen.ParseBracketIncludeTag(raw)
		if err != nil {
			t.Fatalf("unexpected error for %s: %v", raw, err)
		}
		if spec.File != want.file || spec.Fragment != want.fragment || spec.Index != want.index {
			t.Errorf("parse %s mismatch: %+v", raw, spec)
		}
	}
}

func TestParseBracketIncludeTag_Fail(t *testing.T) {
	bad := []string{
		"[include]", "[include/]", "[include/noext]", "[include/a.docx/zzz]",
		"[include/a.docx/table/0]", "[include/a.docx/p/-1]", "random",
	}
	for _, raw := range bad {
		if _, err := docxgen.ParseBracketIncludeTag(raw); err == nil {
			t.Errorf("expected error for %s", raw)
		}
	}
}
