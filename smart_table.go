package docxgen

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// ============================================================================
// Public API
// ============================================================================

// RenderSmartTable — DOCX-driven генерация одной таблицы.
// Правила:
// • named ({name}[|mod]) — рендерим строку ПОКА есть подходящие map-items (есть хотя бы один ключ из тэгов строки).
//   - {name}           → подставляем текст как есть (raw)  [см. xmlEscape хук ниже]
//   - {name|modifier}  → {{ `value` | modifier }} (backticks обязательно)
//
// • positional (%[N]s и кейсы в backticks внутри {`...`|mod}) — СЪЕДАЕМ РОВНО ОДИН slice-item на строку DOCX.
//   - голые %[N]s      → подставляем текст как есть (raw)
//   - {`%[N]s`|mod}    → {{ `value` | mod }}
//
// • строки без плейсхолдеров — статичные, выводим как есть.
// • неизвестные теги ({unknown}) оставляем как есть.
// • backticks сохраняем обязательно.
// • xml-экранирование отключено, но есть закомментированный хук (см. ниже).
func RenderSmartTable(tableXML string, items []any) (string, error) {
	inner := stripOuterTable(tableXML)
	rows := extractTableRows(inner)
	if len(rows) == 0 {
		return "", fmt.Errorf("smart table: no rows found")
	}

	// Разделяем items на очереди
	mapQueue := []map[string]any{}
	sliceQueue := [][]any{}
	knownKeys := make(map[string]struct{}) // для определения "совсем-неизвестных" named-строк как статичных

	for _, it := range items {
		if m, ok := extractInnerMapAny(it); ok {
			mapQueue = append(mapQueue, m)
			for k := range m {
				knownKeys[k] = struct{}{}
			}
			continue
		}
		if arr, ok := extractInnerSliceAny(it); ok {
			sliceQueue = append(sliceQueue, arr)
			continue
		}
	}

	var outRows []string

	for _, rowXML := range rows {
		meta := parseTplMeta(rowXML)

		switch {
		// ======= POSITIONAL (доминирует, даже если внутри фигурных) =======
		case meta.percentSeen > 0:
			if len(sliceQueue) > 0 {
				arr := sliceQueue[0]
				sliceQueue = sliceQueue[1:]
				outRows = append(outRows, renderPositional(rowXML, arr))
			}
			// иначе пропускаем строку

		// ======= NAMED (чистые имена до первого | или }) =======
		case len(meta.names) > 0:
			// Если ни один тег строки нигде не встречается — считаем статичной (например {company} в HEADER)
			if !metaHasAnyKnown(meta, knownKeys) {
				outRows = append(outRows, rowXML)
				continue
			}
			// Иначе — шаблон: реплицируем, пока есть подходящие map-items
			for {
				idx := findMatchingMap(meta, mapQueue)
				if idx < 0 {
					break
				}
				outRows = append(outRows, renderNamed(rowXML, meta, mapQueue[idx]))
				mapQueue = append(mapQueue[:idx], mapQueue[idx+1:]...)
			}

		// ======= STATIC =======
		default:
			outRows = append(outRows, rowXML)
		}
	}

	return TableOpeningTag + strings.Join(outRows, "") + TableEndingTag, nil
}

// ============================================================================
// Optional: Resolve [table name] ... </w:tbl> blocks against data[name]
// ============================================================================

// ResolveTables — находит блоки вида:
//
//	[table/name]
//	<w:tbl>...</w:tbl>
//	[/table]
//
// и заменяет их на результат RenderSmartTable(...), используя items из data[name].
//
// Вариант A (как договорились):
//   - если данных нет — таблицу оставляем как есть,
//     но абзацы с маркерами [table/...] и [/table] удаляем.
//   - если данные есть — подставляем отрендеренную таблицу на место абзаца с [table/...],
//     абзац с [/table] удаляем, исходную таблицу из блока вырезаем.
//
// Работает без регулярок, в стиле ResolveIncludes.
func (d *Docx) ResolveTables(body string, data map[string]any) string {
	const openPrefix = "[table/"
	const closeTag = "[/table]"

	for {
		// 1) ищем открывающий маркер
		start := strings.Index(body, openPrefix)
		if start < 0 {
			break
		}

		// ищем конец открывающего тега ']'
		openEnd := strings.Index(body[start:], "]")
		if openEnd < 0 {
			// битая разметка — удалим маркерный абзац и выйдем
			body = ReplaceTagWithParagraph(body, body[start:], "")
			break
		}
		openEnd = start + openEnd + 1

		openTag := body[start:openEnd] // например: [table/budget_report]
		name := strings.TrimSuffix(strings.TrimPrefix(openTag, openPrefix), "]")

		// 2) ищем закрывающий маркер [/table] ПОСЛЕ открывающего
		closePos := strings.Index(body[openEnd:], closeTag)
		if closePos < 0 {
			// нет закрывающего — просто удалим абзац с открывающим маркером
			body = ReplaceTagWithParagraph(body, openTag, "")
			break
		}
		closePos = openEnd + closePos

		// 3) содержимое между маркерами
		inner := body[openEnd:closePos]

		// 4) найдём первую таблицу внутри блока
		tblStart := strings.Index(inner, "<w:tbl")
		tblEnd := strings.Index(inner, "</w:tbl>")
		if tblStart < 0 || tblEnd < 0 {
			// таблицы нет — удалим оба маркера и двинемся дальше
			body = ReplaceTagWithParagraph(body, closeTag, "")
			body = ReplaceTagWithParagraph(body, openTag, "")
			continue
		}
		tblEnd += len("</w:tbl>")
		tableXML := inner[tblStart:tblEnd]

		// 5) подготовим исходник без блоков, если понадобится
		// (удалим закрывающий маркерный абзац сразу — он нам точно не нужен)
		body = ReplaceTagWithParagraph(body, closeTag, "")

		// 6) проверим наличие данных
		raw, ok := data[name]
		if !ok {
			// Данных нет → оставить таблицу как есть, только убрать маркеры:
			body = ReplaceTagWithParagraph(body, openTag, "")
			// (исходная таблица остаётся на месте между абзацами)
			continue
		}

		// 7) нормализуем items и рендерим
		items, ok := normalizeItems(raw)
		if !ok {
			// некорректный формат данных — оставляем исходную таблицу, убрав маркеры
			body = ReplaceTagWithParagraph(body, openTag, "")
			continue
		}

		rendered, err := RenderSmartTable(tableXML, items)
		if err != nil || strings.TrimSpace(rendered) == "" {
			// не получилось — оставим исходную таблицу, откроющий маркерный абзац уберём
			body = ReplaceTagWithParagraph(body, openTag, "")
			continue
		}

		// 8) удалим из документа исходную таблицу (первое вхождение в пределах блока)
		//    Так как мы ещё не трогали сам inner, tableXML в тексте всё ещё существует.
		//    Удаляем РОВНО одно вхождение, чтобы не задеть другие таблицы.
		body = strings.Replace(body, tableXML, "", 1)

		// 9) подставим отрендеренную таблицу вместо абзаца с открывающим маркером
		body = ReplaceTagWithParagraph(body, openTag, rendered)

		// 10) цикл продолжится — ищем следующий [table/...]
	}

	return body
}

func normalizeItems(v any) ([]any, bool) {
	switch x := v.(type) {
	case []any:
		return x, true
	case []map[string]any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = x[i]
		}
		return out, true
	case []map[string]string:
		out := make([]any, len(x))
		for i := range x {
			m := make(map[string]any, len(x[i]))
			for k, vv := range x[i] {
				m[k] = vv
			}
			out[i] = m
		}
		return out, true
	case [][]any:
		out := make([]any, len(x))
		for i := range x {
			out[i] = x[i]
		}
		return out, true
	case [][]string:
		out := make([]any, len(x))
		for i := range x {
			aa := make([]any, len(x[i]))
			for j := range x[i] {
				aa[j] = x[i][j]
			}
			out[i] = aa
		}
		return out, true
	}
	return nil, false
}

// ============================================================================
// Matching / Rendering
// ============================================================================

func metaHasAnyKnown(meta tplMeta, known map[string]struct{}) bool {
	for _, n := range meta.names {
		if _, ok := known[n]; ok {
			return true
		}
	}
	return false
}

func findMatchingMap(meta tplMeta, queue []map[string]any) int {
	for i, m := range queue {
		for _, name := range meta.names {
			if _, ok := m[name]; ok {
				return i
			}
		}
	}
	return -1
}

// renderNamed:
// 1) {name|mod...} → {{ `value` | mod... }}  (если name есть в data)
// 2) {name}        → value (raw)             (если name есть в data)
// 3) неизвестные теги оставляем как есть
func renderNamed(xmlTpl string, meta tplMeta, data map[string]any) string {
	out := xmlTpl

	// Сначала — {name|mod...}
	reNameMod := regexp.MustCompile(`\{[ \t]*([A-Za-z0-9_.]+)[ \t]*\|([^}]*)}`)
	out = reNameMod.ReplaceAllStringFunc(out, func(tok string) string {
		m := reNameMod.FindStringSubmatch(tok)
		if len(m) != 3 {
			return tok
		}
		name := m[1]
		modTail := strings.TrimSpace(m[2])
		valAny, ok := data[name]
		if !ok {
			return tok // неизвестное имя — оставляем как есть
		}
		val := fmt.Sprint(valAny)

		// // Включить XML-экранирование при необходимости:
		// val = xmlEscape(val)

		// { `value` | modTail }
		return "{ `" + val + "` | " + modTail + " }"
	})

	// Затем — чистые {name}
	for _, name := range meta.names {
		valAny, ok := data[name]
		if !ok {
			continue // оставляем {name} как есть
		}
		val := fmt.Sprint(valAny)
		// val = xmlEscape(val) // <- включить, если решишь экранировать

		// заменяем только "чистые" {name} (без модификатора)
		reExact := regexp.MustCompile(`\{[ \t]*` + regexp.QuoteMeta(name) + `[ \t]*\}`)
		out = reExact.ReplaceAllString(out, val)
	}

	return out
}

// renderPositional:
// 1) {`%[N]s ...`|mod} → {{ `resolved` | mod }}
// 2) голые %[N]s       → текстовые подстановки (raw)
// Паддинг пустыми строками при нехватке значений — встроен.
func renderPositional(xmlTpl string, arr []any) string {
	out := xmlTpl

	// 1) {`...`|mod} с возможными %[N]s внутри backticks
	reBacktickMod := regexp.MustCompile("(?s)\\{[ \\t]*`([^`]*)`[ \\t]*\\|([^}]*)}")
	out = reBacktickMod.ReplaceAllStringFunc(out, func(tok string) string {
		m := reBacktickMod.FindStringSubmatch(tok)
		if len(m) != 3 {
			return tok
		}
		rawInside := m[1] // может содержать %[N]s
		modTail := strings.TrimSpace(m[2])

		resolved := replacePerc(rawInside, arr)

		// resolved = xmlEscape(resolved) // <- включить, если решишь экранировать
		return "{{ `" + resolved + "` | " + modTail + " }}"
	})

	// 2) Оставшиеся %[N]s (вне модификаторных блоков) → текст
	out = replacePerc(out, arr)

	return out
}

// replacePerc — заменяет все %[N]s в строке значениями из arr.
// Индексация 1-базная, при нехватке значений — пустая строка.
func replacePerc(s string, arr []any) string {
	return rePerc.ReplaceAllStringFunc(s, func(tok string) string {
		m := rePerc.FindStringSubmatch(tok)
		if len(m) != 2 {
			return ""
		}
		n, _ := strconv.Atoi(strings.TrimSpace(m[1]))
		idx := n - 1
		if idx < 0 || idx >= len(arr) {
			return ""
		}
		val := fmt.Sprint(arr[idx])
		// val = xmlEscape(val) // <- включить, если решишь экранировать
		return val
	})
}

// ============================================================================
// Template Meta
// ============================================================================

type tplMeta struct {
	names       []string // имена {name} до первого | или }
	percentSeen int      // количество встретившихся %[N]s
}

var (
	// Имена: захватывает {fio}, {fio|...}, {dep.team}, {dep.team | ...}
	reBraceName = regexp.MustCompile(`\{[ \t]*([A-Za-z0-9_.]+)[ \t]*[|}]`)
	// Позиционные: %[N]s
	rePerc = regexp.MustCompile(`%\[\s*(\d+)\s*]s`)
)

func parseTplMeta(rowXML string) tplMeta {
	meta := tplMeta{}
	for _, m := range reBraceName.FindAllStringSubmatch(rowXML, -1) {
		if len(m) >= 2 {
			name := strings.TrimSpace(m[1])
			if name != "" {
				meta.names = append(meta.names, name)
			}
		}
	}
	meta.percentSeen = len(rePerc.FindAllStringSubmatch(rowXML, -1))
	return meta
}

// ============================================================================
// Data Extractors
// ============================================================================

func extractInnerMapAny(it any) (map[string]any, bool) {
	if outer, ok := it.(map[string]any); ok {
		if len(outer) == 1 {
			for _, v := range outer {
				if inner, ok := v.(map[string]any); ok {
					return inner, true
				}
			}
		}
		// fallback: если в значениях нет массивов — считаем плоским map
		for _, v := range outer {
			switch v.(type) {
			case []any, []string:
				return nil, false
			}
		}
		return outer, true
	}
	return nil, false
}

func extractInnerSliceAny(it any) ([]any, bool) {
	if outer, ok := it.(map[string]any); ok && len(outer) == 1 {
		for _, v := range outer {
			switch x := v.(type) {
			case []any:
				return x, true
			case []string:
				res := make([]any, len(x))
				for i := range x {
					res[i] = x[i]
				}
				return res, true
			}
		}
	}
	return nil, false
}

// ============================================================================
// XML / DOCX utils
// ============================================================================

func stripOuterTable(s string) string {
	trim := strings.TrimSpace(s)
	if strings.HasPrefix(trim, TableOpeningTag) && strings.HasSuffix(trim, TableEndingTag) {
		trim = strings.TrimPrefix(trim, TableOpeningTag)
		trim = strings.TrimSuffix(trim, TableEndingTag)
	}
	return trim
}

func extractTableRows(tbl string) []string {
	s := strings.TrimSpace(tbl)
	parts := strings.Split(s, TableRowClosingTag)
	var rows []string
	for _, p := range parts {
		if strings.Contains(strings.ToLower(p), TableRowPartTag) {
			rows = append(rows, p+TableRowClosingTag)
		}
	}
	return rows
}

// XML-escape хук — по умолчанию выключен.
// Оставляю, чтобы можно было легко включить в нужный момент.
// минимальная XML-экранизация (вставляем в <w:t>)
/*
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, `'`, "&apos;")
	return s
}
*/
