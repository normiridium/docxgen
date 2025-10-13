// Package petrovich — новая реализация склонения ФИО по правилам русского языка.
// Основано на данных из проекта “Petrovich” (https://github.com/petrovich/petrovich-rules)
// Используется в соответствии с MIT License (см. LICENSE в этом каталоге).
package petrovich

import (
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

//go:embed assets/rules.json
var rulesPetrovich embed.FS

// --- типы данных ---

type (
	Gender string
	Case   int
)

const (
	Male        Gender = "male"
	Female      Gender = "female"
	Androgynous Gender = "androgynous"
)

const (
	Genitive Case = iota
	Dative
	Accusative
	Instrumental
	Prepositional
)

type rules struct {
	Lastname   rulesGroup `json:"lastname"`
	Firstname  rulesGroup `json:"firstname"`
	Middlename rulesGroup `json:"middlename"`
}

type rulesGroup struct {
	Exceptions []rule `json:"exceptions"`
	Suffixes   []rule `json:"suffixes"`
}

type rule struct {
	Gender string   `json:"gender"`
	Test   []string `json:"test"`
	Mods   []string `json:"mods"`
	Tags   []string `json:"tags"`
}

// --- загрузка правил ---

// LoadRules загружает rules.json из embed и возвращает объект для склонения.
func LoadRules() (*rules, error) {
	file, err := rulesPetrovich.Open("assets/rules.json")
	if err != nil {
		return nil, err
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	var r rules
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// --- публичный API ---

func (r *rules) InfFirstname(value string, c Case, g Gender) string {
	return inflect(value, r.Firstname, c, g)
}

func (r *rules) InfLastname(value string, c Case, g Gender) string {
	return inflect(value, r.Lastname, c, g)
}

func (r *rules) InfMiddlename(value string, c Case, g Gender) string {
	return inflect(value, r.Middlename, c, g)
}

// InfFio — склоняет полное ФИО.
// fio: строка "Фамилия Имя Отчество"
// c: падеж
// short: true = "Иванов И.И.", false = полная форма
func (r *rules) InfFio(fio string, c Case, short bool) string {
	fio = strings.TrimSpace(fio)
	if fio == "" {
		return ""
	}

	parts := strings.Fields(fio)
	if len(parts) != 3 {
		fmt.Println("Error format of FIO: expected 'Lastname Firstname Middlename'")
		return fio
	}

	g := detectGender(parts[2])

	parts[0] = inflect(parts[0], r.Lastname, c, g)
	if short {
		return fmt.Sprintf("%s %s.%s.",
			parts[0],
			string([]rune(parts[1])[0]),
			string([]rune(parts[2])[0]),
		)
	}

	parts[1] = inflect(parts[1], r.Firstname, c, g)
	parts[2] = inflect(parts[2], r.Middlename, c, g)
	return strings.Join(parts, " ")
}

// --- внутренние функции ---

func detectGender(middlename string) Gender {
	l := strings.ToLower(strings.TrimSpace(middlename))
	switch {
	case strings.HasSuffix(l, "ич"):
		return Male
	case strings.HasSuffix(l, "на"):
		return Female
	default:
		return Androgynous
	}
}

func inflect(value string, group rulesGroup, c Case, g Gender) string {
	if res := checkExcludes(value, group, c, g); res != "" {
		return res
	}

	value = strings.TrimSpace(value)
	if value == "" {
		return value
	}

	// поддержка двойных фамилий (через дефис)
	parts := strings.Split(value, "-")
	if len(parts) > 1 {
		for i := range parts {
			parts[i] = findRules(parts[i], group, c, g)
		}
		return strings.Join(parts, "-")
	}
	return findRules(value, group, c, g)
}

func checkExcludes(name string, group rulesGroup, c Case, g Gender) string {
	lower := strings.ToLower(name)
	for _, ex := range group.Exceptions {
		if ex.Gender == string(g) || ex.Gender == string(Androgynous) {
			for _, t := range ex.Test {
				if t == lower {
					return applyRule(ex.Mods[c], name)
				}
			}
		}
	}
	return ""
}

func findRules(name string, group rulesGroup, c Case, g Gender) string {
	for _, rule := range group.Suffixes {
		if rule.Gender == string(g) || rule.Gender == string(Androgynous) {
			for _, test := range rule.Test {
				if len(test) < len(name) && name[len(name)-len(test):] == test {
					if rule.Mods[c] == "." {
						continue
					}
					return applyRule(rule.Mods[c], name)
				}
			}
		}
	}
	return name
}

func applyRule(mod, name string) string {
	if mod == "." {
		return name
	}
	runes := []rune(name)
	remove := strings.Count(mod, "-")
	if remove > len(runes) {
		remove = len(runes)
	}
	base := runes[:len(runes)-remove]
	return string(base) + strings.ReplaceAll(mod, "-", "")
}
