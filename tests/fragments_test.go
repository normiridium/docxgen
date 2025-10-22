package tests

import (
	"docxgen"
	"strings"
	"testing"
)

const testDoc = `
<w:document>
<w:body>
<w:p>AAA</w:p>
<w:tbl>TABLE1</w:tbl>
<w:p>BBB</w:p>
<w:tbl>TABLE2</w:tbl>
<w:p>CCC</w:p>
</w:body>
</w:document>
`

func TestGetBodyFragment(t *testing.T) {
	body, err := docxgen.GetBodyFragment(testDoc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !strings.Contains(body, "AAA") || !strings.Contains(body, "CCC") {
		t.Errorf("body fragment seems broken: %s", body)
	}
}

func TestGetBodyFragment_Fail(t *testing.T) {
	_, err := docxgen.GetBodyFragment("<w:document><w:bod></w:bod></w:document>")
	if err == nil {
		t.Errorf("expected error for invalid body, got nil")
	}
}

func TestGetTableN(t *testing.T) {
	tbl1, err := docxgen.GetTableN(testDoc, 1)
	if err != nil {
		t.Fatalf("unexpected error for table 1: %v", err)
	}
	if !strings.Contains(tbl1, "TABLE1") {
		t.Errorf("table1 mismatch: %s", tbl1)
	}

	tbl2, err := docxgen.GetTableN(testDoc, 2)
	if err != nil {
		t.Fatalf("unexpected error for table 2: %v", err)
	}
	if !strings.Contains(tbl2, "TABLE2") {
		t.Errorf("table2 mismatch: %s", tbl2)
	}
}

func TestGetTableN_Fail(t *testing.T) {
	_, err := docxgen.GetTableN(testDoc, 3)
	if err == nil {
		t.Errorf("expected error for missing table, got nil")
	}
}

func TestGetParagraphN(t *testing.T) {
	p1, err := docxgen.GetParagraphN(testDoc, 1)
	if err != nil {
		t.Fatalf("unexpected para 1 error: %v", err)
	}
	if !strings.Contains(p1, "AAA") {
		t.Errorf("para1 mismatch: %s", p1)
	}

	p3, err := docxgen.GetParagraphN(testDoc, 3)
	if err != nil {
		t.Fatalf("unexpected para 3 error: %v", err)
	}
	if !strings.Contains(p3, "CCC") {
		t.Errorf("para3 mismatch: %s", p3)
	}
}

func TestGetParagraphN_Fail(t *testing.T) {
	_, err := docxgen.GetParagraphN(testDoc, 4)
	if err == nil {
		t.Errorf("expected error for missing parag, got nil")
	}
}
