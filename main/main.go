package main

import (
	"docxgen"
	"docxgen/modifiers"

	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	in := flag.String("in", "", "входной DOCX-шаблон")
	out := flag.String("out", "", "результат (по умолчанию имя шаблона + _out.docx)")
	dataFile := flag.String("data", "", "JSON с данными для подстановки")
	flag.Parse()

	// если не заданы параметры, подставляем examples/*
	if *in == "" {
		*in = "examples/template.docx"
	}
	if *dataFile == "" {
		*dataFile = "examples/data.json"
	}
	if *out == "" {
		base := strings.TrimSuffix(filepath.Join("examples", filepath.Base(*in)), ".docx")
		*out = base + "_out.docx"
	}

	// читаем JSON с данными
	data := map[string]any{}
	raw, err := os.ReadFile(*dataFile)
	if err != nil {
		log.Fatalf("чтение JSON: %v", err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		log.Fatalf("разбор JSON: %v", err)
	}

	// открываем документ
	doc, err := docxgen.Open(*in)
	if err != nil {
		log.Fatalf("открытие DOCX: %v", err)
	}

	// необязательно, но если есть модификатор p_split
	if err := doc.LoadFontsForPSplit(
		"fonts/TimesNewRoman/TimesNewRoman.ttf",
		"fonts/TimesNewRoman/TimesNewRomanBold.ttf",
		"fonts/TimesNewRoman/TimesNewRomanItalic.ttf",
		"fonts/TimesNewRoman/TimesNewRomanBoldItalic.ttf",
	); err != nil {
		log.Fatalf("шрифты: %v", err)
	}

	// пример кастомных модификаторов, регистрация пакетом
	doc.ImportModifiers(map[string]modifiers.ModifierMeta{
		"upper": {Fn: func(value string) string { return strings.ToUpper(value) }, Count: 0},
		"lower": {Fn: func(value string) string { return strings.ToLower(value) }, Count: 0},
		"wrap":  {Fn: func(v, l, r string) string { return l + v + r }, Count: 2},
	})

	// вариант - регистрация по одному
	doc.AddModifier("gender_select", func(v any, forms ...string) string {
		// умолчания
		male, female, neutral := "Уважаемый", "Уважаемая", "Уважаемый(ая)"

		if len(forms) >= 1 && strings.TrimSpace(forms[0]) != "" {
			male = forms[0]
		}
		if len(forms) >= 2 && strings.TrimSpace(forms[1]) != "" {
			female = forms[1]
		}
		if len(forms) >= 3 && strings.TrimSpace(forms[2]) != "" {
			neutral = forms[2]
		}

		// если передали явно "m" / "f" / "м" / "ж"
		if s, ok := v.(string); ok {
			low := strings.ToLower(strings.TrimSpace(s))
			if low == "m" || low == "м" || low == "муж" || low == "мужской" {
				return male
			}
			if low == "f" || low == "ж" || low == "жен" || low == "женский" {
				return female
			}
		}

		// иначе — попытка определить по ФИО
		name := strings.ToLower(strings.TrimSpace(fmt.Sprint(v)))
		if name == "" {
			return neutral
		}
		parts := strings.Fields(name)

		// 1) если есть отчество
		if len(parts) == 3 {
			if strings.HasSuffix(parts[2], "ич") {
				return male
			}
			if strings.HasSuffix(parts[2], "на") {
				return female
			}
		}

		// 2) если фамилия (или одно слово)
		last := parts[0]
		switch {
		case strings.HasSuffix(last, "ов"),
			strings.HasSuffix(last, "ев"),
			strings.HasSuffix(last, "ин"),
			strings.HasSuffix(last, "ский"),
			strings.HasSuffix(last, "цкий"):
			return male
		case strings.HasSuffix(last, "ова"),
			strings.HasSuffix(last, "ева"),
			strings.HasSuffix(last, "ина"),
			strings.HasSuffix(last, "ая"),
			strings.HasSuffix(last, "ская"):
			return female
		}

		return neutral
	}, 0)

	// выполняем шаблон
	if err := doc.ExecuteTemplate(data); err != nil {
		log.Fatalf("шаблон: %v", err)
	}

	// сохраняем результат
	if err := doc.Save(*out); err != nil {
		log.Fatalf("сохранение: %v", err)
	}

	fmt.Println("Файл успешно создан:", *out)
}
