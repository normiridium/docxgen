package tests

import (
	"docxgen/modifiers"
	"testing"
)

// Проброс всех тестируемых функций
var (
	Prefix        = modifiers.Prefix
	UniqPrefix    = modifiers.UniqPrefix
	Postfix       = modifiers.Postfix
	UniqPostfix   = modifiers.UniqPostfix
	DefaultValue  = modifiers.DefaultValue
	Filled        = modifiers.Filled
	Replace       = modifiers.Replace
	Truncate      = modifiers.Truncate
	WordReverse   = modifiers.WordReverse
	ConcatFactory = modifiers.ConcatFactory
)

func TestPrefix(t *testing.T) {
	got := Prefix("Иванов", "гражданин ")
	want := "гражданин Иванов"
	if got != want {
		t.Errorf("Prefix() = %q, want %q", got, want)
	}

	got = Prefix("", "гражданин ")
	if got != "" {
		t.Errorf("Prefix() empty = %q, want empty string", got)
	}
}

func TestUniqPrefix(t *testing.T) {
	got := UniqPrefix("ООО Ромашка", "ООО ")
	if got != "ООО Ромашка" {
		t.Errorf("UniqPrefix() duplicate = %q, want unchanged", got)
	}

	got = UniqPrefix("Ромашка", "ООО ")
	want := "ООО Ромашка"
	if got != want {
		t.Errorf("UniqPrefix() = %q, want %q", got, want)
	}
}

func TestPostfix(t *testing.T) {
	got := Postfix("100", " руб.")
	want := "100 руб."
	if got != want {
		t.Errorf("Postfix() = %q, want %q", got, want)
	}

	got = Postfix("", " руб.")
	if got != "" {
		t.Errorf("Postfix() empty = %q, want empty string", got)
	}
}

func TestUniqPostfix(t *testing.T) {
	got := UniqPostfix("Москва г.", " г.")
	if got != "Москва г." {
		t.Errorf("UniqPostfix() duplicate = %q, want unchanged", got)
	}

	got = UniqPostfix("Москва", " г.")
	want := "Москва г."
	if got != want {
		t.Errorf("UniqPostfix() = %q, want %q", got, want)
	}
}

func TestDefaultValue(t *testing.T) {
	got := DefaultValue("", "сотрудник")
	want := "сотрудник"
	if got != want {
		t.Errorf("DefaultValue() = %q, want %q", got, want)
	}

	got = DefaultValue("менеджер", "сотрудник")
	want = "менеджер"
	if got != want {
		t.Errorf("DefaultValue() non-empty = %q, want %q", got, want)
	}
}

func TestFilled(t *testing.T) {
	if got := Filled("", "—"); got != "" {
		t.Errorf("Filled() empty string = %q, want empty", got)
	}
	if got := Filled("123", "—"); got != "—" {
		t.Errorf("Filled() non-empty = %q, want —", got)
	}
	if got := Filled(nil, "—"); got != "" {
		t.Errorf("Filled() nil = %q, want empty", got)
	}
}

func TestReplace(t *testing.T) {
	got := Replace("Москва и Московская область", "Москва", "СПб")
	want := "СПб и Московская область"
	if got != want {
		t.Errorf("Replace() = %q, want %q", got, want)
	}
}

func TestTruncate(t *testing.T) {
	got := Truncate("Очень длинное название", 10, "…")
	want := "Очень длин…"
	if got != want {
		t.Errorf("Truncate() = %q, want %q", got, want)
	}

	got = Truncate("Коротко", 10, "…")
	want = "Коротко"
	if got != want {
		t.Errorf("Truncate() short = %q, want %q", got, want)
	}
}

func TestWordReverse(t *testing.T) {
	got := WordReverse("раз два три четыре")
	want := "четыре три два раз"
	if got != want {
		t.Errorf("WordReverse() = %q, want %q", got, want)
	}

	got = WordReverse("Фамилия Имя Отчество")
	want = "Отчество Имя Фамилия"
	if got != want {
		t.Errorf("WordReverse() fio = %q, want %q", got, want)
	}
}

func TestConcatFactory(t *testing.T) {
	data := map[string]any{
		"org":    "ООО Ромашка",
		"city":   "Москва",
		"street": "Ленина",
	}
	concat := ConcatFactory(data)

	got := concat("Адрес", "org", "city", "street", ", ")
	want := "Адрес, ООО Ромашка, Москва, Ленина"
	if got != want {
		t.Errorf("ConcatFactory() = %q, want %q", got, want)
	}
}
