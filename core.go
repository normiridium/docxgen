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
	"strings"
	"text/template"
	"time"
)

var globalFiles map[string][]byte

// Docx – основная структура, содержит файлы документа
type Docx struct {
	files       map[string][]byte                 // имя файла в архиве -> содержимое
	globalFiles map[string][]byte                 // добавлено, для надёжного хранения вложений
	filePath    string                            // путь к исходному файлу
	extraFuncs  map[string]modifiers.ModifierMeta // сюда будем складывать кастомные модификаторы
	fonts       *metrics.FontSet
}

// Open – открыть docx как zip, считать все файлы и сразу починить теги
func Open(path string) (*Docx, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open docx: %w", err)
	}
	defer func(r *zip.ReadCloser) {
		_ = r.Close()
	}(r)

	files := make(map[string][]byte)
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("read entry %s: %w", f.Name, err)
		}
		buf, err := io.ReadAll(rc)
		_ = rc.Close() // только для чтения, можно не реагировать на ошибку закрытия
		if err != nil {
			return nil, fmt.Errorf("read entry %s: %w", f.Name, err)
		}
		files[f.Name] = buf
	}

	doc := &Docx{
		files:    files,
		filePath: path,
	}

	modifiers.QrCodeFunc = func(value string, opts ...string) modifiers.RawXML {
		d := doc.QrCode(value, opts...)
		globalFiles = doc.globalFiles
		return d
	}

	body, err := doc.Content()
	if err != nil {
		return nil, err
	}

	// сразу чиним теги и обрабатываем {*tag*}
	repairBody, err := doc.RepairTags(body)
	if err != nil {
		return nil, fmt.Errorf("repair tags: %w", err)
	}

	// теги которые должны убрать оборачивание вокруг себя параграфом при вставке
	repairBody = doc.ProcessUnWrapParagraphTags(repairBody)

	doc.UpdateContent(repairBody)

	return doc, nil
}

// Save – сохранить все файлы обратно в новый docx
func (d *Docx) Save(path string) error {
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// перед сохранением объединяем медиафайлы с основной картой
	for k, v := range globalFiles {
		d.files[k] = v
	}

	for name, data := range d.files {
		// нормализуем имя (zip не любит абсолютные и обратные пути)
		clean := strings.TrimPrefix(name, "/")
		clean = strings.ReplaceAll(clean, "\\", "/")
		clean = strings.TrimSpace(clean)
		if clean == "" {
			continue
		}

		// создаём entry вручную через FileHeader — zip сам добавит "виртуальные" каталоги
		h := &zip.FileHeader{
			Name:   clean,
			Method: zip.Deflate,
		}
		// для Windows Word важно, чтобы дата не была "нулевой"
		h.Modified = time.Now().UTC()

		f, err := zw.CreateHeader(h)
		if err != nil {
			return fmt.Errorf("create entry %s: %w", clean, err)
		}

		if _, err := f.Write(data); err != nil {
			return fmt.Errorf("write entry %s: %w", clean, err)
		}
	}

	if err := zw.Close(); err != nil {
		return fmt.Errorf("close zip: %w", err)
	}

	if err := os.WriteFile(path, buf.Bytes(), 0644); err != nil {
		return fmt.Errorf("write file: %w", err)
	}
	return nil
}

// Close – на будущее, если будут ресурсы для освобождения
func (d *Docx) Close() error {
	return nil
}

// GetFile – получить содержимое файла по имени (например word/document.xml)
func (d *Docx) GetFile(name string) ([]byte, bool) {
	data, ok := d.files[name]
	return data, ok
}

// Content – получить основной документ (word/document.xml)
func (d *Docx) Content() (string, error) {
	data, ok := d.files["word/document.xml"]
	if !ok {
		return "", fmt.Errorf("no document.xml in docx")
	}
	return string(data), nil
}

// UpdateContent – заменить основной документ
func (d *Docx) UpdateContent(content string) {
	d.files["word/document.xml"] = []byte(content)
}

// ReplaceTag – заменить тег (поддерживает {tag} и {*tag*})
func (d *Docx) ReplaceTag(tag, content string) error {
	normalized := tag
	if strings.HasPrefix(tag, "{*") && strings.HasSuffix(tag, "*}") {
		normalized = "{" + strings.TrimSpace(tag[2:len(tag)-2]) + "}"
	}

	body, err := d.Content()
	if err != nil {
		return fmt.Errorf("replace tag: %w", err)
	}

	updated := strings.ReplaceAll(body, normalized, content)
	d.UpdateContent(updated)
	return nil
}

// ExecuteTemplate – обработка документа через Go templates
func (d *Docx) ExecuteTemplate(data map[string]any) error {

	// 1) ещё раз чиним теги в случае, если include внёс новые разорванные <w:t>
	body, err := d.Content()
	if err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	body, err = d.RepairTags(body)
	if err != nil {
		return fmt.Errorf("repair tags after include: %w", err)
	}

	// 2) разворачиваем include перед шаблонизацией
	body = d.ResolveIncludes(body)

	body = d.ResolveTables(body, data)

	body, err = d.RepairTags(body)
	if err != nil {
		return fmt.Errorf("repair tags after include: %w", err)
	}

	body = d.ProcessUnWrapParagraphTags(body)
	body = d.ProcessTrimTags(body)

	// 3) преобразуем {fio|declension:`genitive`} → {{ .fio | declension "genitive" }}
	tmplSrc := TransformTemplate(body)

	// 4) собираем FuncMap
	fm := modifiers.NewFuncMap(modifiers.Options{
		Fonts:      d.fonts,
		Data:       data,
		ExtraFuncs: d.extraFuncs,
	})

	// 5) парсим Go-шаблон
	tmpl, err := template.New("docx").
		Delims("{", "}").
		Funcs(fm).
		Parse(tmplSrc)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	// 6) выполняем Go-шаблон
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	// 7) записываем результат
	d.UpdateContent(buf.String())
	return nil
}

// ImportModifiers – регистрирует пользовательские модификаторы (аналог ExtraFuncs)
func (d *Docx) ImportModifiers(fm map[string]modifiers.ModifierMeta) {
	if d.extraFuncs == nil {
		d.extraFuncs = make(map[string]modifiers.ModifierMeta)
	}
	for k, v := range fm {
		d.extraFuncs[k] = v
	}
}

// AddModifier — удобное добавление одного модификатора без карты
func (d *Docx) AddModifier(name string, fn any, count int) {
	if d.extraFuncs == nil {
		d.extraFuncs = make(map[string]modifiers.ModifierMeta)
	}
	d.extraFuncs[name] = modifiers.ModifierMeta{Fn: fn, Count: count}
}

// LoadFontsForPSplit – подгружает шрифты для работы модификатора p_split
func (d *Docx) LoadFontsForPSplit(pathRegular, pathBold, pathItalic, pathBoldItalic string) error {
	fonts, err := metrics.LoadFonts(pathRegular, pathBold, pathItalic, pathBoldItalic)
	if err != nil {
		return fmt.Errorf("load fonts for p_split: %w", err)
	}
	d.fonts = fonts
	return nil
}

// SetFile – заменить содержимое файла
func (d *Docx) SetFile(name string, data []byte) {
	name = strings.TrimPrefix(name, "/")
	name = strings.ReplaceAll(name, "\\", "/")

	// медиаконтент храним отдельно, чтобы не потерять при шаблонизации
	if strings.HasPrefix(name, "word/media/") ||
		strings.HasPrefix(name, "word/_rels/") ||
		strings.HasPrefix(name, "[Content_Types]") {
		if d.globalFiles == nil {
			d.globalFiles = make(map[string][]byte)
		}
		d.globalFiles[name] = data
	} else {
		d.files[name] = data
	}
}

// AddImageRel — добавляет связь для изображения (image/*) в document.xml.rels
// и регистрирует MIME-тип как Override в [Content_Types].xml.
func (d *Docx) AddImageRel(bdata []byte) (string, string) {
	const relsPath = "word/_rels/document.xml.rels"

	sum := sha1.Sum(bdata)
	base := fmt.Sprintf("%x", sum) // без расширения
	name := base + ".png"
	rId := fmt.Sprintf("rId_%s", base)

	// сохраняем файл
	d.SetFile("word/media/"+name, bdata)

	// --- читаем или создаём файл связей ---
	data, _ := d.GetFile(relsPath)
	if len(data) == 0 {
		data = []byte(`<?xml version="1.0" encoding="UTF-8"?><Relationships></Relationships>`)
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
	_ = xml.Unmarshal(data, &rels)

	if rels.XMLNS == "" {
		rels.XMLNS = "http://schemas.openxmlformats.org/package/2006/relationships"
	}

	for _, r := range rels.Items {
		if r.ID == rId {
			d.ensureContentType(name)
			return rId, base
		}
	}

	rels.Items = append(rels.Items, Relationship{
		ID:     rId,
		Type:   "http://schemas.openxmlformats.org/officeDocument/2006/relationships/image",
		Target: "media/" + name,
	})

	out, _ := xml.MarshalIndent(rels, "", "  ")
	xmlData := append([]byte(xml.Header), out...)
	d.SetFile(relsPath, xmlData)

	// добавляем MIME Override для расширения
	d.ensureContentType(name)
	return rId, base
}

// ensureContentType — добавляет Override-тип изображения в [Content_Types].xml.
func (d *Docx) ensureContentType(filename string) {
	const typesPath = "[Content_Types].xml"
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(filename), "."))

	mimeMap := map[string]string{
		"png":  "image/png",
		"jpg":  "image/jpeg",
		"jpeg": "image/jpeg",
		"gif":  "image/gif",
		"bmp":  "image/bmp",
		"tif":  "image/tiff",
		"tiff": "image/tiff",
		"svg":  "image/svg+xml",
	}
	mimeType, ok := mimeMap[ext]
	if !ok {
		mimeType = "application/octet-stream"
	}

	ctData, _ := d.GetFile(typesPath)
	if len(ctData) == 0 {
		ctData = []byte(`<?xml version="1.0" encoding="UTF-8"?><Types></Types>`)
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
	_ = xml.Unmarshal(ctData, &types)

	if types.XMLNS == "" {
		types.XMLNS = "http://schemas.openxmlformats.org/package/2006/content-types"
	}

	part := fmt.Sprintf("/word/media/%s", filename)
	for _, o := range types.Overrides {
		if o.PartName == part {
			return // уже есть
		}
	}

	types.Overrides = append(types.Overrides, Override{
		PartName:    part,
		ContentType: mimeType,
	})

	out, _ := xml.MarshalIndent(types, "", "  ")
	xmlData := append([]byte(xml.Header), out...)
	d.SetFile(typesPath, xmlData)
}
