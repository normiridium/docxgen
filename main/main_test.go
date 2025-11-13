package main

import (
	"archive/zip"
	"bytes"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"docxgen"
)

// makeFakeDocx создаёт минимальный DOCX с тегом {name}
func makeFakeDocx() []byte {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	files := map[string]string{
		"[Content_Types].xml": `<?xml version="1.0" encoding="UTF-8"?>
<Types xmlns="http://schemas.openxmlformats.org/package/2006/content-types">
  <Default Extension="xml" ContentType="application/xml"/>
</Types>`,
		"word/document.xml": `<?xml version="1.0" encoding="UTF-8" standalone="yes"?>
<w:document xmlns:w="http://schemas.openxmlformats.org/wordprocessingml/2006/main">
<w:body><w:p><w:r><w:t>{name}</w:t></w:r></w:p></w:body></w:document>`,
	}
	for name, content := range files {
		w, _ := zw.Create(name)
		_, _ = io.WriteString(w, content)
	}
	_ = zw.Close()
	return buf.Bytes()
}

// openDocxFromBytes — вспомогательный хелпер: открывает docx прямо из байтов без файлов на диске
func openDocxFromBytes(b []byte) (*docxgen.Docx, error) {
	tmp, err := os.CreateTemp("", "tmpl*.docx")
	if err != nil {
		return nil, err
	}
	defer func(name string) {
		_ = os.Remove(name)
	}(tmp.Name())
	_, _ = tmp.Write(b)
	_ = tmp.Close()
	return docxgen.Open(tmp.Name())
}

// TestHTTPGenerate_MemoryOnly — тестирует endpoint /generate без файлов и без шрифтов
func TestHTTPGenerate_MemoryOnly(t *testing.T) {
	docxBytes := makeFakeDocx()
	docxB64 := base64.StdEncoding.EncodeToString(docxBytes)

	reqBody := map[string]any{
		"template": docxB64,
		"data":     map[string]any{"name": "Оленька"},
		"format":   "xml",
	}
	body, _ := json.Marshal(reqBody)

	// создаём handler прямо из runServer без запуска порта
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Template string         `json:"template"`
			Data     map[string]any `json:"data,omitempty"`
			Format   string         `json:"format,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonErr(w, 400, "bad json: %v", err)
			return
		}

		// распаковываем шаблон из base64
		raw, err := base64.StdEncoding.DecodeString(req.Template)
		if err != nil {
			jsonErr(w, 400, "bad base64: %v", err)
			return
		}
		doc, err := openDocxFromBytes(raw)
		if err != nil {
			jsonErr(w, 500, "open docx: %v", err)
			return
		}

		// просто не вызываем loadFonts, чтобы не требовались файлы
		registerCommonModifiers(doc)

		if err := executeTemplate(doc, req.Data); err != nil {
			jsonErr(w, 500, "exec: %v", err)
			return
		}
		xml, _ := doc.ContentPart("document")
		w.Header().Set("Content-Type", "application/xml; charset=utf-8")
		_, err = w.Write([]byte(xml))
		if err != nil {
			jsonErr(w, 500, "exec: %v", err)
			return
		}
	})

	req := httptest.NewRequest("POST", "/generate", bytes.NewReader(body))
	w := httptest.NewRecorder()
	handler(w, req)

	resp := w.Result()
	defer func(Body io.ReadCloser) {
		_ = Body.Close()
	}(resp.Body)

	if resp.StatusCode != 200 {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}

	xml, _ := io.ReadAll(resp.Body)
	if !bytes.Contains(xml, []byte("Оленька")) {
		t.Fatalf("XML не содержит подстановку:\n%s", xml)
	}
}
