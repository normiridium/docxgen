package main

import (
	"bytes"
	"docxgen"
	"docxgen/modifiers"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
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
	watch := flag.Bool("watch", false, "следить за изменениями и пересборкой автоматически")
	debounce := flag.Duration("debounce", 300*time.Millisecond, "дебаунс перед пересборкой")
	serve := flag.Bool("serve", false, "режим демона (HTTP API)")
	port := flag.Int("port", 8080, "порт HTTP демона")
	download := flag.Bool("download", false, "не сохранять, а вывести готовый DOCX в stdout")
	flag.Parse()

	baseDir, _ := os.Getwd()

	// ищем корень проекта по наличию go.mod
	projectRoot := baseDir
	for {
		if _, err := os.Stat(filepath.Join(projectRoot, "go.mod")); err == nil {
			break
		}
		parent := filepath.Dir(projectRoot)
		if parent == projectRoot {
			break
		}
		projectRoot = parent
	}

	if *serve {
		runServer(*port, projectRoot)
		return
	}

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

	// первая сборка
	if err := render(*in, *dataFile, *out, projectRoot, *download); err != nil {
		log.Fatalf("💥  ошибка сборки: %v\n", err)
	}
	if *download {
		return
	}
	fmt.Println("💚  готово: " + strings.TrimPrefix(*out, baseDir))

	// watch
	if !*watch {
		return
	}

	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatalf("watcher: %v", err)
	}
	defer func() {
		_ = watcher.Close()
	}()

	toWatch := dedupe([]string{
		*in, filepath.Dir(*in),
		*dataFile, filepath.Dir(*dataFile),
	})
	for _, p := range toWatch {
		if p == "" {
			continue
		}
		if err := watcher.Add(p); err != nil {
			log.Printf("warn: не удалось добавить в watch %s: %v\n", p, err)
		}
	}

	outAbs, _ := filepath.Abs(*out)
	ignore := func(name string) bool {
		n, _ := filepath.Abs(name)
		if n == outAbs {
			return true
		}
		low := strings.ToLower(n)
		return strings.HasSuffix(low, "~") ||
			strings.HasSuffix(low, ".tmp") ||
			strings.HasSuffix(low, ".swp") ||
			strings.HasSuffix(low, ".lock") ||
			strings.HasSuffix(low, "_out.docx")
	}

	fmt.Print("\033[?25l")
	defer fmt.Print("\033[?25h")

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, os.Interrupt)

	var t *time.Timer
	schedule := func() {
		if t != nil {
			t.Stop()
		}
		t = time.AfterFunc(*debounce, func() {
			fmt.Println("🔄  пересборка…")
			if err := render(*in, *dataFile, *out, projectRoot, false); err != nil {
				fmt.Printf("💥  %v\n", err)
			} else {
				fmt.Println("💚  готово: " + strings.TrimPrefix(*out, baseDir))
			}
		})
	}

	fmt.Println("👀  watch-режим (Ctrl+C — выход)")
	for {
		select {
		case ev := <-watcher.Events:
			if ev.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename|fsnotify.Remove) == 0 {
				continue
			}
			if ignore(ev.Name) {
				continue
			}
			if hasAnySuffix(strings.ToLower(ev.Name), ".docx", ".docm", ".dotx", ".json") {
				fmt.Println("📝  изменено: " + filepath.Base(ev.Name) + " → жду дебаунс…")
				schedule()
			}
		case err := <-watcher.Errors:
			log.Printf("watch error: %v\n", err)
		case <-sig:
			fmt.Print("\r\033[K👋  пока\n")
			return
		}
	}
}

// ---------- общий пайплайн ----------
func buildDocFromPath(path, projectRoot string) (*docxgen.Docx, error) {
	doc, err := docxgen.Open(path)
	if err != nil {
		return nil, fmt.Errorf("открытие DOCX: %w", err)
	}
	if err := loadFonts(doc, projectRoot); err != nil {
		// не критично
		log.Printf("шрифты: %v\n", err)
	}
	registerCommonModifiers(doc)
	return doc, nil
}

func executeTemplate(doc *docxgen.Docx, data map[string]any) error {
	// в ExecuteTemplate внутри добавляются builtins; наши моды уже в extraFuncs
	if err := doc.ExecuteTemplate(data); err != nil {
		return fmt.Errorf("шаблон: %w", err)
	}
	return nil
}

func loadFonts(doc *docxgen.Docx, projectRoot string) error {
	return doc.LoadFontsForPSplit(
		filepath.Join(projectRoot, "fonts/TimesNewRoman/TimesNewRoman.ttf"),
		filepath.Join(projectRoot, "fonts/TimesNewRoman/TimesNewRomanBold.ttf"),
		filepath.Join(projectRoot, "fonts/TimesNewRoman/TimesNewRomanItalic.ttf"),
		filepath.Join(projectRoot, "fonts/TimesNewRoman/TimesNewRomanBoldItalic.ttf"),
	)
}

func registerCommonModifiers(doc *docxgen.Docx) {
	doc.ImportModifiers(map[string]modifiers.ModifierMeta{
		"upper": {Fn: func(value string) string { return strings.ToUpper(value) }, Count: 0},
		"lower": {Fn: func(value string) string { return strings.ToLower(value) }, Count: 0},
		"wrap":  {Fn: func(v, l, r string) string { return l + v + r }, Count: 2},
		"gender_select": {
			Fn: func(v any, forms ...string) string {
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
			},
			Count: 0,
		},
	})
}

// ---------- CLI рендер ----------
func render(in, dataFile, out, projectRoot string, download bool) error {
	data := map[string]any{}
	raw, err := os.ReadFile(dataFile)
	if err != nil {
		return fmt.Errorf("чтение JSON: %w", err)
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return fmt.Errorf("разбор JSON: %w", err)
	}

	doc, err := buildDocFromPath(in, projectRoot)
	if err != nil {
		return err
	}

	if err := executeTemplate(doc, data); err != nil {
		return err
	}

	if download {
		var buf bytes.Buffer
		if err = doc.SaveToWriter(&buf); err != nil {
			return fmt.Errorf("сохранение в поток: %w", err)
		}
		if _, err = io.Copy(os.Stdout, &buf); err != nil {
			return fmt.Errorf("вывод stdout: %w", err)
		}
		return nil
	}

	if err := doc.Save(out); err != nil {
		return fmt.Errorf("сохранение: %w", err)
	}
	return nil
}

// ---------- демон ----------
func runServer(port int, projectRoot string) {
	http.HandleFunc("/generate", func(w http.ResponseWriter, r *http.Request) {
		var req struct {
			Template string         `json:"template"`
			Data     map[string]any `json:"data,omitempty"`
			Format   string         `json:"format,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			jsonErr(w, 400, "invalid json: %v", err)
			return
		}
		if strings.TrimSpace(req.Template) == "" {
			jsonErr(w, 400, "template is required: pass a file path, base64 DOCX, or <w:document> xml")
			return
		}

		var (
			doc *docxgen.Docx
			err error
		)

		switch {
		case fileExists(req.Template):
			doc, err = docxgen.Open(req.Template)
			if err != nil {
				jsonErr(w, 500, "template open error: %v", err)
				return
			}
		case hasAnySuffix(strings.ToLower(req.Template), ".docx", ".docm", ".dotx"):
			candidate := filepath.Join(projectRoot, req.Template)
			if fileExists(candidate) {
				doc, err = docxgen.Open(candidate)
			} else {
				candidate = filepath.Join(projectRoot, "main", req.Template)
				if fileExists(candidate) {
					doc, err = docxgen.Open(candidate)
				} else {
					jsonErr(w, 400, "file not found: %s", candidate)
					return
				}
			}
		case strings.HasPrefix(strings.TrimSpace(req.Template), "<w:"):
			// нужен «скелет» docx; используем любой валидный в проекте
			doc, err = docxgen.Open("examples/template.docx")
			if err != nil {
				jsonErr(w, 500, "template skeleton error: %v", err)
				return
			}
			doc.UpdateContentPart("document", req.Template)
		default:
			raw, decErr := base64.StdEncoding.DecodeString(req.Template)
			if decErr != nil {
				jsonErr(w, 400, "template: not a path, not xml, and bad base64: %v", decErr)
				return
			}
			tmp := filepath.Join(os.TempDir(), fmt.Sprintf("tmpl_%d.docx", time.Now().UnixNano()))
			if err := os.WriteFile(tmp, raw, 0644); err != nil {
				jsonErr(w, 500, "write temp: %v", err)
				return
			}
			defer func() {
				err = os.Remove(tmp)
				if err != nil {
					jsonErr(w, 500, "template remove error: %v", err)
					return
				}
			}()
			doc, err = docxgen.Open(tmp)
			if err != nil {
				jsonErr(w, 500, "template open error: %v", err)
				return
			}
		}

		// общие шрифты/модификаторы и выполнение
		if err := loadFonts(doc, "."); err != nil {
			log.Printf("шрифты: %v\n", err)
		}
		registerCommonModifiers(doc)
		if err := executeTemplate(doc, req.Data); err != nil {
			jsonErr(w, 500, "%v", err)
			return
		}

		if strings.EqualFold(req.Format, "xml") {
			xml, _ := doc.ContentPart("document")
			w.Header().Set("Content-Type", "application/xml; charset=utf-8")
			_, _ = w.Write([]byte(xml))
			return
		}

		// отдаём файл напрямую
		w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
		w.Header().Set("Content-Disposition", `attachment; filename="result.docx"`)
		if err := doc.SaveToWriter(w); err != nil {
			jsonErr(w, 500, "stream error: %v", err)
			return
		}
	})

	log.Printf("🦌  Демон слушает порт %d\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

// ---------- вспомогательные ----------
func fileExists(p string) bool {
	fi, err := os.Stat(p)
	return err == nil && !fi.IsDir()
}

func jsonErr(w http.ResponseWriter, code int, fmtStr string, a ...any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	msg := fmt.Sprintf(fmtStr, a...)
	_, _ = w.Write([]byte(`{"error":"` + strings.ReplaceAll(msg, `"`, `\"`) + `"}`))
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
