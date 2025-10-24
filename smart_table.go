package docxgen

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

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

/*
КАНОН:

• Порядок строк = порядок данных (DATA диктует порядок).
• DOCX — библиотека форм: уникальные шаблонные строки (named/positional),
  HEADER (до первой шаблонной строки) и FOOTER (после последней).
• Matching: Pass#1 (биндинг key→template), Pass#2 (waitZone retry), Pass#3 (bucket fields union).
• Render: идём ПО ДАННЫМ, для каждого item используем закреплённый шаблон и правила подстановки:
    L1  — локальное поле из item
    L2  — если поле встречалось в bucket (union), но в item его нет → подставляем "" (E1)
    L3  — если поле глобальное → оставляем {name} как есть, ExecuteTemplate потом разберёт
    L4  — если нигде нет → оставляем {name} как есть
• Positional: 1 item slice → 1 строка шаблона; %[N]s и {`%[N]s`|mod} поддержаны.
• Backticks сохраняем обязательно.
*/

// ============================================================================
// Public API
// ============================================================================
type normItem struct {
	raw      any
	groupKey string
	kind     string
	mapVal   map[string]any
	sliceVal []any
}

func RenderSmartTable(tableXML string, items []any) (string, error) {
	inner := stripOuterTable(tableXML)
	rows := extractTableRows(inner)
	if len(rows) == 0 {
		return "", fmt.Errorf("smart table: no rows found")
	}

	// 1) Разметим строки таблицы: header / templateRows / footer
	type tplRow struct {
		idx      int
		xml      string
		meta     tplMeta
		isNamed  bool
		isPos    bool
		isStatic bool
	}
	var (
		tplRows     []tplRow
		firstTplIdx = -1
		lastTplIdx  = -1
	)
	localKeys := collectLocalKeys(items)
	for i, r := range rows {
		m := parseTplMeta(r)
		isPos := m.percentSeen > 0
		isNamed := !isPos && len(m.names) > 0 && metaHasAnyKnown(m, localKeys)
		isStatic := !isPos && !isNamed
		tr := tplRow{idx: i, xml: r, meta: m, isNamed: isNamed, isPos: isPos, isStatic: isStatic}
		if isNamed || isPos {
			if firstTplIdx == -1 {
				firstTplIdx = i
			}
			lastTplIdx = i
		}
		tplRows = append(tplRows, tr)
	}

	// Header/ Footer
	var headerRows, footerRows []string
	if firstTplIdx > 0 {
		for i := 0; i < firstTplIdx; i++ {
			headerRows = append(headerRows, tplRows[i].xml)
		}
	}
	if lastTplIdx >= 0 && lastTplIdx < len(tplRows)-1 {
		for i := lastTplIdx + 1; i < len(tplRows); i++ {
			footerRows = append(footerRows, tplRows[i].xml)
		}
	}

	// Коллекция только шаблонных строк (named/positional) — библиотека форм
	var templates []tplRow
	for _, tr := range tplRows {
		if tr.isNamed || tr.isPos {
			templates = append(templates, tr)
		}
	}
	if len(templates) == 0 {
		// нет ни одной шаблонной строки → вернуть исходную таблицу
		return TableOpeningTag + inner + TableEndingTag, nil
	}

	var nitems []normItem
	for _, it := range items {
		ni := normalizeItem(it)
		if ni.kind == "other" {
			// одиночные скаляры не поддерживаем как осмысленные строки (оставим на будущее)
			continue
		}
		nitems = append(nitems, ni)
	}
	if len(nitems) == 0 {
		// только header+footer
		return TableOpeningTag + strings.Join(headerRows, "") + strings.Join(footerRows, "") + TableEndingTag, nil
	}

	// 3) Matching Phase#1: биндинг key→template, плюс waitZone
	binding := make(map[string]int)      // groupKey -> templates[idx]
	assigned := make([]int, len(nitems)) // по индексу item -> индекс templates, либо -1 (skip) либо -2 (wait)
	for i := range assigned {
		assigned[i] = -2 // по умолчанию в "ожидании"
	}
	type bucket struct {
		tplIdx int
		items  []int // индексы nitems
	}
	// buckets по индексу шаблона
	buckets := make([]bucket, len(templates))
	for i := range buckets {
		buckets[i] = bucket{tplIdx: i}
	}

	tryMatch := func(it normItem) (tplIdx int, score int) {
		bestScore := -1
		bestIdx := -1
		for i, t := range templates {
			sc := 0
			if it.kind == "map" && t.isNamed {
				// score = число совпавших полей
				for _, name := range t.meta.names {
					if _, ok := it.mapVal[name]; ok {
						sc++
					}
				}
			} else if it.kind == "slice" && t.isPos {
				// score по близости количества %[N]s
				seen := t.meta.percentSeen
				diff := seen - len(it.sliceVal)
				if diff < 0 {
					diff = -diff
				}
				if seen == len(it.sliceVal) {
					sc = 1000 + seen // идеал
				} else {
					sc = 100 - diff // чем ближе — тем выше
				}
			}
			if sc > bestScore {
				bestScore = sc
				bestIdx = i
			}
		}
		if bestScore <= 0 {
			return -1, 0
		}
		return bestIdx, bestScore
	}

	// Pass #1
	waitZone := []int{}
	for idx, it := range nitems {
		// если уже есть биндинг на группу — используем его сразу
		if it.groupKey != "" {
			if b, ok := binding[it.groupKey]; ok {
				assigned[idx] = b
				buckets[b].items = append(buckets[b].items, idx)
				continue
			}
		}

		tplIdx, sc := tryMatch(it)
		if sc > 0 {
			assigned[idx] = tplIdx
			buckets[tplIdx].items = append(buckets[tplIdx].items, idx)
			// фиксируем биндинг для группы
			if it.groupKey != "" {
				binding[it.groupKey] = tplIdx
			}
		} else {
			waitZone = append(waitZone, idx)
		}
	}

	// Pass #2 — retry waitZone
	if len(waitZone) > 0 {
		for _, idx := range waitZone {
			it := nitems[idx]
			// если после первого прохода биндинг для группы появился — кидаем туда
			if it.groupKey != "" {
				if b, ok := binding[it.groupKey]; ok {
					assigned[idx] = b
					buckets[b].items = append(buckets[b].items, idx)
					continue
				}
			}
			// иначе ещё раз пробуем подобрать шаблон
			tplIdx, sc := tryMatch(it)
			if sc > 0 {
				assigned[idx] = tplIdx
				buckets[tplIdx].items = append(buckets[tplIdx].items, idx)
				if it.groupKey != "" {
					binding[it.groupKey] = tplIdx
				}
			} else {
				// B-логика: не подошёл — пропускаем
				assigned[idx] = -1
			}
		}
	}

	// Pass #3 — нормализация дырок внутри каждого bucket (union полей)
	// unionFields[tplIdx] -> set of names seen in bucket for named rows
	unionFields := make([]map[string]struct{}, len(templates))
	for i := range unionFields {
		unionFields[i] = make(map[string]struct{})
	}
	for tIdx, b := range buckets {
		if !templates[tIdx].isNamed {
			continue
		}
		for _, itemIdx := range b.items {
			mv := nitems[itemIdx].mapVal
			for k := range mv {
				unionFields[tIdx][k] = struct{}{}
			}
		}
	}

	// 4) Генерация результата: HEADER + (по данным) + FOOTER
	var outRows []string
	if len(headerRows) > 0 {
		outRows = append(outRows, headerRows...)
	}

	for i, it := range nitems {
		tidx := assigned[i]
		if tidx < 0 {
			// skip
			continue
		}
		t := templates[tidx]
		if t.isPos {
			outRows = append(outRows, renderPositional(t.xml, it.sliceVal))
			continue
		}
		// named
		outRows = append(outRows, renderNamedWithUnion(t.xml, t.meta, it.mapVal, unionFields[tidx]))
	}

	if len(footerRows) > 0 {
		outRows = append(outRows, footerRows...)
	}

	return TableOpeningTag + strings.Join(outRows, "") + TableEndingTag, nil
}

func metaHasAnyKnown(meta tplMeta, known map[string]struct{}) bool {
	for _, n := range meta.names {
		if _, ok := known[n]; ok {
			return true
		}
	}
	return false
}

// collectLocalKeys вытаскивает имена локальных полей из входных items.
// Смотрим {"group": { ... }} и плоские map[string]any без слайсов.
func collectLocalKeys(items []any) map[string]struct{} {
	keys := make(map[string]struct{})
	for _, it := range items {
		switch m := it.(type) {

		case map[string]any:
			// {"group": {...}} → берём ключи из inner map
			if len(m) == 1 {
				for _, v := range m {
					if inner, ok := v.(map[string]any); ok {
						for k := range inner {
							keys[k] = struct{}{}
						}
					}
				}
				break
			}

			// плоский map без слайсов → тоже считаем локальными ключами
			flat := true
			for _, v := range m {
				switch v.(type) {
				case []any, []string:
					flat = false
				}
			}
			if flat {
				for k := range m {
					keys[k] = struct{}{}
				}
			}
		}
	}
	return keys
}

// ============================================================================
// Rendering helpers
// ============================================================================

// L1/L2-bucket/L3-global/L4-leave реализация:
// - если name в data → подставляем
// - иначе если name присутствует в union (встречался в других item’ах bucket’а) → подставляем ""
// - иначе оставляем как есть (глобальные теги обработает ExecuteTemplate)
func renderNamedWithUnion(xmlTpl string, meta tplMeta, data map[string]any, union map[string]struct{}) string {
	out := xmlTpl

	// 1) {name|mod...}
	reNameMod := regexp.MustCompile(`\{[ \t]*([A-Za-z0-9_.]+)[ \t]*\|([^}]*)}`)
	out = reNameMod.ReplaceAllStringFunc(out, func(tok string) string {
		m := reNameMod.FindStringSubmatch(tok)
		if len(m) != 3 {
			return tok
		}
		name := m[1]
		modTail := strings.TrimSpace(m[2])
		if valAny, ok := data[name]; ok {
			val := fmt.Sprint(valAny)
			return "{ `" + val + "` | " + modTail + " }"
		}
		// L2 — если поле встречается в bucket → пустая строка через модификатор
		if _, seen := union[name]; seen {
			return "{ `` | " + modTail + " }"
		}
		// L3/L4 — оставляем как есть
		return tok
	})

	// 2) Чистые {name}
	for _, name := range meta.names {
		// чистые — это ровно { name } без пайпа
		reExact := regexp.MustCompile(`\{[ \t]*` + regexp.QuoteMeta(name) + `[ \t]*\}`)
		if valAny, ok := data[name]; ok {
			val := fmt.Sprint(valAny)
			out = reExact.ReplaceAllString(out, val)
			continue
		}
		// L2 — если поле есть в union → ставим ""
		if _, seen := union[name]; seen {
			out = reExact.ReplaceAllString(out, "")
		}
		// иначе L3/L4 — оставить как есть
	}

	return out
}

// Positional:
// 1) {`...%[N]s...`|mod} → { `resolved` | mod }
// 2) голые %[N]s → текст
func renderPositional(xmlTpl string, arr []any) string {
	out := xmlTpl

	reBacktickMod := regexp.MustCompile("(?s)\\{[ \\t]*`([^`]*)`[ \\t]*\\|([^}]*)}")
	out = reBacktickMod.ReplaceAllStringFunc(out, func(tok string) string {
		m := reBacktickMod.FindStringSubmatch(tok)
		if len(m) != 3 {
			return tok
		}
		rawInside := m[1]
		modTail := strings.TrimSpace(m[2])
		resolved := replacePerc(rawInside, arr)
		return "{ `" + resolved + "` | " + modTail + " }"
	})

	out = replacePerc(out, arr)
	return out
}

// replacePerc — заменяет %[N]s в строке из arr (1-базная индексация).
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
		return fmt.Sprint(arr[idx])
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
	// Имена: {fio}, {dep.team}, {fio|...}, {dep.team | ...}
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
// Data normalization
// ============================================================================

func normalizeItem(v any) normItem {
	// первичный кейс: {"group": {...}} или {"group": []}
	if outer, ok := v.(map[string]any); ok && len(outer) == 1 {
		for gk, inner := range outer {
			switch x := inner.(type) {
			case map[string]any:
				return normItem{raw: v, groupKey: gk, kind: "map", mapVal: x}
			case map[string]string:
				mv := make(map[string]any, len(x))
				for k, vv := range x {
					mv[k] = vv
				}
				return normItem{raw: v, groupKey: gk, kind: "map", mapVal: mv}
			case []any:
				return normItem{raw: v, groupKey: gk, kind: "slice", sliceVal: x}
			case []string:
				ss := make([]any, len(x))
				for i := range x {
					ss[i] = x[i]
				}
				return normItem{raw: v, groupKey: gk, kind: "slice", sliceVal: ss}
			}
		}
	}

	// запасной вариант: плоский map (будем считать одноразовым map-item без явного groupKey)
	if m, ok := v.(map[string]any); ok {
		// если в значениях встречаются слайсы — не считаем map-item
		for _, vv := range m {
			switch vv.(type) {
			case []any, []string:
				return normItem{raw: v, kind: "other"}
			}
		}
		return normItem{raw: v, kind: "map", mapVal: m}
	}

	// слайсы без обёртки — считаем positional item
	if a, ok := v.([]any); ok {
		return normItem{raw: v, kind: "slice", sliceVal: a}
	}
	if s, ok := v.([]string); ok {
		ss := make([]any, len(s))
		for i := range s {
			ss[i] = s[i]
		}
		return normItem{raw: v, kind: "slice", sliceVal: ss}
	}

	return normItem{raw: v, kind: "other"}
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

/*
// XML-escape хук (по умолчанию выключен — мы отдаём в <w:t> raw, а строковые моды через Go-templates)
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, `'`, "&apos;")
	return s
}
*/
