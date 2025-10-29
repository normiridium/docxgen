package tests

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"docxgen"
)

func TestDocxgenEndToEnd(t *testing.T) {
	// создаём фейковый docx с document.xml
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	w, _ := zw.Create("word/document.xml")
	_, _ = w.Write([]byte(`<w:document><w:body><w:p><w:r><w:t>{fio}</w:t></w:r></w:p></w:body></w:document>`))
	_ = zw.Close()

	tmp := filepath.Join(os.TempDir(), "test.docx")
	if err := os.WriteFile(tmp, buf.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write temp docx: %v", err)
	}
	defer func() {
		err := os.Remove(tmp)
		if err != nil {
			t.Fatalf("failed to remove temp docx: %v", err)
		}
	}()

	// открываем
	doc, err := docxgen.Open(tmp)
	if err != nil {
		t.Fatalf("failed to open temp docx: %v", err)
	}

	// выполняем шаблон
	data := map[string]any{"fio": "Иванов Иван Иванович"}
	if err := doc.ExecuteTemplate(data); err != nil {
		t.Fatalf("failed to execute template: %v", err)
	}

	// сохраняем
	out := tmp + ".out"
	if err := doc.Save(out); err != nil {
		t.Fatalf("failed to save output docx: %v", err)
	}
	defer func(name string) {
		_ = os.Remove(name)
	}(out)

	// повторно открываем
	doc, err = docxgen.Open(out)
	if err != nil {
		t.Fatalf("failed to open output docx: %v", err)
	}

	// читаем контент
	testStr, err := doc.ContentPart("document")
	if err != nil {
		t.Fatalf("failed to extract content from output docx: %v", err)
	}

	// убеждаемся, что подставилось ФИО
	if !strings.Contains(testStr, "Иванов Иван Иванович") {
		t.Errorf("fio not replaced in output document, got content:\n%s", testStr)
	}
}
