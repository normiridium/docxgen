package tests

import (
	"bytes"
	"docxgen/modifiers"
	"strings"
	"testing"
	"text/template"
)

func TestCascadeModifiers(t *testing.T) {
	data := map[string]any{
		"Fio":     "Иванов Иван Иванович",
		"Title":   "Очень длинное название для проверки каскадных модификаторов",
		"Phone":   "89101234567",
		"Count":   5,
		"Address": "Адрес",
	}

	tmpl := `
1) {{ .Fio | decl "родительный" "и.о. фамилия" }}
2) {{ .Title | truncate 10 "…" }}
3) {{ .Phone | ru_phone | compact }}
4) {{ .Fio | prefix "гражданин " | postfix " (подп.)" }}
5) {{ .Fio | replace "Иванов" "Петров" | decl "винительный" "фамилия" }}
6) {{ .Count }} {{ .Count | plural "день" "дня" "дней" }}
7) {{ .Address | concat "г. Москва" "ул. Ленина" ", " }}
8) {{ .Fio | decl "предложный" "ф и о" | word_reverse | prefix "[" | postfix "]" }}
`

	fm := modifiers.NewFuncMap(modifiers.Options{Data: data})
	tpl, err := template.New("cascade").Funcs(fm).Parse(tmpl)
	if err != nil {
		t.Fatalf("template parse error: %v", err)
	}

	var buf bytes.Buffer
	if err := tpl.Execute(&buf, data); err != nil {
		t.Fatalf("template execute error: %v", err)
	}

	result := strings.Split(strings.TrimSpace(buf.String()), "\n")

	want := []string{
		"1) И.И. Иванова",
		"2) Очень длин…",
		"3) +7\u202f(910)\u202f123-45-67",
		"4) гражданин Иванов Иван Иванович (подп.)",
		"5) Петрова",
		"6) 5 дней",
		"7) Адрес, г. Москва, ул. Ленина",
		"8) [Ивановиче Иване Иванове]",
	}

	if len(want) != len(result) {
		t.Fatalf("expected %d lines, got %d:\n%s", len(want), len(result), strings.Join(result, "\n"))
	}

	for i := range want {
		got := strings.TrimSpace(result[i])
		if got != want[i] {
			t.Errorf("line %d mismatch:\n got:  %q\n want: %q", i+1, got, want[i])
		}
	}
}
