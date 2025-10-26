package tests

import (
	"docxgen/modifiers"
	"strings"
	"testing"
)

var Declension = modifiers.Declension

// ————————————————————————————————————————————————————————————————
// Тест: базовые склонения ФИО (женские) через petrovich, единый формат
// ————————————————————————————————————————————————————————————————
func TestDeclension_BasicCases_Female(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		caseName string
		format   string
		expected string
	}{
		{
			name:     "Родительный падеж, полное ФИО",
			input:    "Петрова Анна Сергеевна",
			caseName: "родительный",
			format:   "ф и о",
			expected: "Петровой Анны Сергеевны",
		},
		{
			name:     "Дательный падеж, полное ФИО",
			input:    "Петрова Анна Сергеевна",
			caseName: "дательный",
			format:   "ф и о",
			expected: "Петровой Анне Сергеевне",
		},
		{
			name:     "Винительный падеж, полное ФИО",
			input:    "Петрова Анна Сергеевна",
			caseName: "винительный",
			format:   "ф и о",
			expected: "Петрову Анну Сергеевну",
		},
		{
			name:     "Творительный падеж, полное ФИО",
			input:    "Петрова Анна Сергеевна",
			caseName: "творительный",
			format:   "ф и о",
			expected: "Петровой Анной Сергеевной",
		},
		{
			name:     "Предложный падеж, полное ФИО",
			input:    "Петрова Анна Сергеевна",
			caseName: "предложный",
			format:   "ф и о",
			expected: "Петровой Анне Сергеевне",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Declension(tt.input, tt.caseName, tt.format)
			if got != tt.expected {
				t.Errorf("Declension(%q, %q, %q) = %q, want %q",
					tt.input, tt.caseName, tt.format, got, tt.expected)
			}
		})
	}
}

// ————————————————————————————————————————————————————————————————
// Тест: поддержка сокращённых форматов
// ————————————————————————————————————————————————————————————————
func TestDeclension_ShortFormats(t *testing.T) {
	tests := []struct {
		input    string
		caseName string
		format   string
		expected string
	}{
		{"Сидоров Пётр Павлович", "родительный", "ф и.", "Сидорова П."},
		{"Сидоров Пётр Павлович", "дательный", "ф и.о.", "Сидорову П.П."},
		{"Сидоров Пётр Павлович", "творительный", "и.о. ф", "П.П. Сидоровым"},
	}

	for _, tt := range tests {
		got := Declension(tt.input, tt.caseName, tt.format)
		if got != tt.expected {
			t.Errorf("Declension(%q, %q, %q) = %q, want %q",
				tt.input, tt.caseName, tt.format, got, tt.expected)
		}
	}
}

// ————————————————————————————————————————————————————————————————
// Тест: готовые формы (map[string]string)
// ————————————————————————————————————————————————————————————————
func TestDeclension_PreparedForms(t *testing.T) {
	forms := map[string]string{
		"first_gen":  "Ивана",
		"last_gen":   "Иванова",
		"middle_gen": "Ивановича",
		"first_nom":  "Иван",
		"last_nom":   "Иванов",
		"middle_nom": "Иванович",
	}

	tests := []struct {
		name     string
		caseName string
		format   string
		expected string
	}{
		{"родительный ф и о", "родительный", "ф и о", "Иванова Ивана Ивановича"},
		{"именительный и.о. ф", "именительный", "и.о. ф", "И.И. Иванов"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Declension(forms, tt.caseName, tt.format)
			if got != tt.expected {
				t.Errorf("Declension(map, %q, %q) = %q, want %q",
					tt.caseName, tt.format, got, tt.expected)
			}
		})
	}
}

// ————————————————————————————————————————————————————————————————
// Тест: короткие имена без отчества
// ————————————————————————————————————————————————————————————————
func TestDeclension_ShortName(t *testing.T) {
	tests := []struct {
		input    string
		caseName string
		expected string
	}{
		{"Петров Николай", "дательный", "Петрову Николаю"},
		{"Морозова Екатерина", "творительный", "Морозовой Екатериной"},
	}

	for _, tt := range tests {
		got := Declension(tt.input, tt.caseName)
		if got != tt.expected {
			t.Errorf("Declension(%q, %q) = %q, want %q",
				tt.input, tt.caseName, got, tt.expected)
		}
	}
}

// ————————————————————————————————————————————————————————————————
// Тест: граничные случаи
// ————————————————————————————————————————————————————————————————
func TestDeclension_EdgeCases(t *testing.T) {
	t.Run("пустая строка", func(t *testing.T) {
		got := Declension("")
		if got != "" {
			t.Errorf("Declension(\"\") = %q, want empty string", got)
		}
	})

	t.Run("одна фамилия", func(t *testing.T) {
		got := Declension("Смирнов", "дательный")
		if !strings.Contains(got, "Смирнов") {
			t.Errorf("Declension(\"Смирнов\") = %q, want contains фамилия", got)
		}
	})

	t.Run("двойная фамилия через дефис", func(t *testing.T) {
		got := Declension("Петров-Водкин", "творительный")
		want := "Петровым-Водкиным"
		if got != want {
			t.Errorf("Declension(\"Петров-Водкин\", \"творительный\") = %q, want %q", got, want)
		}
	})
}
