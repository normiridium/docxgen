# 📄 docxgen — DOCX Document Generation Library for Go
[🇷🇺 Русский](README.ru.md) | [🇬🇧 English](README.md)

**docxgen** is a powerful and simple Go library for generating DOCX (Word) documents from templates.  
It supports nested structures, loops, tables, data substitution, QR/barcode insertion, and direct XML fragment manipulation.

---

## 🚀 Installation

```bash
go get github.com/normiridium/docxgen
```

---

## 🧩 Key Features

| Feature | Description |
|--------|-------------|
| 🔹 **DOCX Templating** | Supports `{var}`, `{if}`, `{range}` and modifiers |
| 🔹 **Loops & Conditions** | Use `{{range}}`, `{{if}}`, `{{else}}` as in Go templates |
| 🔹 **Custom Modifiers** | Add your own functions via `AddModifier` |
| 🔹 **Built‑in Modifiers** | `upper`, `lower`, `wrap`, `gender_select`, `qrcode`, `barcode` |
| 🔹 **Header/Footer Support** | Modify `headerX` and `footerX` sections |
| 🔹 **Includes** | `[include/file]` inside templates |
| 🔹 **Streaming Output** | `SaveToWriter(w)` – perfect for HTTP APIs |
| 🔹 **Font-based Text Splitting** | `LoadFontsForPSplit()` for proper width calculations |
| 🔹 **Dynamic Tables & Data Import** | via `{range}` and `[table/]` blocks |
| 🔹 **PDF Conversion (CLI)** | works via the `--pdf` flag |

---

## ⚡ Usage Example

```go
package main

import (
    "docxgen"
    "docxgen/modifiers"
    "fmt"
    "strings"
)

func main() {
    // 1. Open a template
    doc, err := docxgen.Open("examples/template.docx")
    if err != nil {
        panic(err)
    }

    // 2. Register custom modifiers
    doc.ImportModifiers(map[string]modifiers.ModifierMeta{
        "upper": {Fn: func(s string) string { return strings.ToUpper(s) }, Count: 0},
        "wrap":  {Fn: func(v, l, r string) string { return l + v + r }, Count: 2},
    })

    // 3. Prepare data
    data := map[string]any{
        "fio":     "Ivanov Ivan Ivanovich",
        "project": "DOCX Template Engine",
        "items": []map[string]any{
            {"n": 1, "task": "Import JSON"},
            {"n": 2, "task": "Render DOCX"},
            {"n": 3, "task": "PDF Conversion"},
        },
    }

    // 4. Render template
    if err := doc.ExecuteTemplate(data); err != nil {
        panic(err)
    }

    // 5. Save result
    if err := doc.Save("result.docx"); err != nil {
        panic(err)
    }

    fmt.Println("💚 Document created: result.docx")
}
```

---

## 🧾 Word Template Example

`template.docx`:

```
Hello, {fio|upper}!

Your project: {project}

Task list:
{range .items}
  {n}. {task}
{/range}
```

Output:

```
Hello, IVANOV IVAN IVANOVICH!

Your project: DOCX Template Engine

Task list:
  1. Import JSON
  2. Render DOCX
  3. PDF Conversion
```

---

## 🧠 Tags with Modifier Examples

| Modifier | Example                                        | Result                     |
|----------|------------------------------------------------|----------------------------|
| `wrap` | `{fio\|wrap:"(":")"}`                          | (Ivanov Ivan Ivanovich)    |
| `qrcode` | `{fio\|qrcode}`                                | inserts QR code            |
| `barcode` | `{code\|barcode}`                              | inserts barcode            |

[Detailed tags reference](tags.md)

[Detailed modifiers reference](modifiers/modifiers.md)

---

## 🧩 XML & Streaming

You can stream output (e.g., in an HTTP handler):

```go
func handler(w http.ResponseWriter, r *http.Request) {
    data := map[string]any{"fio": "Ivanov Ivan Ivanovich"}
    doc, _ := docxgen.Open("examples/template.docx")
    doc.ExecuteTemplate(data)

    w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
    w.Header().Set("Content-Disposition", "attachment; filename=result.docx")
    _ = doc.SaveToWriter(w)
}
```

---

## 📦 Working with Media

Add images manually:

```go
imageData, _ := os.ReadFile("photo.png")
rID, _ := doc.AddImageRel(imageData)
fmt.Println("image id:", rID)
```

docxgen updates `.rels` and `[Content_Types].xml` automatically.

---

## ⚙️ Core Methods

| Method | Description |
|--------|-------------|
| `Open(path)` | Opens and unpacks a DOCX |
| `Save(path)` | Writes the document back to DOCX |
| `SaveToWriter(w io.Writer)` | Streams DOCX to writer |
| `ExecuteTemplate(data)` | Applies template substitutions |
| `ContentPart("document")` | Returns main XML |
| `UpdateContentPart("document", xml)` | Replaces XML fragment |
| `ImportModifiers(map)` | Registers custom functions |
| `AddModifier(name, fn, args)` | Adds a single modifier |
| `LoadFontsForPSplit(...)` | Loads fonts for text measurement |
| `AddImageRel(data)` | Embeds an image |

---

## 🧩 CLI Tool

In `main/` directory:

```bash
go run ./main --in examples/template.docx --data examples/data.json --out examples/result.docx
```

Supports:

- file watching (`--watch`)
- HTTP server (`--serve`)
- PDF output (`--pdf`)
- PDF preview (`--pdf-preview`)

📘 Full CLI and HTTP daemon reference is available [here](main/README.md)

---

## 🪶 License

MIT © 2025 — normiridium  
Created with love and attention to detail 📄
