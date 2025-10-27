package modifiers

import (
	"bufio"
	"bytes"
	"docxgen/metrics"
	"encoding/xml"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"text/template"
)

// RawXML — тип для "сырых" XML-вставок, которые не нужно экранировать.
type RawXML string

// Options задаёт параметры построения FuncMap.
type Options struct {
	// Fonts — набор шрифтов для p_split. Если nil, p_split не подключаем.
	Fonts *metrics.FontSet
	// Data — входные данные шаблона (нужны для concat, чтобы уметь подцеплять другие теги по имени).
	Data map[string]any
	// ExtraFuncs — пользовательские модификаторы с количеством фиксированных параметров.
	// Поведение полностью аналогично builtins.
	ExtraFuncs map[string]ModifierMeta
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

// ---- Реестр модификаторов ----

type ModifierMeta struct {
	Fn    any // целевая функция (с "красивой" сигнатурой)
	Count int // сколько ПЕРВЫХ аргументов считать фиксированными; pipeline-value всегда последний
}

// Встроенные модификаторы.
// Count — это число "фиксированных" параметров модификатора (идут перед pipeline-value в шаблоне).
// WrapModifier сам разложит и вызовет функцию в сигнатуре вида: fn(value, fixed..., formats...)
var builtins = map[string]ModifierMeta{
	// string mods
	"prefix":       {Fn: Prefix, Count: 1},
	"uniq_prefix":  {Fn: UniqPrefix, Count: 1},
	"postfix":      {Fn: Postfix, Count: 1},
	"uniq_postfix": {Fn: UniqPostfix, Count: 1},
	"default":      {Fn: DefaultValue, Count: 1},
	"filled":       {Fn: Filled, Count: 1},
	"replace":      {Fn: Replace, Count: 2},
	"truncate":     {Fn: Truncate, Count: 2},
	"word_reverse": {Fn: WordReverse, Count: 0},
	"br":           {Fn: NewLine, Count: 0},
	"nl":           {Fn: NewLine, Count: 0},

	// text mods
	"nowrap":   {Fn: Nowrap, Count: 0},
	"compact":  {Fn: Compact, Count: 0},
	"abbr":     {Fn: Abbr, Count: 0},
	"ru_phone": {Fn: RuPhone, Count: 0},

	// numeric mods
	"numeral":   {Fn: Numeral, Count: 0},
	"plural":    {Fn: Plural, Count: 0},
	"sign":      {Fn: Sign, Count: 0},
	"pad_left":  {Fn: PadLeft, Count: 2},
	"pad_right": {Fn: PadRight, Count: 2},
	"money":     {Fn: Money, Count: 1},
	"roman":     {Fn: Roman, Count: 0},

	// declension mods
	"decl":       {Fn: Declension, Count: 1},
	"declension": {Fn: Declension, Count: 1},

	// date mods
	"date_format": {Fn: DateFormat, Count: 1},

	// qrcode mod
	"qrcode": {Fn: QrCode, Count: 0},
}

// NewFuncMap возвращает карту функций для Go-шаблонов.
// Подключает стандартные модификаторы и сливает ExtraFuncs поверх.
func NewFuncMap(opts Options) template.FuncMap {
	fm := template.FuncMap{}

	// Регистрируем builtins с учётом количества фиксированных параметров
	for name, meta := range builtins {
		fm[name] = WrapModifier(meta.Fn, meta.Count)
	}

	// concat — особый: нужен доступ к opts.Data; сигнатура: func(base string, parts ...string) string
	// В шаблоне: {base|concat:`x`:`y`:`, `}
	// Здесь Count=0: все параметры считаем "formats", они идут после value.
	fm["concat"] = WrapModifier(ConcatFactory(opts.Data), 0)

	// p_split подключаем, если есть шрифты.
	// Сигнатура замыкания: func(text string, firstUnders, otherUnders, nLine any, extra ...any) string
	// В шаблоне: {text|p_split:20:65:2} или {text|p_split:20:65:+2:`bold`:12}
	// Здесь Count=3 (firstUnders, otherUnders, nLine) — extra уйдут как variadic после них.
	if opts.Fonts != nil {
		fm["p_split"] = WrapModifier(MakePSplit(opts.Fonts), 3)
	}

	// Мержим пользовательские модификаторы (полноценные участники DSL)
	if opts.ExtraFuncs != nil {
		for k, meta := range opts.ExtraFuncs {
			fm[k] = WrapModifier(meta.Fn, meta.Count)
		}
	}

	return fm
}

// ----------------- ВСПОМОГАТЕЛЬНОЕ -----------------

// splitArgs — раскладывает args из шаблона по договорённостям DSL.
// args = [fixed1, fixed2, ..., fixedN, formats..., value]
// Count = N.
// Возвращает:
//
//	values  — ровно Count первых параметров (если их меньше, включаем режим B: мягкий возврат value без изменений);
//	formats — все промежуточные;
//	value   — последний (pipeline) аргумент.
//
// Стратегия B (для библиотеки): при нехватке аргументов возвращаем исходное value без изменений.
func splitArgs(countFirst int, args []any) (values []any, formats []any, value any) {
	n := len(args)
	if n == 0 {
		return nil, nil, nil
	}

	value = args[n-1]

	// нормализуем countFirst: минимум 0
	if countFirst < 0 {
		countFirst = 0
	}

	// если не хватает параметров для фиксированных — мягко выходим (B)
	if n-1 < countFirst {
		return nil, nil, value
	}

	// фиксированные
	if countFirst > 0 {
		values = args[:countFirst]
	}

	// formats — всё между фиксированными и value
	startFormats := countFirst
	endFormats := n - 1
	if startFormats < endFormats {
		formats = args[startFormats:endFormats]
	}

	return
}

// WrapModifier — единая обёртка вызова модификатора.
// Делает разбор аргументов по правилу DSL: первые "fixed" — фиксированные, последний — pipeline-value,
// всё между ними — "formats". Затем вызывает целевую функцию в виде:
//
//	fn(value, fixed..., formats...)
//
// Поддерживает вариадики.
func WrapModifier(fn any, fixed int) any {
	return func(args ...any) any {
		values, formats, value := splitArgs(fixed, args)

		fnVal := reflect.ValueOf(fn)
		fnType := fnVal.Type()
		if fnType.Kind() != reflect.Func {
			// не функция — безопасно вернуть pipeline как есть
			return value
		}

		// Сколько параметров у функции?
		numIn := fnType.NumIn()
		isVariadic := fnType.IsVariadic()

		// Сколько non-variadic параметров ожидается?
		nonVarCount := numIn
		if isVariadic {
			nonVarCount = numIn - 1
		}

		// Собираем список финальных аргументов DSL-уровня: value, fixed..., formats...
		final := make([]any, 0, 1+len(values)+len(formats))
		final = append(final, value)
		final = append(final, values...)
		final = append(final, formats...)

		// Если конечных аргументов меньше, чем non-variadic ожидает функция — мягко возвращаем value (B).
		if len(final) < nonVarCount {
			return value
		}

		callArgs := make([]reflect.Value, 0, numIn)

		// Приведение типов для non-variadic параметров
		for i := 0; i < nonVarCount; i++ {
			paramT := fnType.In(i)
			argV := toReflectValue(final[i], paramT)
			callArgs = append(callArgs, argV)
		}

		// Variadic-часть (если требуется)
		if isVariadic {
			// ожидаемый тип последнего параметра — слайс
			variadicSliceT := fnType.In(numIn - 1)
			elemT := variadicSliceT.Elem()

			// соберём formats (остаток final) в слайс нужного типа
			variadicCount := len(final) - nonVarCount
			sliceV := reflect.MakeSlice(variadicSliceT, variadicCount, variadicCount)
			for i := 0; i < variadicCount; i++ {
				elemV := toReflectValue(final[nonVarCount+i], elemT)
				sliceV.Index(i).Set(elemV)
			}
			callArgs = append(callArgs, sliceV)

			// Вызов CallSlice для вариадиков
			out := fnVal.CallSlice(callArgs)
			return normalizeReturn(out)
		}

		// Если не вариадик — лишние аргументы игнорируем
		out := fnVal.Call(callArgs)
		return normalizeReturn(out)
	}
}

// normalizeReturn — нормализует возвращаемые значения модификатора:
// - один string → экранируем под Word
// - один любой → как есть
// - несколько → []any
func normalizeReturn(out []reflect.Value) any {
	// если модификатор вернул RawXML — вставляем как есть
	if len(out) == 1 && out[0].IsValid() {
		if raw, ok := out[0].Interface().(RawXML); ok {
			return string(raw)
		}
	}
	if len(out) == 1 && out[0].IsValid() && out[0].Kind() == reflect.String {
		if safe, err := escapeForWord(out[0].String()); err == nil {
			return safe
		}
		return out[0].String()
	}
	if len(out) == 1 {
		return out[0].Interface()
	}
	res := make([]any, len(out))
	for i, v := range out {
		res[i] = v.Interface()
	}
	return res
}

// toReflectValue — аккуратно приводит значение к нужному типу параметра функции.
// Стратегия: Assignable → Convertible → особые случаи (string, int) → zero value.
func toReflectValue(v any, target reflect.Type) reflect.Value {
	// nil → zero
	if v == nil {
		return reflect.Zero(target)
	}

	rv := reflect.ValueOf(v)
	rt := rv.Type()

	// Если уже assignable — готово
	if rt.AssignableTo(target) {
		return rv
	}

	// Если convertible — конвертируем
	if rt.ConvertibleTo(target) {
		return rv.Convert(target)
	}

	// Частые удобные приведения:
	switch target.Kind() {
	case reflect.Interface:
		// Любое значение подходит под interface{}
		return rv

	case reflect.String:
		// Всё можно превратить в строку
		return reflect.ValueOf(fmt.Sprint(v))

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// Частный случай: пришла строка — попробуем atoi
		if s, ok := v.(string); ok {
			if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
				x := reflect.New(target).Elem()
				x.SetInt(n)
				return x
			}
		}
		// Если число, но другого знакового типа — попробуем конвертировать через fmt → atoi
		if isNumeric(rt) {
			s := fmt.Sprint(v)
			if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
				x := reflect.New(target).Elem()
				x.SetInt(n)
				return x
			}
		}

	case reflect.Float32, reflect.Float64:
		if s, ok := v.(string); ok {
			if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
				x := reflect.New(target).Elem()
				x.SetFloat(f)
				return x
			}
		}
		if isNumeric(rt) {
			s := fmt.Sprint(v)
			if f, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err == nil {
				x := reflect.New(target).Elem()
				x.SetFloat(f)
				return x
			}
		}
	default:
		// Неподдержанный тип — просто возвращаем zero value
		return reflect.Zero(target)
	}

	// Не смогли — отдаём zero value нужного типа (стратегия B — не паниковать)
	return reflect.Zero(target)
}

func isNumeric(t reflect.Type) bool {
	switch t.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return true
	default:
		return false
	}
}
