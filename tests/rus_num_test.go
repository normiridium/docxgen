package tests

import (
	"testing"

	rn "docxgen/rusnum"
)

type (
	Option = rn.Option
	Gender = rn.Gender
	Case   = rn.Case
)

var (
	ToWords         = rn.ToWords
	WithCase        = rn.WithCase
	WithGender      = rn.WithGender
	WithNullStyle   = rn.WithNullStyle
	WithInsEightAlt = rn.WithInsEightAlt

	ZeroNol = rn.ZeroNol
	Masc    = rn.Masc
	Fem     = rn.Fem
	Neut    = rn.Neut
	Nom     = rn.Nom
	Gen     = rn.Gen
	Dat     = rn.Dat
	Acc     = rn.Acc
	Ins     = rn.Ins
	Prep    = rn.Prep
)

func TestBasicNumbers(t *testing.T) {
	cases := []struct {
		n    int
		opts []Option
		want string
	}{
		{0, nil, "нуль"},
		{5, nil, "пять"},
		{21, nil, "двадцать один"},
		{1234, nil, "одна тысяча двести тридцать четыре"},
		{-7, nil, "минус семь"},
	}

	for _, c := range cases {
		got := ToWords(c.n, c.opts...)
		if got != c.want {
			t.Errorf("ToWords(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestGender(t *testing.T) {
	got := ToWords(1, WithGender(Fem))
	if got != "одна" {
		t.Errorf("female one = %q, want %q", got, "одна")
	}
	got = ToWords(1, WithGender(Neut))
	if got != "одно" {
		t.Errorf("neuter one = %q, want %q", got, "одно")
	}
}

func TestCases(t *testing.T) {
	type C = Case
	tests := []struct {
		n    int
		c    C
		want string
	}{
		{2, C(Nom), "два"},
		{2, C(Gen), "двух"},
		{2, C(Dat), "двум"},
		{2, C(Acc), "два"},
		{2, C(Ins), "двумя"},
		{2, C(Prep), "двух"},
	}

	for _, tt := range tests {
		got := ToWords(tt.n, WithCase(tt.c))
		if got != tt.want {
			t.Errorf("ToWords(%d,%v)=%q, want %q", tt.n, tt.c, got, tt.want)
		}
	}
}

func TestAltFormEight(t *testing.T) {
	got := ToWords(80, WithCase(Ins), WithInsEightAlt(true))
	want := "восемьюдесятью"
	if got != want {
		t.Errorf("alt 80: got %q, want %q", got, want)
	}
}

func TestThousandsAndMegas(t *testing.T) {
	cases := []struct {
		n    int
		want string
	}{
		{1000, "одна тысяча"},
		{2001, "две тысячи один"},
		{5000000, "пять миллионов"},
		{123456789, "сто двадцать три миллиона четыреста пятьдесят шесть тысяч семьсот восемьдесят девять"},
	}

	for _, c := range cases {
		got := ToWords(c.n)
		if got != c.want {
			t.Errorf("%d: got %q, want %q", c.n, got, c.want)
		}
	}
}

func TestThousandsAndMegas_ByCasesAndGenders(t *testing.T) {
	type G = Gender
	type C = Case

	numbers := []int{1000, 2001, 5000000, 123456789}
	genders := []G{Masc, Fem, Neut}
	cases := []C{Nom, Gen, Dat, Acc, Ins, Prep}

	for _, n := range numbers {
		for _, g := range genders {
			for _, c := range cases {
				got := ToWords(n,
					WithGender(g),
					WithCase(c),
				)
				if got == "" {
					t.Errorf("empty result for n=%d, gender=%v, case=%v", n, g, c)
					continue
				}
				//t.Logf("%d (%v, %v): %s", n, g, c, got)
			}
		}
	}
}

func TestMorphologicallySensitiveNumbers_Strict(t *testing.T) {
	type G = Gender
	type C = Case

	tests := []struct {
		n    int
		g    G
		c    C
		want string
	}{
		// 1 001
		{1001, Masc, Nom, "одна тысяча один"},
		{1001, Fem, Nom, "одна тысяча одна"},
		{1001, Neut, Nom, "одна тысяча одно"},
		{1001, Masc, Gen, "одной тысячи одного"},
		{1001, Fem, Gen, "одной тысячи одной"},
		{1001, Neut, Gen, "одной тысячи одного"},
		{1001, Masc, Dat, "одной тысяче одному"},
		{1001, Fem, Dat, "одной тысяче одной"},
		{1001, Neut, Dat, "одной тысяче одному"},
		{1001, Masc, Ins, "одной тысячей одним"},
		{1001, Fem, Ins, "одной тысячей одной"},
		{1001, Neut, Ins, "одной тысячей одним"},
		{1001, Masc, Prep, "одной тысяче одном"},
		{1001, Fem, Prep, "одной тысяче одной"},
		{1001, Neut, Prep, "одной тысяче одном"},

		// 2 002
		{2002, Masc, Nom, "две тысячи два"},
		{2002, Fem, Nom, "две тысячи две"},
		{2002, Neut, Nom, "две тысячи два"},
		{2002, Masc, Gen, "двух тысяч двух"},
		{2002, Fem, Gen, "двух тысяч двух"},
		{2002, Neut, Gen, "двух тысяч двух"},
		{2002, Masc, Dat, "двум тысячам двум"},
		{2002, Fem, Dat, "двум тысячам двум"},
		{2002, Neut, Dat, "двум тысячам двум"},
		{2002, Masc, Ins, "двумя тысячами двумя"},
		{2002, Fem, Ins, "двумя тысячами двумя"},
		{2002, Neut, Ins, "двумя тысячами двумя"},
		{2002, Masc, Prep, "двух тысячах двух"},
		{2002, Fem, Prep, "двух тысячах двух"},
		{2002, Neut, Prep, "двух тысячах двух"},

		// 21 021
		{21021, Masc, Nom, "двадцать одна тысяча двадцать один"},
		{21021, Fem, Nom, "двадцать одна тысяча двадцать одна"},
		{21021, Neut, Nom, "двадцать одна тысяча двадцать одно"},
		{21021, Masc, Gen, "двадцати одной тысячи двадцати одного"},
		{21021, Fem, Gen, "двадцати одной тысячи двадцати одной"},
		{21021, Neut, Gen, "двадцати одной тысячи двадцати одного"},

		// 3 000 001
		{3000001, Masc, Nom, "три миллиона один"},
		{3000001, Fem, Nom, "три миллиона одна"},
		{3000001, Neut, Nom, "три миллиона одно"},
		{3000001, Masc, Gen, "трёх миллионов одного"},
		{3000001, Fem, Gen, "трёх миллионов одной"},
		{3000001, Neut, Gen, "трёх миллионов одного"},
	}
	for _, tt := range tests {
		got := ToWords(tt.n, WithGender(tt.g), WithCase(tt.c))
		if got != tt.want {
			t.Errorf("n=%d, g=%v, c=%v → %q; want %q", tt.n, tt.g, tt.c, got, tt.want)
		}
	}
}

func TestZeroAndAlt8VariantsAndBigNumbers(t *testing.T) {
	// --- стиль нуля ---
	got := ToWords(0, WithNullStyle(ZeroNol))
	want := "ноль"
	if got != want {
		t.Errorf("ZeroNol: got %q, want %q", got, want)
	}

	// --- стандартная форма 8 ---
	got = ToWords(8, WithCase(Ins), WithInsEightAlt(false))
	want = "восьмью"
	if got != want {
		t.Errorf("std 8: got %q, want %q", got, want)
	}

	// --- альтернативная форма 8 (восемью) ---
	got = ToWords(8, WithCase(Ins), WithInsEightAlt(true))
	want = "восемью"
	if got != want {
		t.Errorf("alt 8: got %q, want %q", got, want)
	}

	// --- стандартная форма 8 (восемьюдесятью) ---
	got = ToWords(80, WithCase(Ins), WithInsEightAlt(false))
	want = "восьмьюдесятью"
	if got != want {
		t.Errorf("alt 80: got %q, want %q", got, want)
	}

	// --- альтернативная форма 8 (восемьюдесятью) ---
	got = ToWords(80, WithCase(Ins), WithInsEightAlt(true))
	want = "восемьюдесятью"
	if got != want {
		t.Errorf("alt 80: got %q, want %q", got, want)
	}

	// --- очень большое число ---
	// просто проверяем, что функция не падает и выдаёт осмысленный текст
	big := 999_999_999_999_999_999
	got = ToWords(big)
	if got == "" {
		t.Errorf("ToWords(%d) returned empty string", big)
	}
	if len(got) < 20 {
		t.Errorf("ToWords(%d) seems too short: %q", big, got)
	}
}
