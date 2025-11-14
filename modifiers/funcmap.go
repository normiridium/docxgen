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

// RawXML is a type for raw XML inserts that do not need to be escaped.
type RawXML string

// Options sets the parameters for building the FuncMap.
type Options struct {
	// Fonts is a set of fonts for p_split. If nil, don't connect p_split.
	Fonts *metrics.FontSet
	// Data — template input data (needed for concat to be able to pick up other tags by name).
	Data map[string]any
	// ExtraFuncs are custom modifiers with a number of fixed parameters.
	// The behavior is completely similar to builtins.
	ExtraFuncs map[string]ModifierMeta
}

// For Word to display the tab correctly, you need to close the previous text element.
// Therefore, with several tabs in a row, empty <w:t></w:t> appear, but in Word they are
// still display correctly and look better than any other options.
const (
	TAB     = "</w:t><w:tab/><w:t>"
	NEWLINE = "<w:br/>"
)

// wordReplacer - Performs post-processing of the result xml.Encoder:
// - removes the <string>...</string> wrapper that xml.Encoder adds to strings;
// - Replaces control characters with Word-compatible tags (<w:br/>, <w:tab/>).
var wordReplacer = strings.NewReplacer(
	"<string>", "",
	"</string>", "",
	"&#xD;&#xA;", NEWLINE, // Windows-перенос \r\n
	"&#xD;", NEWLINE, // старые Mac-переносы \r
	"&#xA;", NEWLINE, // Unix/Linux/macOS переносы \n
	"&#x9;", TAB, // табуляция \t
)

// escapeForWord - Prepares a string to be inserted into the document.xml.
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

// ---- Register of modifiers ----

type ModifierMeta struct {
	Func  any // target function (with a "beautiful" signature)
	Count int // how many FIRST arguments are considered fixed; pipeline-value is always the last
}

// Built-in modifiers.
// Count is the number of "fixed" modifier parameters (preceded by pipeline-value in the template).
// WrapModifier itself will decompose and call a function in the signature of the form: fn(value, fixed..., formats...)
var builtins = map[string]ModifierMeta{
	// string mods
	"prefix":       {Func: Prefix, Count: 1},
	"uniq_prefix":  {Func: UniqPrefix, Count: 1},
	"postfix":      {Func: Postfix, Count: 1},
	"uniq_postfix": {Func: UniqPostfix, Count: 1},
	"default":      {Func: DefaultValue, Count: 1},
	"filled":       {Func: Filled, Count: 1},
	"replace":      {Func: Replace, Count: 2},
	"truncate":     {Func: Truncate, Count: 2},
	"word_reverse": {Func: WordReverse, Count: 0},
	"br":           {Func: NewLine, Count: 0},
	"nl":           {Func: NewLine, Count: 0},

	// text mods
	"nowrap":   {Func: Nowrap, Count: 0},
	"compact":  {Func: Compact, Count: 0},
	"abbr":     {Func: Abbr, Count: 0},
	"ru_phone": {Func: RuPhone, Count: 0},

	// numeric mods
	"numeral":   {Func: Numeral, Count: 0},
	"plural":    {Func: Plural, Count: 0},
	"sign":      {Func: Sign, Count: 0},
	"pad_left":  {Func: PadLeft, Count: 2},
	"pad_right": {Func: PadRight, Count: 2},
	"money":     {Func: Money, Count: 1},
	"roman":     {Func: Roman, Count: 0},

	// declension mods
	"decl":       {Func: Declension, Count: 1},
	"declension": {Func: Declension, Count: 1},

	// date mods
	"date_format": {Func: DateFormat, Count: 1},

	// qrcode mod
	"qrcode":  {Func: QrCode, Count: 0},
	"barcode": {Func: BarCode, Count: 0},
}

// NewFuncMap returns a function map for Go templates.
// Plugs in the standard modifiers and drains the ExtraFuncs on top.
func NewFuncMap(opts Options) template.FuncMap {
	fm := template.FuncMap{}

	// Registering builtins taking into account the number of fixed parameters
	for name, meta := range builtins {
		fm[name] = WrapModifier(meta.Func, meta.Count)
	}

	// concat is special: you need access to opts. Data; signature: func(base string, parts ... string) string
	//	In the template: {base|concat:'x':'y':', '}
	//	Here Count=0: all parameters are considered "formats", they come after value.
	fm["concat"] = WrapModifier(ConcatFactory(opts.Data), 0)

	// p_split include if there are fonts.
	//	Closure signature: func(text string, firstUnders, otherUnders, nLine any, extra ... any) string
	//	In the template: {text|p_split:20:65:2} or {text|p_split:20:65:+2:'bold':12}
	//	Here, Count=3 (firstUnders, otherUnders, nLine) — extra will go as variadic after them.
	if opts.Fonts != nil {
		fm["p_split"] = WrapModifier(MakePSplit(opts.Fonts), 3)
	}

	// Merge custom modifiers (full DSL participants)
	if opts.ExtraFuncs != nil {
		for k, meta := range opts.ExtraFuncs {
			fm[k] = WrapModifier(meta.Func, meta.Count)
		}
	}

	return fm
}

// -----------------AUXILIARY-----------------
//
// splitArgs — decomposes args from a template according to DSL conventions.
// args = [fixed1, fixed2, ..., fixedN, formats..., value]
// Count = N.
// Returns:
//
// values — exactly the Count of the first parameters (if there are fewer of them, turn on mode B: soft return of value without changes);
//
//	formats — all intermediate ones;
//	value is the last (pipeline) argument.
//
// Strategy B (for the library): if there are not enough arguments, return the original value unchanged.
func splitArgs(countFirst int, args []any) (values []any, formats []any, value any) {
	n := len(args)
	if n == 0 {
		return nil, nil, nil
	}

	value = args[n-1]

	// normalize countFirst: minimum 0
	if countFirst < 0 {
		countFirst = 0
	}

	// if there are not enough parameters for fixed ones, gently exit (B)
	if n-1 < countFirst {
		return nil, nil, value
	}

	// fixed
	if countFirst > 0 {
		values = args[:countFirst]
	}

	// formats — everything between fixed and value
	startFormats := countFirst
	endFormats := n - 1
	if startFormats < endFormats {
		formats = args[startFormats:endFormats]
	}

	return
}

// WrapModifier is a single wrapper for a modifier call.
// Parses arguments according to the DSL rule: the first "fixed" is fixed, the last is pipeline-value,
// everything in between is "formats". Then calls the target function in the form:
//
// fn(value, fixed..., formats...)
//
// Supports variadics.
func WrapModifier(fn any, fixed int) any {
	return func(args ...any) any {
		values, formats, value := splitArgs(fixed, args)

		fnVal := reflect.ValueOf(fn)
		fnType := fnVal.Type()
		if fnType.Kind() != reflect.Func {
			// не функция — безопасно вернуть pipeline как есть
			return value
		}

		// How many parameters does a function have?
		numIn := fnType.NumIn()
		isVariadic := fnType.IsVariadic()

		// How many non-variadic parameters are expected?
		nonVarCount := numIn
		if isVariadic {
			nonVarCount = numIn - 1
		}

		// Assembling a list of final DSL-level arguments: value, fixed..., formats...
		final := make([]any, 0, 1+len(values)+len(formats))
		final = append(final, value)
		final = append(final, values...)
		final = append(final, formats...)

		// If there are fewer finite arguments than the non-variadic function expects, softly return value (B).
		if len(final) < nonVarCount {
			return value
		}

		callArgs := make([]reflect.Value, 0, numIn)

		// Type casting for non-variadic parameters
		for i := 0; i < nonVarCount; i++ {
			paramT := fnType.In(i)
			argV := toReflectValue(final[i], paramT)
			callArgs = append(callArgs, argV)
		}

		// Variadic part (if required)
		if isVariadic {
			// The expected type of the last parameter is slice
			variadicSliceT := fnType.In(numIn - 1)
			elemT := variadicSliceT.Elem()

			// Assemble the formats (final remainder) into a slice of the desired type
			variadicCount := len(final) - nonVarCount
			sliceV := reflect.MakeSlice(variadicSliceT, variadicCount, variadicCount)
			for i := 0; i < variadicCount; i++ {
				elemV := toReflectValue(final[nonVarCount+i], elemT)
				sliceV.Index(i).Set(elemV)
			}
			callArgs = append(callArgs, sliceV)

			// Calling CallSlice for Variadics
			out := fnVal.CallSlice(callArgs)
			return normalizeReturn(out)
		}

		// If you don't have a variadic, ignore unnecessary arguments
		out := fnVal.Call(callArgs)
		return normalizeReturn(out)
	}
}

// normalizeReturn - Normalizes the return values of the modifier:
// - one string → escaped under Word
// - Any one → as is
// - multiple → []any
func normalizeReturn(out []reflect.Value) any {
	// if the modifier returned RawXML, paste it as it is
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

// toReflectValue - Gently casts the value to the desired function parameter type.
// Strategy: Assignable → Convertible → special cases (string, int) → zero value.
func toReflectValue(v any, target reflect.Type) reflect.Value {
	// nil → zero
	if v == nil {
		return reflect.Zero(target)
	}

	rv := reflect.ValueOf(v)
	rt := rv.Type()

	// If already assignable, you're done
	if rt.AssignableTo(target) {
		return rv
	}

	// If convertible, convert
	if rt.ConvertibleTo(target) {
		return rv.Convert(target)
	}

	// Frequent convenient ghosts:
	switch target.Kind() {
	case reflect.Interface:
		// Any value fits interface{}
		return rv

	case reflect.String:
		// Everything can be turned into a string
		return reflect.ValueOf(fmt.Sprint(v))

	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		// A special case: a line came — let's try atoi
		if s, ok := v.(string); ok {
			if n, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64); err == nil {
				x := reflect.New(target).Elem()
				x.SetInt(n)
				return x
			}
		}
		// If it's a number, but of a different sign type, let's try to convert it via fmt → atoi
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
		// Unsupported type – just return zero value
		return reflect.Zero(target)
	}

	// If you can't, give zero value of the desired type (strategy B – don't panic)
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
