package docxgen

import (
	"archive/zip"
	"bytes"
	"docxgen/metrics"
	"docxgen/modifiers"
	"fmt"
	"io"
	"os"
	"strings"
	"text/template"
)

// Docx – основная структура, содержит файлы документа
type Docx struct {
	files      map[string][]byte // имя файла в архиве -> содержимое
	filePath   string            // путь к исходному файлу
	extraFuncs template.FuncMap  // сюда будем складывать кастомные модификаторы
	fonts      *metrics.FontSet
}

// Open – открыть docx как zip, считать все файлы и сразу починить теги
func Open(path string) (*Docx, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return nil, fmt.Errorf("open docx: %w", err)
	}
	defer r.Close()

	files := make(map[string][]byte)
	for _, f := range r.File {
		rc, err := f.Open()
		if err != nil {
			return nil, fmt.Errorf("read entry %s: %w", f.Name, err)
		}
		buf, err := io.ReadAll(rc)
		rc.Close()
		if err != nil {
			return nil, fmt.Errorf("read entry %s: %w", f.Name, err)
		}
		files[f.Name] = buf
	}

	doc := &Docx{
		files:    files,
		filePath: path,
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
	w := zip.NewWriter(buf)

	for name, data := range d.files {
		f, err := w.Create(name)
		if err != nil {
			return fmt.Errorf("create entry %s: %w", name, err)
		}
		if _, err := f.Write(data); err != nil {
			return fmt.Errorf("write entry %s: %w", name, err)
		}
	}

	if err := w.Close(); err != nil {
		return fmt.Errorf("close zip: %w", err)
	}

	return os.WriteFile(path, buf.Bytes(), 0644)
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

// SetFile – заменить содержимое файла
func (d *Docx) SetFile(name string, data []byte) {
	d.files[name] = data
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
	// получаем содержимое document.xml
	body, err := d.Content()
	if err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	// преобразуем синтаксис {fio|declension:`genitive`} → {{ .fio | declension "genitive" }}
	tmplSrc := TransformTemplate(body)

	// собираем FuncMap
	fm := modifiers.NewFuncMap(modifiers.Options{
		Fonts:      d.fonts,
		Data:       data,
		ExtraFuncs: d.extraFuncs,
	})

	// парсим и выполняем шаблон
	tmpl, err := template.New("docx").Delims("{", "}").Funcs(fm).Parse(tmplSrc)
	if err != nil {
		return fmt.Errorf("parse template: %w", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return fmt.Errorf("execute template: %w", err)
	}

	// заменяем содержимое
	d.UpdateContent(buf.String())
	return nil
}

// ImportModifiers – регистрирует пользовательские модификаторы
func (d *Docx) ImportModifiers(fm template.FuncMap) {
	if d.extraFuncs == nil {
		d.extraFuncs = make(template.FuncMap)
	}
	for k, v := range fm {
		d.extraFuncs[k] = v
	}
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
