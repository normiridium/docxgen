# 📘 docxgen Special Tags Reference

In docxgen templates, not only modifier functions (`|money`, `|plural`, `|declension`, etc.) are used,  
but also built‑in **structural tags** that control insertions, whitespace, tables, and loops.  
These tags are processed inside the engine (`ProcessTrimTags`, `ResolveIncludes`, `ProcessUnWrapParagraphTags`, `ExecuteTemplate`).

---

## 🔖 Basic Markers

| Syntax               | Purpose | Example |
|----------------------|---------|---------|
| `{tag}`              | Regular data placeholder. | `{fio}` → “Ivanov Ivan Ivanovich” |
| `{tag\|mod1\|mod2:a}` | Tag with modifiers. | `{fio\|abbr\|prefix:\`citizen \`}` |
| `{.field}`           | Access to field inside `{range}`. | `{range .clients}{.name\|abbr}{end}` |

---

## 🧩 Whitespace & Line Control

| Syntax | Purpose | Example |
|--------|---------|---------|
| `{-tag-}` | Removes spaces and tabs around the tag. | `word {-tag-} word` → `wordtextword` |
| `{~tag~}` | Removes spaces, tabs **and line breaks** around the tag. | `line {tag~}\n\n\nline` → `linetextline` |
| `{-tag}` / `{tag-}` | Removes whitespace only on one side. | `{tag-} word` → `textword` |

---

## 🧱 Block Tags & Includes

| Syntax | Purpose | Example |
|--------|---------|---------|
| `{*tag*}` | “Unwraps” a paragraph into a standalone block. | `{*include_block*}` |
| `[include/file.docx]` | Inserts `<w:body>` content from external DOCX. | `[include/blocks/sign.docx]` |
| `[include/file.docx/table/2]` | Inserts the second table from DOCX. | `[include/report.docx/table/2]` |
| `[include/file.docx/p/3]` | Inserts the third paragraph. | `[include/text.docx/p/3]` |

Supported fragments:
- `body` — whole document body  
- `table` — tables (1..N)  
- `p` / `paragraph` — paragraphs (1..N)

---

## 📊 Tables & Loops

| Syntax | Purpose | Example |
|--------|---------|---------|
| `[table/name]` | Begin a table block. | `[table/budget_report]` |
| `[/table]` | End a table block. | `[/table]` |
| `{range .collection}{...}{end}` | Iteration (Go template style). | `{range .clients}{.name\|abbr}{end}` |
| `{range .clients}[include/blocks/sign.docx]{end}` | External block per element. | `{range .clients}[include/blocks/sign.docx]{end}` |
| `{n}`, `{annotation}`, `{deadline}`, `{price\|money}` | Tags inside table rows. | `{price\|money}` |

### How It Works

- `[table/name] ... [/table]` declares a table template.  
- Engine clones it for each element in corresponding data array.  
- Nested `{range}` allowed both inside and outside tables.

<pre>
[table/budget_report]
╔══════╦═════════════════════════════════════╦═══════════════╦═══════════════╗
║  №№  ║               Deadline              ║ Annotation    ║     Price     ║
╠══════╩═════════════════════════════════════╩═══════════════╩═══════════════╣
║                    {title_sub_block} (subtitle section)                    ║
╠══════╦═════════════════════════════════════╦═══════════════╦═══════════════╣
║  {n} ║ {deadline|date_format:`02.01.2006`} ║ {annotation}  ║ {price|money} ║
╚══════╩═════════════════════════════════════╩═══════════════╩═══════════════╝
[/table]
</pre>

---

## 📘 Combined Loop Example

```
{range .clients}[include/blocks/sign.docx]{end}
```

⟶ Inserts `sign.docx` for each client, with tags like `{.name}`, `{.phone}`, etc.

---

## Inside loops you can use:

- `.field` — current field (`{.name}`)  
- `.index` — index (if implemented)  
- Any modifiers (`|abbr`, `|nowrap`, `|declension`, etc.)

---

## 🧭 Special Elements

| Syntax | Description | Example |
|--------|-------------|---------|
| `{project.code\|qrcode}` | Inserts a QR code. | `{link\|qrcode:\`8%\`:\`5/5\`:\`border\`}` |
| `{range ...}{end}` | Loop. | `{range .clients}{.name} — {.phone}{end}` |
| `{~}` / `{-}` | Whitespace control. | `text {~fio-} text2` |

---

## ⚙️ Processing Order

1. **RepairTags** — merges `{}` / `[]` if Word split them.  
2. **ProcessUnWrapParagraphTags** — expands `{*tag*}` into blocks.  
3. **ResolveIncludes** — applies `[include/... ]`.  
4. **ProcessTrimTags** — handles whitespace tags.  
5. **ExecuteTemplate** — applies Go template engine + modifiers.

