package main

import (
	"docxgen"
	"docxgen/modifiers"

	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

func main() {
	in := flag.String("in", "", "входной DOCX-шаблон")
	out := flag.String("out", "", "результат (по умолчанию имя шаблона + _out.docx)")
	dataFile := flag.String("data", "", "JSON с данными для подстановки")
	watch := flag.Bool("watch", false, "следить за изменениями и пересобирать автоматически")
	debounce := flag.Duration("debounce", 300*time.Millisecond, "дебаунс перед пересборкой")
	quiet := flag.Bool("quiet", false, "минимальный лог (одна статусная строка)")
	flag.Parse()

	baseDir, _ := os.Getwd()

	// ищем корень проекта по наличию go.mod
	search := baseDir
	for {
		if _, err := os.Stat(filepath.Join(search, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(search)
		if parent == search {
			break
		}
		search = parent
	}
	projectRoot := search

	// дефолты
	if *in == "" {
		*in = filepath.Join(projectRoot, "main/examples/template.docx")
	}
	if *dataFile == "" {
		*dataFile = filepath.Join(projectRoot, "main/examples/data.json")
	}
	if *out == "" {
		base := strings.TrimSuffix(filepath.Join(projectRoot, "main/examples", filepath.Base(*in)), ".docx")
		*out = base + "_out.docx"
	}

	// подготовим однострочный логгер
	logPrint := func(s string) {
		if *quiet {
			return
		}
		fmt.Printf("\r%-100s", s)
	}

	// первая сборка
	if err := render(*in, *dataFile, *out, projectRoot); err != nil {
		fmt.Print("\r")
		log.Printf("❌ ошибка сборки: %v\n", err)
	} else {
		logPrint("✅ готово: " + *out)
	}

	// go run . --watch
	if !*watch {
		if !*quiet {
			fmt.Println()
		}
		return
	}

	// режим слежения
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("watcher: %v", err)
	}
	defer func(watcher *fsnotify.Watcher) {
		err = watcher.Close()
		log.Fatalf("close watcher: %v", err)
	}(watcher)

	// добавляем файлы и их директории
	toWatch := dedupe([]string{
		*in, filepath.Dir(*in),
		*dataFile, filepath.Dir(*dataFile),
	})
	for _, p := range toWatch {
		if p == "" {
			continue
		}
		if err := watcher.Add(p); err != nil {
			fmt.Print("\r")
			log.Printf("warn: не удалось добавить в watch %s: %v\n", p, err)
		}
	}

	// игнорируем наш выходной файл и типичные артефакты редакторов
	outAbs, _ := filepath.Abs(*out)
	ignore := func(name string) bool {
		n, _ := filepath.Abs(name)
		if n == outAbs {
			return true
		}
		low := strings.ToLower(n)
		if strings.HasSuffix(low, "~") ||
			strings.HasSuffix(low, ".tmp") ||
			strings.HasSuffix(low, ".swp") ||
			strings.HasSuffix(low, ".lock") ||
			strings.HasSuffix(low, "_out.docx") {
			return true
		}
		return false
	}

	// ловим Ctrl+C
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	var t *time.Timer
	schedule := func() {
		if t != nil {
			t.Stop()
		}
		t = time.AfterFunc(*debounce, func() {
			logPrint("⚙️  собираю…")
			if err := render(*in, *dataFile, *out, projectRoot); err != nil {
				fmt.Print("\r")
				log.Printf("❌ %v\n", err)
			} else {
				logPrint("✅ готово: " + *out)
			}
		})
	}

	if !*quiet {
		fmt.Println("\n👀 watch-режим (Ctrl+C — выход)")
	}

	for {
		select {
		case ev := <-watcher.Events:
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
				continue
			}
			if ignore(ev.Name) {
				continue
			}
			low := strings.ToLower(ev.Name)
			if hasAnySuffix(low, ".docx", ".docm", ".dotx", ".json") {
				logPrint("✎ изменено: " + filepath.Base(ev.Name) + " → жду дебаунс…")
				schedule()
			}
		case err := <-watcher.Errors:
			fmt.Print("\r")
			log.Printf("watch error: %v\n", err)
		case <-sig:
			if !*quiet {
				fmt.Println("\n👋 выхожу")
			}
			return
		}
	}
}

func render(in, dataFile, out, projectRoot string) error {
	// читаем JSON
	data := map[string]any{}
	raw, err := os.ReadFile(dataFile)
	if err != nil {
		return fmt.Errorf("чтение JSON: %w", err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("разбор JSON: %w", err)
	}

	// открываем документ
	doc, err := docxgen.Open(in)
	if err != nil {
		return fmt.Errorf("открытие DOCX: %w", err)
	}

	// (опционально) шрифты для p_split — если используется
	if err := doc.LoadFontsForPSplit(
		filepath.Join(projectRoot, "fonts/TimesNewRoman/TimesNewRoman.ttf"),
		filepath.Join(projectRoot, "fonts/TimesNewRoman/TimesNewRomanBold.ttf"),
		filepath.Join(projectRoot, "fonts/TimesNewRoman/TimesNewRomanItalic.ttf"),
		filepath.Join(projectRoot, "fonts/TimesNewRoman/TimesNewRomanBoldItalic.ttf"),
	); err != nil {
		// не критично: просто сообщим в лог
		log.Printf("шрифты: %v\n", err)
	}

	// кастомные модификаторы (пример — можно убрать/заменить на ваши)
	doc.ImportModifiers(map[string]modifiers.ModifierMeta{
		"upper": {Fn: func(value string) string { return strings.ToUpper(value) }, Count: 0},
		"lower": {Fn: func(value string) string { return strings.ToLower(value) }, Count: 0},
		"wrap":  {Fn: func(v, l, r string) string { return l + v + r }, Count: 2},
	})
	doc.AddModifier("gender_select", func(v any, forms ...string) string {
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
		s, _ := v.(string)
		low := strings.ToLower(strings.TrimSpace(s))
		switch low {
		case "m", "м", "муж", "мужской":
			return male
		case "f", "ж", "жен", "женский":
			return female
		}
		name := strings.ToLower(strings.TrimSpace(fmt.Sprint(v)))
		if name == "" {
			return neutral
		}
		parts := strings.Fields(name)
		if len(parts) == 3 {
			if strings.HasSuffix(parts[2], "ич") {
				return male
			}
			if strings.HasSuffix(parts[2], "на") {
				return female
			}
		}
		last := parts[0]
		switch {
		case strings.HasSuffix(last, "ов"), strings.HasSuffix(last, "ев"),
			strings.HasSuffix(last, "ин"), strings.HasSuffix(last, "ский"),
			strings.HasSuffix(last, "цкий"):
			return male
		case strings.HasSuffix(last, "ова"), strings.HasSuffix(last, "ева"),
			strings.HasSuffix(last, "ина"), strings.HasSuffix(last, "ая"),
			strings.HasSuffix(last, "ская"):
			return female
		}
		return neutral
	}, 0)

	// выполняем шаблон
	if err := doc.ExecuteTemplate(data); err != nil {
		return fmt.Errorf("шаблон: %w", err)
	}

	// сохраняем результат
	if err := doc.Save(out); err != nil {
		return fmt.Errorf("сохранение: %w", err)
	}
	return nil
}

func dedupe(in []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, p := range in {
		if p == "" {
			continue
		}
		abs, _ := filepath.Abs(p)
		if _, ok := seen[abs]; !ok {
			seen[abs] = struct{}{}
			out = append(out, abs)
		}
	}
	return out
}

func hasAnySuffix(s string, exts ...string) bool {
	for _, e := range exts {
		if strings.HasSuffix(s, e) {
			return true
		}
	}
	return false
}
