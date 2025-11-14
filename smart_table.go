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

// ResolveTables — finds blocks of the form:
//
// [table/name]
//
//	<w:tbl>...</w:tbl>
//	[/table]
//
// and replaces them with the result of RenderSmartTable(...) using items from data[name].
//
// Option A (as agreed):
//   - if there is no data, leave the table as it is,
//     However, the paragraphs with the [table/...] and [/table] markers are removed.
//   - if there is data, substitute the rendered table in place of the paragraph with [table/...],
//     Delete the paragraph with [/table], cut out the original table from the block.
//
// It works without regulars, in the ResolveIncludes style.
func (d *Docx) ResolveTables(body string, data map[string]any) string {
	const openPrefix = "[table/"
	const closeTag = "[/table]"

	for {
		// 1) Looking for the opening marker
		start := strings.Index(body, openPrefix)
		if start < 0 {
			break
		}

		// 2) Looking for the end of the opening ']' tag
		openEnd := strings.Index(body[start:], "]")
		if openEnd < 0 {
			// broken markup — delete the bullet paragraph and exit
			body = ReplaceTagWithParagraph(body, body[start:], "")
			break
		}
		openEnd = start + openEnd + 1

		openTag := body[start:openEnd] // For example: [table/budget_report]
		name := strings.TrimSuffix(strings.TrimPrefix(openTag, openPrefix), "]")

		// 3) look for the closing marker [/table] AFTER the opening
		closePos := strings.Index(body[openEnd:], closeTag)
		if closePos < 0 {
			// if there is no closing marker, just delete the paragraph with the opening marker
			body = ReplaceTagWithParagraph(body, openTag, "")
			break
		}
		closePos = openEnd + closePos

		// 4) content between markers
		inner := body[openEnd:closePos]

		// 5) Let's find the first table inside the block
		tblStart := strings.Index(inner, "<w:tbl")
		tblEnd := strings.Index(inner, "</w:tbl>")
		if tblStart < 0 || tblEnd < 0 {
			// There is no table, so remove both markers and move on
			body = ReplaceTagWithParagraph(body, closeTag, "")
			body = ReplaceTagWithParagraph(body, openTag, "")
			continue
		}
		tblEnd += len("</w:tbl>")
		tableXML := inner[tblStart:tblEnd]

		// 6) prepare the source code without blocks, if necessary
		// (let's remove the closing bullet paragraph right away — we definitely don't need it)
		body = ReplaceTagWithParagraph(body, closeTag, "")

		// 7) Let's check the availability of data
		raw, ok := data[name]
		if !ok {
			// There is no data → leave the table as it is, only remove the markers:
			body = ReplaceTagWithParagraph(body, openTag, "")
			// (the original table remains in place between paragraphs)
			continue
		}

		// 8) normalize items and render
		items, ok := normalizeItems(raw)
		if !ok {
			// Incorrect data format — leave the original table, removing the markers
			body = ReplaceTagWithParagraph(body, openTag, "")
			continue
		}

		rendered, err := RenderSmartTable(tableXML, items)
		if err != nil || strings.TrimSpace(rendered) == "" {
			// If it doesn't work, we'll keep the original table, and remove the opening bullet paragraph
			body = ReplaceTagWithParagraph(body, openTag, "")
			continue
		}

		// 9) delete the source table from the document (the first occurrence within the block)
		// Since we haven't touched inner itself yet, tableXML in the text still exists.
		// Remove EXACTLY one occurrence so as not to touch other tables.
		body = strings.Replace(body, tableXML, "", 1)

		// 10) Substitute a rendered table instead of a paragraph with an opening marker
		body = ReplaceTagWithParagraph(body, openTag, rendered)

		// 11) The cycle will continue — looking for the next one [table/...]
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
 CANON:

• Row order = data order (DATA dictates the order).
• DOCX — form library: unique template strings (named/positional),
  HEADER (before the first placeholder line) and FOOTER (after the last).
• Matching: Pass#1 (key→template binding), Pass#2 (waitZone retry), Pass#3 (bucket fields union).
• Render: Go BY DATA, use a pinned template and substitution rules for each item:
    L1 is the local field from item
    L2 — if the field was found in bucket (union), but it is not in item → substitute "" (E1)
    L3 — if the global field → leave {name} as is, ExecuteTemplate will parse the
    L4 — if it's nowhere → leave {name} as it is
• Positional: 1 item slice → 1 template line; %[N]s and {'%[N]s'|mod} are supported.
• Backticks must be saved.
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

	// 1) Mark up the rows of the table: header / templateRows / footer
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

	// Named/positional Only Collection - Form Library
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
			// Single scalars are not supported as meaningful strings (we'll leave them for later)
			continue
		}
		nitems = append(nitems, ni)
	}
	if len(nitems) == 0 {
		// only header+footer
		return TableOpeningTag + strings.Join(headerRows, "") + strings.Join(footerRows, "") + TableEndingTag, nil
	}

	// 3) Matching Phase#1: key→template binding, plus waitZone
	binding := make(map[string]int)      // groupKey -> templates[idx]
	assigned := make([]int, len(nitems)) // either -1 (skip) or -2 (wait)
	for i := range assigned {
		assigned[i] = -2 // Default in "wait"
	}
	type bucket struct {
		tplIdx int
		items  []int // indices nitems
	}
	// buckets by template index
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
				// score = Number of matched fields
				for _, name := range t.meta.names {
					if _, ok := it.mapVal[name]; ok {
						sc++
					}
				}
			} else if it.kind == "slice" && t.isPos {
				// score by proximity to quantity %[N]s
				seen := t.meta.percentSeen
				diff := seen - len(it.sliceVal)
				if diff < 0 {
					diff = -diff
				}
				if seen == len(it.sliceVal) {
					sc = 1000 + seen // ideal
				} else {
					sc = 100 - diff // The closer you are, the higher
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
		// If there is already a binding for the group, use it immediately
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
			// Fixing the binding for the group
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
			// If after the first pass there is binding for the group, throw it there
			if it.groupKey != "" {
				if b, ok := binding[it.groupKey]; ok {
					assigned[idx] = b
					buckets[b].items = append(buckets[b].items, idx)
					continue
				}
			}
			// Otherwise, we try to choose a template again
			tplIdx, sc := tryMatch(it)
			if sc > 0 {
				assigned[idx] = tplIdx
				buckets[tplIdx].items = append(buckets[tplIdx].items, idx)
				if it.groupKey != "" {
					binding[it.groupKey] = tplIdx
				}
			} else {
				// Logic: if it doesn't fit, we let it pass
				assigned[idx] = -1
			}
		}
	}

	// Pass #3 — normalizing holes inside each bucket (union fields)
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

	// 4) Result generation: HEADER + (based on data) + FOOTER
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

// collectLocalKeys pulls the names of local fields from the input items.
// Look at {"group": { ... }} and flat map[string]any without slices.
func collectLocalKeys(items []any) map[string]struct{} {
	keys := make(map[string]struct{})
	for _, it := range items {
		switch m := it.(type) {

		case map[string]any:
			// {"group": {...}} = Taking the keys from the Inner Map
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

			// Flat map without slices = also considered local keys
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

// L1/L2-bucket/L3-global/L4-leave implementation:
// - if the name in the data → is substituted
// - otherwise, if the name is present in union (found in other bucket items) → substitute ""
// - otherwise leave it as it is (global tags will be handled by ExecuteTemplate)
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
		// L2 — if a field occurs in bucket → an empty string via the
		if _, seen := union[name]; seen {
			return "{ `` | " + modTail + " }"
		}
		// L3/L4 — leave it as it is
		return tok
	})

	// 2) Pure {name}
	for _, name := range meta.names {
		// Clean is exactly { name } without a pipe
		reExact := regexp.MustCompile(`\{[ \t]*` + regexp.QuoteMeta(name) + `[ \t]*\}`)
		if valAny, ok := data[name]; ok {
			val := fmt.Sprint(valAny)
			out = reExact.ReplaceAllString(out, val)
			continue
		}
		// L2 — if the field is in union → put ""
		if _, seen := union[name]; seen {
			out = reExact.ReplaceAllString(out, "")
		}
		// otherwise L3/L4 – leave it as it is
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

// replacePerc - Replaces %[N]s in a string from arr (1-basis indexing).
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
	names       []string // names {name} before first | or }
	percentSeen int      // number of met %[N]s
}

var (
	// Named tags: {fio}, {dep.team}, {fio|...}, {dep.team | ...}
	reBraceName = regexp.MustCompile(`\{[ \t]*([A-Za-z0-9_.]+)[ \t]*[|}]`)
	// Positional formatting in a string pattern: %[N]s
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
	// Primary case: {"group": {...}} или {"group": []}
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

	// fallback: flat map (we'll treat it as a one-time map-item without an explicit groupKey)
	if m, ok := v.(map[string]any); ok {
		// If there are slices in the values, do not count map-item
		for _, vv := range m {
			switch vv.(type) {
			case []any, []string:
				return normItem{raw: v, kind: "other"}
			}
		}
		return normItem{raw: v, kind: "map", mapVal: m}
	}

	// slices without a wrapper — let's count the positional item
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
// XML-escape hook (disabled by default — we pass raw to <w:t>, and string mods through Go-templates)
func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, `'`, "&apos;")
	return s
}
*/
