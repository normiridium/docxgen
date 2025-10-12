package tests

import (
	"bytes"
	"testing"
	"text/template"

	"docxgen/modifiers"
)

func TestFuncMap_Concat(t *testing.T) {
	data := map[string]any{
		"first":  "Адрес 1",
		"second": " ",
		"third":  "Адрес 3",
	}
	fm := modifiers.NewFuncMap(modifiers.Options{Data: data})
	tmpl, err := template.New("concat").Funcs(fm).Parse(`{{ .first | concat "second" "third" "; " }}`)
	if err != nil {
		t.Fatalf("parse template: %v", err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		t.Fatalf("execute: %v", err)
	}

	got := buf.String()
	want := "Адрес 1; Адрес 3"
	if got != want {
		t.Errorf("concat with space:\n got  %q\n want %q", got, want)
	}
}

// TestFuncMap_ModifiersWork проверяет работу всех функций из FuncMap через шаблон
func TestFuncMap_ModifiersWork(t *testing.T) {
	fm := modifiers.NewFuncMap(modifiers.Options{})

	cases := []struct {
		name     string
		template string
		want     string
	}{
		// базовые функции
		{"prefix", `{{ "слово" | prefix "до-" }}`, "до-слово"},
		{"postfix", `{{ "слово" | postfix "-после" }}`, "слово-после"},
		{"default when empty", `{{ "" | default "нет" }}`, "нет"},
		{"default when filled", `{{ "значение" | default "нет" }}`, "значение"},
		{"replace simple", `{{ "abc" | replace "a" "z" }}`, "zbc"},
		{"truncate short", `{{ "123456" | truncate 3 "..." }}`, "123..."},
		{"truncate long", `{{ "12" | truncate 5 "..." }}`, "12"},
		{"word_reverse", `{{ "Иванов Иван Иванович" | word_reverse }}`, "Иванович Иван Иванов"},
		{"filled true", `{{ "что-то" | filled "ok" }}`, "ok"},
		{"filled false", `{{ "" | filled "ok" }}`, ""},

		// уникальные приставки / постфиксы
		{"uniq_prefix", `{{ "Б" | uniq_prefix "п." }}`, "п.Б"},
		{"uniq_prefix already exists", `{{ "п.Б" | uniq_prefix "п." }}`, "п.Б"},
		{"uniq_postfix", `{{ "Б" | uniq_postfix " Перечня" }}`, "Б Перечня"},
		{"uniq_postfix already exists", `{{ "Б Перечня" | uniq_postfix " Перечня" }}`, "Б Перечня"},

		// краевые случаи
		{"truncate empty", `{{ "" | truncate 10 "..." }}`, ""},
		{"replace no match", `{{ "abc" | replace "x" "y" }}`, "abc"},
		{"prefix empty", `{{ "" | prefix ">" }}`, ""},
		{"postfix empty", `{{ "" | postfix "<" }}`, ""},
		{"default whitespace", `{{ " " | default "нет" }}`, "нет"},

		// numeric mods
		{"numeral_default", `{{ 8 | numeral }}`, "восемь"},
		{"numeral_fem_prep", `{{ 2 | numeral "женский" "предложный" }}`, "двух"},
		{"numeral_dat", `{{ 35147 | numeral "дательный" }}`, "тридцати пяти тысячам ста сорока семи"},
		{"numeral_alt8", `{{ 8 | numeral "творительный" "восемью" }}`, "восемью"},
		{"numeral_nol", `{{ 0 | numeral "ноль" }}`, "ноль"},
		{"numeral_nul", `{{ 0 | numeral "нуль" }}`, "нуль"},
		{"numeral_fem_gen", `{{ 2 | numeral "женский" "родительный" }}`, "двух"},

		{"plural simple", `{{ 3 | plural "день" "дня" "дней" }}`, "дня"},
		{"plural five", `{{ 5 | plural "день" "дня" "дней" }}`, "дней"},
		{"plural one", `{{ 1 | plural "день" "дня" "дней" }}`, "день"},
		{"sign positive", `{{ 5 | sign }}`, "+5"},
		{"sign negative", `{{ -3 | sign }}`, "-3"},
		{"sign zero", `{{ 0 | sign }}`, "0"},
		{"pad_left", `{{ 42 | pad_left 5 "0" }}`, "00042"},
		{"pad_right", `{{ 42 | pad_right 5 "x" }}`, "42xxx"},
		{"money int", `{{ 123456 | money }}`, "123 456,00"},
		{"money float", `{{ 1234.5 | money }}`, "1 234,50"},
		{"money spaced", `{{ "12000.05" | money }}`, "12 000,05"},
		{"roman simple", `{{ 4 | roman }}`, "IV"},
		{"roman big", `{{ 2024 | roman }}`, "MMXXIV"},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			tmpl, err := template.New("test").Funcs(fm).Parse(c.template)
			if err != nil {
				t.Fatalf("parse template: %v", err)
			}

			var buf bytes.Buffer
			if err := tmpl.Execute(&buf, nil); err != nil {
				t.Fatalf("execute: %v", err)
			}

			if buf.String() != c.want {
				t.Errorf("%s:\n got  %q\n want %q", c.name, buf.String(), c.want)
			}
		})
	}
}
