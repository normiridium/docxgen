package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"docxgen"
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
		base := strings.TrimSuffix(filepath.Base(*in), ".docx")
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
	defer doc.Close()

	// необязательно, но если есть модификатор p_split
	if err := doc.LoadFontsForPSplit(
		"fonts/TimesNewRoman/TimesNewRoman.ttf",
		"fonts/TimesNewRoman/TimesNewRomanBold.ttf",
		"fonts/TimesNewRoman/TimesNewRomanItalic.ttf",
		"fonts/TimesNewRoman/TimesNewRomanBoldItalic.ttf",
	); err != nil {
		log.Fatalf("шрифты: %v", err)
	}

	// пример кастомных модификаторов
	doc.ImportModifiers(template.FuncMap{
		"upper": func(s string) string { return strings.ToUpper(s) },
		"lower": func(s string) string { return strings.ToLower(s) },
	})

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
