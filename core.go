package docxgen

import (
	"archive/zip"
	"bytes"
	"crypto/sha1"
	"docxgen/metrics"
	"docxgen/modifiers"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"
)

// sharedMedia — потокобезопасное хранилище медиафайлов (png, jpg и т.п.),
// используемое всеми экземплярами Docx при генерации документов.
type sharedMedia struct {
	mu    sync.Mutex
	files map[string][]byte
}

// глобальный экземпляр
var globalMedia = &sharedMedia{
	files: make(map[string][]byte),
}

// AddAll — добавляет все файлы из другой карты в общий пул.
func (m *sharedMedia) AddAll(from map[string][]byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range from {
		m.files[k] = v
	}
}

// ForEach — выполняет действие для каждого файла в пуле.
func (m *sharedMedia) ForEach(fn func(name string, data []byte)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for k, v := range m.files {
		fn(k, v)
	}
}

// Docx представляет собой распакованный DOCX-документ
// и предоставляет API для чтения, модификации и повторной упаковки.
//
// Поля структуры:
//   - files — все файлы из архива (xml, styles, media и т.п.);
//   - localMedia — вложения, добавленные внутри текущего экземпляра;
//   - sourcePath — исходный путь к шаблону;
//   - extraFuncs — дополнительные зарегистрированные модификаторы;
//   - fonts — набор шрифтов (для p_split и подобных операций);
//   - activePart — текущий редактируемый раздел документа ("document", "header1", "footer1" и т.д.).
//     ⚠️ Не потокобезопасно — нельзя менять в нескольких горутинах одновременно.
type Docx struct {
	files      map[string][]byte
	localMedia map[string][]byte
	sourcePath string
	extraFuncs map[string]modifiers.ModifierMeta
	fonts      *metrics.FontSet
	activePart string
}

//
// ──────────────────────────── ОСНОВНЫЕ ОПЕРАЦИИ ────────────────────────────
//

// Open — открывает DOCX-файл, распаковывает его и подготавливает структуру.
func Open(path string) (*Docx, error) {
	reader, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open docx: %w", err)
	}
	defer func(reader *zip.ReadCloser) {
		_ = reader.Close()
	}(reader)

	files := make(map[string][]byte)
	for _, file := range reader.File {
		rc, err := file.Open()
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file.Name, err)
		}
		data, err := io.ReadAll(rc)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", file.Name, err)
		}

		err = rc.Close()
		if err != nil {
			return nil, fmt.Errorf("close %s: %w", file.Name, err)
		}

		files[file.Name] = data
	}

	doc := &Docx{
		files:      files,
		sourcePath: path,
		localMedia: make(map[string][]byte),
	}

	// Восстанавливаем сломанные теги, чтобы шаблон можно было интерпретировать корректно.
	body, err := doc.ContentPart("document")
	if err != nil {
		return nil, err
	}

	body, err = doc.RepairTags(body)
	if err != nil {
		return nil, fmt.Errorf("repair tags: %w", err)
	}

	body = doc.ProcessUnWrapParagraphTags(body)
	doc.UpdateContentPart("document", body)

	return doc, nil
}

// Save — записывает все файлы документа обратно в DOCX-архив.
func (d *Docx) Save(path string) error {
	buffer := new(bytes.Buffer)
	writer := zip.NewWriter(buffer)

	// 1. Объединяем все медиафайлы в единую карту
	// mediaByPart — хранит файлы для разных частей документа
	mediaByPart := map[string][]string{}
	globalMedia.ForEach(func(filename string, data []byte) {
		d.files[filename] = data

		mediaName := strings.TrimPrefix(filename, "word/media/")
		// имя секции кодируем в имени файла, например:
		//   word/media/document_abc.png
		//   word/media/footer1_xyz.png
		//   word/media/header2_zzz.png
		parts := strings.SplitN(mediaName, "_", 2)
		part := "document" // по умолчанию

		if len(parts) > 1 {
			switch {
			case strings.HasPrefix(parts[0], "header"):
				part = parts[0]
			case strings.HasPrefix(parts[0], "footer"):
				part = parts[0]
			}
		}

		mediaByPart[part] = append(mediaByPart[part], mediaName)
	})

	// 2. Обновляем rels и [Content_Types].xml
	for part, names := range mediaByPart {
		d.updateMediaRelationships(part, names)
	}

	// 3. Создаём ZIP-архив
	for name, data := range d.files {
		name = strings.TrimPrefix(name, "/")
		name = strings.ReplaceAll(name, "\\", "/")
		if strings.TrimSpace(name) == "" {
			continue
		}

		header := &zip.FileHeader{
			Name:     name,
			Method:   zip.Deflate,
			Modified: time.Now().UTC(),
		}
		writerFile, err := writer.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("create entry %s: %w", name, err)
		}
		if _, err := writerFile.Write(data); err != nil {
			return fmt.Errorf("write entry %s: %w", name, err)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close zip: %w", err)
	}
	if err := os.WriteFile(path, buffer.Bytes(), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

//
// ──────────────────────────── РАБОТА С XML ────────────────────────────
//

// GetFile возвращает содержимое файла из архива.
func (d *Docx) GetFile(name string) ([]byte, bool) {
	data, ok := d.files[name]
	return data, ok
}

// SetFile обновляет или добавляет файл в документ.
func (d *Docx) SetFile(name string, data []byte) {
	name = strings.ReplaceAll(strings.TrimPrefix(name, "/"), "\\", "/")

	if strings.HasPrefix(name, "word/media/") {
		d.localMedia[name] = data
	} else {
		d.files[name] = data
	}
}

// ContentPart возвращает XML тела документа, хедера или футера.
func (d *Docx) ContentPart(part string) (string, error) {
	d.activePart = part

	if !strings.HasPrefix(part, "word/") {
		part = "word/" + part
	}
	if !strings.HasSuffix(part, ".xml") {
		part += ".xml"
	}
	data, ok := d.files[part]
	if !ok {
		return "", fmt.Errorf("no %s in docx", part)
	}
	return string(data), nil
}

// UpdateContentPart заменяет XML указанного раздела.
func (d *Docx) UpdateContentPart(part, content string) {
	if !strings.HasPrefix(part, "word/") {
		part = "word/" + part
	}
	if !strings.HasSuffix(part, ".xml") {
		part += ".xml"
	}
	d.files[part] = []byte(content)
}

// ListHeaderFooterParts возвращает имена всех headerX и footerX файлов,
// реально подключённых к документу через <w:headerReference> / <w:footerReference>.
// ListHeaderFooterParts возвращает имена всех реально подключённых header*/footer* файлов.
func (d *Docx) ListHeaderFooterParts() []string {
	const (
		docPath  = "word/document.xml"
		relsPath = "word/_rels/document.xml.rels"
	)
	var parts []string

	doc, ok1 := d.files[docPath]
	rels, ok2 := d.files[relsPath]
	if !ok1 || !ok2 {
		return parts
	}

	// ищем r:id из <w:headerReference> / <w:footerReference>
	re := regexp.MustCompile(`<w:(?:headerReference|footerReference)[^>]+r:id="([^"]+)"`)
	ids := re.FindAllStringSubmatch(string(doc), -1)
	if len(ids) == 0 {
		return parts
	}

	type Relationship struct {
		ID     string `xml:"Id,attr"`
		Type   string `xml:"Type,attr"`
		Target string `xml:"Target,attr"`
	}
	type Relationships struct {
		XMLName xml.Name       `xml:"Relationships"`
		Items   []Relationship `xml:"Relationship"`
	}
	var rel Relationships
	_ = xml.Unmarshal(rels, &rel)

	for _, id := range ids {
		for _, r := range rel.Items {
			if r.ID == id[1] &&
				(strings.Contains(r.Type, "/header") || strings.Contains(r.Type, "/footer")) {
				name := strings.TrimSuffix(filepath.Base(r.Target), ".xml")
				parts = append(parts, name)
			}
		}
	}
	return parts
}

//
// ──────────────────────────── ТЕМПЛАТЫ И МОДИФИКАТОРЫ ────────────────────────────
//

// ImportBuiltins добавляет встроенные стандартные модификаторы
// (qrcode, barcode и др.) через общий механизм ImportModifiers.
func (d *Docx) ImportBuiltins() {
	// добавляем QR сюда, чтобы несколько документов работали со своими данными, а globalMedia получал сведения о файлах
	mods := map[string]modifiers.ModifierMeta{
		"qrcode": {
			Func: func(value string, opts ...string) modifiers.RawXML {
				xmlData := d.QrCode(value, opts...)
				globalMedia.AddAll(d.localMedia)
				return xmlData
			},
			Count: 0,
		},
		"barcode": {
			Func: func(value string, opts ...string) modifiers.RawXML {
				xmlData := d.Barcode(value, opts...)
				globalMedia.AddAll(d.localMedia)
				return xmlData
			},
			Count: 0,
		},
	}

	d.ImportModifiers(mods)
}

// ExecuteTemplate выполняет шаблон документа, используя переданные данные.
func (d *Docx) ExecuteTemplate(data map[string]any) error {
	parts := d.ListHeaderFooterParts()
	parts = append(parts, "document")
	for _, part := range parts {
		content, err := d.ContentPart(part)
		if err != nil {
			if part == "document" {
				return fmt.Errorf("execute template: %w", err)
			} else {
				continue
			}
		}

		if content, err = d.RepairTags(content); err != nil {
			return fmt.Errorf("repair tags (initial): %w", err)
		}

		content = d.ResolveIncludes(content, data)
		content = d.ResolveTables(content, data)

		if content, err = d.RepairTags(content); err != nil {
			return fmt.Errorf("repair tags (after includes): %w", err)
		}

		content = d.ProcessUnWrapParagraphTags(content)
		content = d.ProcessTrimTags(content)

		// Преобразуем теги {var|mod} в {{ .var | mod }}
		content = TransformTemplate(content)

		d.ImportBuiltins()
		funcMap := modifiers.NewFuncMap(modifiers.Options{
			Fonts:      d.fonts,
			Data:       data,
			ExtraFuncs: d.extraFuncs,
		})

		tmpl, err := template.New("docx").
			Delims("{", "}").
			Funcs(funcMap).
			Parse(content)
		if err != nil {
			return fmt.Errorf("parse template: %w", err)
		}

		var out bytes.Buffer
		if err := tmpl.Execute(&out, data); err != nil {
			return fmt.Errorf("execute template: %w", err)
		}

		d.UpdateContentPart(part, out.String())
	}
	return nil
}

// ImportModifiers добавляет набор пользовательских модификаторов.
func (d *Docx) ImportModifiers(mods map[string]modifiers.ModifierMeta) {
	if d.extraFuncs == nil {
		d.extraFuncs = make(map[string]modifiers.ModifierMeta)
	}
	for k, v := range mods {
		d.extraFuncs[k] = v
	}
}

// AddModifier добавляет один модификатор.
func (d *Docx) AddModifier(name string, fn any, args int) {
	if d.extraFuncs == nil {
		d.extraFuncs = make(map[string]modifiers.ModifierMeta)
	}
	d.extraFuncs[name] = modifiers.ModifierMeta{Func: fn, Count: args}
}

// LoadFontsForPSplit подключает набор шрифтов для модификатора p_split.
func (d *Docx) LoadFontsForPSplit(pathRegular, pathBold, pathItalic, pathBoldItalic string) error {
	fonts, err := metrics.LoadFonts(pathRegular, pathBold, pathItalic, pathBoldItalic)
	if err != nil {
		return fmt.Errorf("load fonts: %w", err)
	}
	d.fonts = fonts
	return nil
}

//
// ──────────────────────────── МЕДИАФАЙЛЫ ────────────────────────────
//

// AddImageRel добавляет изображение и возвращает его rId + базовое имя.
func (d *Docx) AddImageRel(data []byte) (string, string) {
	hash := sha1.Sum(data)
	base := fmt.Sprintf("%s_%x", d.activePart, hash)
	filename := base + ".png"
	rId := "rId_" + base

	d.SetFile("word/media/"+filename, data)
	return rId, base
}

// updateMediaRelationships обновляет связи (rels) и MIME-типы для набора медиафайлов.
func (d *Docx) updateMediaRelationships(part string, filenames []string) {
	var relsPath = fmt.Sprintf("word/_rels/%s.xml.rels", part)

	// читаем или создаём <Relationships>
	relsData, _ := d.GetFile(relsPath)
	if len(relsData) == 0 {
		relsData = []byte(`<?xml version="1.0" encoding="UTF-8"?><Relationships></Relationships>`)
	}

	type Relationship struct {
		ID     string `xml:"Id,attr"`
		Type   string `xml:"Type,attr"`
		Target string `xml:"Target,attr"`
	}
	type Relationships struct {
		XMLName xml.Name       `xml:"Relationships"`
		XMLNS   string         `xml:"xmlns,attr,omitempty"`
		Items   []Relationship `xml:"Relationship"`
	}

	var rels Relationships
	err := xml.Unmarshal(relsData, &rels)
	if err != nil {
		return
	}
	if rels.XMLNS == "" {
		rels.XMLNS = "http://schemas.openxmlformats.org/package/2006/relationships"
	}

	existing := make(map[string]bool)
	for _, r := range rels.Items {
		existing[r.ID] = true
	}

	for _, name := range filenames {
		ext := filepath.Ext(name)
		base := strings.TrimSuffix(name, ext)
		rId := "rId_" + base
		if existing[rId] {
			continue
		}
		rels.Items = append(rels.Items, Relationship{
			ID:     rId,
			Type:   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image",
			Target: "media/" + name,
		})
	}

	output, _ := xml.MarshalIndent(rels, "", "  ")
	d.SetFile(relsPath, append([]byte(xml.Header), output...))
	d.updateContentTypes(filenames)
}

// updateContentTypes добавляет MIME-типы для набора изображений.
func (d *Docx) updateContentTypes(filenames []string) {
	const contentPath = "[Content_Types].xml"

	data, _ := d.GetFile(contentPath)
	if len(data) == 0 {
		data = []byte(`<?xml version="1.0" encoding="UTF-8"?><Types></Types>`)
	}

	type Override struct {
		PartName    string `xml:"PartName,attr"`
		ContentType string `xml:"ContentType,attr"`
	}
	type Types struct {
		XMLName   xml.Name   `xml:"Types"`
		XMLNS     string     `xml:"xmlns,attr,omitempty"`
		Overrides []Override `xml:"Override"`
	}

	var types Types
	err := xml.Unmarshal(data, &types)
	if err != nil {
		return
	}
	if types.XMLNS == "" {
		types.XMLNS = "http://schemas.openxmlformats.org/package/2006/content-types"
	}

	mime := map[string]string{
		"png":  "image/png",
		"jpg":  "image/jpeg",
		"jpeg": "image/jpeg",
		"gif":  "image/gif",
		"bmp":  "image/bmp",
		"tif":  "image/tiff",
		"tiff": "image/tiff",
		"svg":  "image/svg+xml",
	}

	exists := make(map[string]struct{})
	for _, o := range types.Overrides {
		exists[o.PartName] = struct{}{}
	}

	for _, file := range filenames {
		part := "/word/media/" + file
		if _, ok := exists[part]; ok {
			continue
		}
		ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(file), "."))
		ct := mime[ext]
		if ct == "" {
			ct = "application/octet-stream"
		}
		types.Overrides = append(types.Overrides, Override{
			PartName:    part,
			ContentType: ct,
		})
	}

	out, _ := xml.MarshalIndent(types, "", "  ")
	d.SetFile(contentPath, append([]byte(xml.Header), out...))
}

// SaveToWriter — записывает текущий документ DOCX напрямую в поток (например, http.ResponseWriter).
// Повторяет логику Save(), но не пишет во временный файл.
func (d *Docx) SaveToWriter(w io.Writer) error {
	buffer := new(bytes.Buffer)
	writer := zip.NewWriter(buffer)

	// 1. Объединяем все медиафайлы в единую карту
	mediaByPart := map[string][]string{}
	globalMedia.ForEach(func(filename string, data []byte) {
		d.files[filename] = data

		mediaName := strings.TrimPrefix(filename, "word/media/")
		parts := strings.SplitN(mediaName, "_", 2)
		part := "document"
		if len(parts) > 1 {
			switch {
			case strings.HasPrefix(parts[0], "header"):
				part = parts[0]
			case strings.HasPrefix(parts[0], "footer"):
				part = parts[0]
			}
		}
		mediaByPart[part] = append(mediaByPart[part], mediaName)
	})

	// 2. Обновляем rels и [Content_Types].xml
	for part, names := range mediaByPart {
		d.updateMediaRelationships(part, names)
	}

	// 3. Создаём ZIP-архив
	for name, data := range d.files {
		name = strings.TrimPrefix(name, "/")
		name = strings.ReplaceAll(name, "\\", "/")
		if strings.TrimSpace(name) == "" {
			continue
		}

		header := &zip.FileHeader{
			Name:     name,
			Method:   zip.Deflate,
			Modified: time.Now().UTC(),
		}
		writerFile, err := writer.CreateHeader(header)
		if err != nil {
			return fmt.Errorf("create entry %s: %w", name, err)
		}
		if _, err := writerFile.Write(data); err != nil {
			return fmt.Errorf("write entry %s: %w", name, err)
		}
	}

	if err := writer.Close(); err != nil {
		return fmt.Errorf("close zip: %w", err)
	}

	// 4. Отдаём результат в поток
	if _, err := io.Copy(w, buffer); err != nil {
		return fmt.Errorf("write to stream: %w", err)
	}
	return nil
}
