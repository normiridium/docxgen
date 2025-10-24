package docxgen

import (
	"fmt"
	"regexp"
	"strings"
)

// ============================================================================
// Public API
// ============================================================================

// RenderSmartTable — DOCX-driven генерация:
// • идём по строкам DOCX сверху вниз;
// • positional (%[N]s, в т.ч. внутри бэктиков) — берёт РОВНО ОДИН следующий slice-item;
// • named ({name} без модификаторов) — реплицируется, пока есть подходящие map-items (есть хоть один ключ из тэгов строки);
// • строка с только "неизвестными" тегами (ни один тег не встречается в items) — считается статичной и выводится;
// • статичные строки всегда выводятся;
// • map НИКОГДА не подставляем в %[N]s; slice НИКОГДА не лезет в {name}.
func RenderSmartTable(tableXML string, items []any) (string, error) {
	inner := stripOuterTable(tableXML)
	rows := extractTableRows(inner)
	if len(rows) == 0 {
		return "", fmt.Errorf("smart table: no rows found")
	}

	// Разделяем items на очереди
	mapQueue := []map[string]any{}
	sliceQueue := [][]any{}
	knownKeys := make(map[string]struct{}) // для определения "неизвестных" named-строк как статичных

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
		// прочие типы игнорируем
	}

	var outRows []string

	// Идём по строкам DOCX
	for _, rowXML := range rows {
		meta := parseTplMeta(rowXML)

		switch {
		// ======= POSITIONAL (доминирует, даже если внутри фигурных скобок) =======
		case meta.percentSeen > 0:
			// РОВНО ОДИН следующий slice-item (если есть)
			if len(sliceQueue) > 0 {
				arr := sliceQueue[0]
				sliceQueue = sliceQueue[1:]
				outRows = append(outRows, renderPositional(rowXML, arr, meta.percentSeen))
			}
			// иначе — пропуск строки

		// ======= NAMED (чистые {name}) =======
		case len(meta.names) > 0:
			// Если НИ ОДНОГО тега этой строки нет в knownKeys — это статичная строка (глобальная разметка)
			if !metaHasAnyKnown(meta, knownKeys) {
				outRows = append(outRows, rowXML)
				continue
			}
			// Иначе — это шаблон: реплицируем ПОКА есть подходящие map-items
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
// Matching / Rendering
// ============================================================================

// среди meta.names есть ли ключ, встречающийся в items вообще
func metaHasAnyKnown(meta tplMeta, known map[string]struct{}) bool {
	for _, n := range meta.names {
		if _, ok := known[n]; ok {
			return true
		}
	}
	return false
}

// первый map, у которого есть хотя бы один тег из meta.names
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

func renderNamed(xmlTpl string, meta tplMeta, data map[string]any) string {
	out := xmlTpl
	// Подставляем ТОЛЬКО существующие ключи; остальные {unknown} остаются как есть.
	for _, name := range meta.names {
		if val, ok := data[name]; ok {
			out = strings.ReplaceAll(out, "{"+name+"}", xmlEscape(fmt.Sprint(val)))
		}
	}
	return out
}

// need — сколько %[N]s найдено в строке
func renderPositional(xmlTpl string, arr []any, need int) string {
	// добиваем пустыми строками, чтобы не получить BADINDEX
	if len(arr) < need {
		padded := make([]any, need)
		copy(padded, arr)
		for i := len(arr); i < need; i++ {
			padded[i] = ""
		}
		arr = padded
	}
	args := make([]any, len(arr))
	copy(args, arr)
	return fmt.Sprintf(xmlTpl, args...)
}

// ============================================================================
// Template Meta
// ============================================================================

type tplMeta struct {
	names       []string // {name}, только "чистые" имена без пайпов/бэктиков
	percentSeen int      // количество %[N]s в строке (доминирующий признак позиционного шаблона)
}

var (
	// Матчит только чистые {name} без модификаторов/бэктиков: {fio}, {pos}, {title}, {dep.team}
	reBrace = regexp.MustCompile(`\{[ \t]*([A-Za-z0-9_\.]+)[ \t]*\}`)
	// Матчит %[N]s (включая случаи внутри бэктиков), чтобы строка считалась positional при наличии любого %[]
	rePerc = regexp.MustCompile(`%\[\s*(\d+)\s*\]s`)
)

func parseTplMeta(rowXML string) tplMeta {
	meta := tplMeta{}
	// IMPORTANT: сначала считаем позиционные — это доминирующий признак строки
	meta.percentSeen = len(rePerc.FindAllStringSubmatch(rowXML, -1))
	// затем чистые {name}; backticks/pipe не пройдут, и это хорошо
	for _, m := range reBrace.FindAllStringSubmatch(rowXML, -1) {
		if len(m) >= 2 {
			name := strings.TrimSpace(m[1])
			if name != "" {
				meta.names = append(meta.names, name)
			}
		}
	}
	return meta
}

// ============================================================================
// Data Extractors
// ============================================================================

// Предпочитает формат { anyKey: { ... } }, но допускает плоский map как inner-map,
// если в значениях НЕТ массивов (чтобы не спутать со slice-item).
func extractInnerMapAny(it any) (map[string]any, bool) {
	if outer, ok := it.(map[string]any); ok {
		if len(outer) == 1 {
			for _, v := range outer {
				if inner, ok := v.(map[string]any); ok {
					return inner, true
				}
			}
		}
		// Fallback: если в значениях нет массивов — считаем плоским map
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
// XML helpers / Rows
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

func xmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, `'`, "&apos;")
	return s
}
