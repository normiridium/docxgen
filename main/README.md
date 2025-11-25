# 📄 docxgen — DOCX Document Generator for Go

**docxgen** is a lightweight engine for injecting data into DOCX (Word) templates with modifiers, tables, images, barcodes, and QR codes.  
It can work both as a **CLI tool** and as an **HTTP daemon (API)** for server‑side document generation.

---

## 🚀 Features

- Insert JSON data into DOCX templates.
- Modifier support (`upper`, `lower`, `wrap`, `gender_select`, etc.).
- Insert QR codes and barcodes directly into templates.
- Automatic rebuild (`--watch`) when templates or data change.
- Daemon mode `--serve` with HTTP API for integrations.
- XML output mode for debugging templates.
- Supports local or base64‑encoded templates.
- Output directly to PDF (`--pdf`).
- Live PDF preview in a browser when using `--watch --pdf-preview`.

---

## 🧩 Installation

```bash
go install github.com/normiridium/docxgen/main@latest
```

or clone:

```bash
git clone https://github.com/normiridium/docxgen.git
cd docxgen/main
go run . --help
```

---

## 🛠️ CLI Usage Example

```bash
go run . --in ../examples/template.docx --data ../examples/data.json --out ../examples/result.docx
```

or enable watch mode with automatic rebuilds:

```bash
go run . --watch
```

📘 Example log:
```
💚 ready: /examples/template_out.docx
👀  watch mode (Ctrl+C to exit)
📝  changed: template.docx → waiting for debounce…
🔄  rebuilding…
💚  ready: /examples/template_out.docx
```

---

## 🌐 HTTP API (Daemon Mode)

Run the server:

```bash
go run . --serve
```

By default it listens at `http://localhost:8080`.

---

### ▶️ Generate DOCX

**POST /generate**

```http
POST http://localhost:8080/generate
Content-Type: application/json

{
  "template": "examples/test.docx",
  "data": {
    "fio": "Ivanov Ivan Ivanovich",
    "project": "DocX Template Engine"
  }
}
```

📤 Response: `result.docx`  
📄 Content-Type: `application/vnd.openxmlformats-officedocument.wordprocessingml.document`

---

### 🔍 XML Output

**POST /generate**

```http
POST http://localhost:8080/generate
Content-Type: application/json

{
  "template": "examples/test.docx",
  "format": "xml",
  "data": {
    "fio": "Ivanov Ivan Ivanovich"
  }
}
```

📄 Response: `application/xml`, raw Word document XML.

---

### 🧾 PDF Generation

**CLI:**

```bash
go run . --pdf --in examples/template.docx --data examples/data.json --out examples/result.pdf
```

**HTTP API:**

```http
POST http://localhost:8080/generate
Content-Type: application/json

{
  "template": "examples/test.docx",
  "format": "pdf",
  "data": { "fio": "Ivanov Ivan Ivanovich" }
}
```

📤 Response: `application/pdf`  
📄 Can be viewed in-browser or saved.

---

### 🖥️ Live PDF Preview

```bash
go run . --watch --pdf-preview
```

docxgen launches a local PDF preview server at `http://localhost:8090`  
and automatically updates the PDF when the template changes.

---

## 💡 Modifiers

| Name | Example | Result |
|------|---------|---------|
| `wrap` | `{fio\|wrap:"<<":">>"}` | <<Ivanov Ivan Ivanovich>> |
| `gender_select` | `{fio\|gender_select:"Dear Sir":"Dear Madam"}` | selects correct form using gender or FIO |

---

## ⚙️ Command-Line Flags

| Flag | Description |
|------|-------------|
| `--in` | Input DOCX template |
| `--data` | JSON file with data |
| `--out` | Output path |
| `--watch` | Watch for changes and rebuild |
| `--download` | Write DOCX to stdout instead of saving |
| `--serve` | Start HTTP daemon |
| `--port` | Daemon port (default `8080`) |
| `--pdf` | Save result as PDF |
| `--pdf-preview` | Browser preview of PDF when using `--watch` |

---

## 🌸 Example JSON Data

```json
{
  "fio": "Ivanov Ivan Ivanovich",
  "project": "DocX Template Engine",
  "date": "2025-11-03"
}
```

---

## 🪶 License

MIT © 2025 — normiridium  
Created with love and attention to detail 📄
