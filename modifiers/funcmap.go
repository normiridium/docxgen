package modifiers

import (
	"bufio"
	"bytes"
	"docxgen/metrics"
	"encoding/xml"
	"reflect"
	"strings"
	"text/template"
)

// Options задаёт параметры построения FuncMap.
type Options struct {
	// Fonts — набор шрифтов для p_split. Если nil, p_split не подключаем.
	Fonts *metrics.FontSet
	// Data — входные данные шаблона (нужны для concat, чтобы уметь подцеплять другие теги по имени).
	Data map[string]any
	// ExtraFuncs — пользовательские функции, которые будут добавлены/переопределены.
	ExtraFuncs template.FuncMap
}

// Чтобы Word корректно отображал табуляцию, нужно закрывать предыдущий текстовый элемент.
// Поэтому при нескольких подряд табах появляются пустые <w:t></w:t>, но в Word они
// всё равно отображаются правильно и выглядят лучше, чем любые другие варианты.
const (
	TAB     = "</w:t><w:tab/><w:t>"
	NEWLINE = "<w:br/>"
)

// wordReplacer — выполняет постобработку результата xml.Encoder:
// - убирает обёртку <string>...</string>, которую xml.Encoder добавляет для строк;
// - заменяет управляющие символы на совместимые с Word теги (<w:br/>, <w:tab/>).
var wordReplacer = strings.NewReplacer(
	"<string>", "",
	"</string>", "",
	"&#xD;&#xA;", NEWLINE, // Windows-перенос \r\n
	"&#xD;", NEWLINE, // старые Mac-переносы \r
	"&#xA;", NEWLINE, // Unix/Linux/macOS переносы \n
	"&#x9;", TAB, // табуляция \t
)

// escapeForWord — готовит строку для вставки в document.xml.
func escapeForWord(s string) (string, error) {
	var b bytes.Buffer
	w := bufio.NewWriter(&b)
	enc := xml.NewEncoder(w)
	if err := enc.Encode(s); err != nil {
		return s, err
	}
	_ = w.Flush()
	return wordReplacer.Replace(b.String()), nil
}

// NewFuncMap возвращает карту функций для Go-шаблонов.
// Подключает стандартные модификаторы и сливает ExtraFuncs поверх.
func NewFuncMap(opts Options) template.FuncMap {
	fm := template.FuncMap{
		// string mods
		"prefix":       Prefix,       // {tag|prefix:`с `}
		"uniq_prefix":  UniqPrefix,   // {tag|uniq_prefix:`п. `}
		"postfix":      Postfix,      // {tag|postfix:`)`} (часто парно с prefix)
		"uniq_postfix": UniqPostfix,  // {tag|uniq_postfix:` Перечня`}
		"default":      DefaultValue, // {tag|default:`не указано`}
		"filled":       Filled,       // {tag|filled:`есть значение`}
		"replace":      Replace,      // {tag|replace:`исх.`:`экз.`}
		"truncate":     Truncate,     // {tag|truncate:250:`...`}
		"word_reverse": WordReverse,  // {fio|reverse}
	}

	// concat, который может брать значения других переменных из opts.Data
	fm["concat"] = ConcatFactory(opts.Data)

	// p_split подключаем, если есть шрифты
	if opts.Fonts != nil {
		fm["p_split"] = MakePSplit(opts.Fonts)
	}

	// Мержим пользовательские функции поверх (могут переопределить стандартные).
	if opts.ExtraFuncs != nil {
		for k, v := range opts.ExtraFuncs {
			fm[k] = v
		}
	}

	for k, v := range fm {
		fm[k] = wrapStrings(v)
	}

	return fm
}

func wrapStrings(f any) any {
	return func(args ...any) any {
		if len(args) > 1 {
			// переносим последний аргумент (pipeline) в начало
			reordered := make([]any, 0, len(args))
			reordered = append(reordered, args[len(args)-1])
			reordered = append(reordered, args[:len(args)-1]...)
			args = reordered
		}

		// вызов через reflect
		vals := make([]reflect.Value, len(args))
		for i, a := range args {
			vals[i] = reflect.ValueOf(a)
		}
		out := reflect.ValueOf(f).Call(vals)

		results := make([]any, len(out))
		for i, v := range out {
			if v.Kind() == reflect.String {
				if safe, err := escapeForWord(v.String()); err == nil {
					results[i] = safe
				} else {
					results[i] = v.String()
				}
			} else {
				results[i] = v.Interface()
			}
		}

		if len(results) == 1 {
			return results[0]
		}
		return results
	}
}
