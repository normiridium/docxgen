package tests

import (
	"testing"

	"docxgen/modifiers"
)

// проброс функций
var (
	Nowrap  = modifiers.Nowrap
	Compact = modifiers.Compact
	Abbr    = modifiers.Abbr
	Phone   = modifiers.RuPhone

	NBSP  = modifiers.NBSP
	NNBSP = modifiers.NNBSP
)

func TestNowrap(t *testing.T) {
	in := "Дело № 15"
	got := Nowrap(in)
	want := "Дело" + NBSP + "№" + NBSP + "15"
	if got != want {
		t.Errorf("Nowrap() = %q, want %q", got, want)
	}

	// не должен менять пустую строку
	if got := Nowrap(""); got != "" {
		t.Errorf("Nowrap() empty = %q, want empty string", got)
	}
}

func TestCompact(t *testing.T) {
	in := "+7 (4912) 572 466"
	got := Compact(in)
	want := "+7" + NNBSP + "(4912)" + NNBSP + "572" + NNBSP + "466"
	if got != want {
		t.Errorf("Compact() = %q, want %q", got, want)
	}
}

func TestAbbr(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"г. Москва", "г." + NBSP + "Москва"},
		{"ул. Ленина", "ул." + NBSP + "Ленина"},
		{"ООО Центр", "ООО" + NBSP + "Центр"},
		{"И. И. Иванов", "И." + NBSP + "И." + NBSP + "Иванов"},
		{"И.И. Иванов", "И.И." + NBSP + "Иванов"},
	}

	for _, tt := range tests {
		got := Abbr(tt.in)
		if got != tt.want {
			t.Errorf("Abbr(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestPhone_All(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		// --- мобильные (3 цифры после 8 или +7) ---
		{"Mobile plain", "89101234567", "+7 (910) 123-45-67"},
		{"Mobile spaced", "8 910 123 45 67", "+7 (910) 123-45-67"},
		{"Mobile with +7", "+7(916)1234567", "+7 (916) 123-45-67"},
		{"Mobile dashed", "8-999-888-77-66", "+7 (999) 888-77-66"},
		{"Mobile weird format", "7(927)888 55 33", "+7 (927) 888-55-33"},

		// --- региональные (4 цифры после 8 или +7, не начинаются с 9) ---
		{"Regional plain", "84912572466", "+7 (4912) 572-466"},
		{"Regional spaced", "8 4912 572 466", "+7 (4912) 572-466"},
		{"Regional with +7", "+7(4912)572466", "+7 (4912) 572-466"},
		{"Regional dashed", "8-4852-123-456", "+7 (4852) 123-456"},
		{"Regional weird format", "7(812)5553344", "+7 (812) 555-33-44"},

		// --- другие / не должны трогаться ---
		{"Too short", "12345", "12345"},
		{"Too long", "84991234567890", "84991234567890"},
		{"Foreign number", "+1 (555) 123-4567", "+1 (555) 123-4567"},
		{"Empty string", "", ""},
		{"Text inside", "тел. 89101234567 доб. 101", "тел. +7 (910) 123-45-67 доб. 101"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Phone(tt.input)
			if got != tt.want {
				t.Errorf("Phone(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
