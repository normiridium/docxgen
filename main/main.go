package main

import (
	"bytes"
	"context"
	"docxgen"
	"docxgen/modifiers"
	"encoding/base64"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

// ---------- live-preview (SSE) ----------

var (
	sseMu      sync.Mutex
	sseClients = map[chan struct{}]struct{}{}
)

// Send a signal to all subscribers /events
func sseNotifyReload() {
	sseMu.Lock()
	defer sseMu.Unlock()
	for ch := range sseClients {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}

func sseHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := make(chan struct{}, 1)

	sseMu.Lock()
	sseClients[ch] = struct{}{}
	sseMu.Unlock()

	// первое "привет"
	_, err := fmt.Fprintf(w, "data: init\n\n")
	if err != nil {
		return
	}
	if f, ok := w.(http.Flusher); ok {
		f.Flush()
	}

	notify := func() error {
		_, err := fmt.Fprintf(w, "data: reload\n\n")
		if err == nil {
			if f, ok := w.(http.Flusher); ok {
				f.Flush()
			}
		}
		return err
	}

	ctx := r.Context()
	for {
		select {
		case <-ch:
			if err := notify(); err != nil {
				sseMu.Lock()
				delete(sseClients, ch)
				sseMu.Unlock()
				return
			}
		case <-ctx.Done():
			sseMu.Lock()
			delete(sseClients, ch)
			sseMu.Unlock()
			return
		}
	}
}

// the path to the file that we are looking at in the preview
func previewOutputPath(out string, pdfOut bool) string {
	if pdfOut {
		low := strings.ToLower(out)
		if strings.HasSuffix(low, ".pdf") {
			return out
		}
		return strings.TrimSuffix(out, filepath.Ext(out)) + ".pdf"
	}
	return out
}

const previewHTML = `<!DOCTYPE html>
<html>
	<head>
		<meta charset="utf-8">
		<title>docxgen preview</title>
		<style>
			html, body { margin:0; padding:0; height:100%; }
			iframe { border:0; width:100%; height:100%; }
		</style>
	</head>
	<body>
		<iframe id="frame" src="/file"></iframe>
		<script>
			const es = new EventSource("/events");
			es.onmessage = function() {
			const f = document.getElementById("frame");
			f.src = "/file?t=" + Date.now();
			};
		</script>
	</body>
</html>
`

func runPreviewServer(port int, out string, pdfOut bool) {
	outPath := previewOutputPath(out, pdfOut)

	http.HandleFunc("/view", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = io.WriteString(w, previewHTML)
	})

	http.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
		path := outPath

		w.Header().Set("Cache-Control", "no-cache, no-store, must-revalidate")
		w.Header().Set("Pragma", "no-cache")
		w.Header().Set("Expires", "0")

		if pdfOut {
			w.Header().Set("Content-Type", "application/pdf")
		} else {
			w.Header().Set("Content-Type", "application/vnd.openxmlformats-officedocument.wordprocessingml.document")
		}
		http.ServeFile(w, r, path)
	})

	http.HandleFunc("/events", sseHandler)

	log.Printf("🦌 preview: http://localhost:%d/view\n", port)
	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", port), nil))
}

// ---------- main ----------

func main() {
	in := flag.String("in", "", "input DOCX template")
	out := flag.String("out", "", "result (default template name + _out.docx)")
	dataFile := flag.String("data", "", "JSON with lookup data")
	watch := flag.Bool("watch", false, "monitor changes and rebuilds automatically")
	debounce := flag.Duration("debounce", 300*time.Millisecond, "debounce before rebuild")
	serve := flag.Bool("serve", false, "daemon mode (HTTP API)")
	port := flag.Int("port", 8080, "daemon HTTP port/preview")
	download := flag.Bool("download", false, "do not save, but output the finished DOCX to stdout")
	pdfOut := flag.Bool("pdf", false, "immediately convert to PDF (without saving DOCX)")
	preview := flag.Bool("preview", false, "run the HTML /view viewer for the result (handy with --watch and --pdf)")
	pdfEngine := flag.String("pdf-engine", "", "preferred PDF engine: libreoffice|soffice|unoconv")
	lang := flag.String("lang", "eng", "localization")
	flag.Parse()

	baseDir, _ := os.Getwd()
	pdfEngineFlag = *pdfEngine

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

	// defaults
	if *in == "" {
		*in = filepath.Join(projectRoot, fmt.Sprintf("main/examples/template_%s.docx", *lang))
	}
	if *dataFile == "" {
		*dataFile = filepath.Join(projectRoot, fmt.Sprintf("main/examples/data_%s.json", *lang))
	}
	if *out == "" {
		base := strings.TrimSuffix(filepath.Join(projectRoot, "main/examples", filepath.Base(*in)), ".docx")
		*out = base + "_out.docx"
	}

	// First assembly
	if err := render(*in, *dataFile, *out, projectRoot, *download, *pdfOut); err != nil {
		log.Fatalf("💥  ошибка сборки: %v\n", err)
	}
	if *download {
		return
	}
	fmt.Println("💚  готово: " + prettyOutputPath(*out, *pdfOut, baseDir))

	// If it's a preview, start the server
	if *preview {
		if *watch {
			go runPreviewServer(*port, *out, *pdfOut)
		} else {
			// без watch — просто сервер-просмотрщик
			runPreviewServer(*port, *out, *pdfOut)
			return
		}
	}

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
			if err := render(*in, *dataFile, *out, projectRoot, false, *pdfOut); err != nil {
				fmt.Printf("💥  %v\n", err)
			} else {
				fmt.Println("💚  готово: " + prettyOutputPath(*out, *pdfOut, baseDir))
				// пинг браузеру
				sseNotifyReload()
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

// ---------- Shared Pipeline ----------
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
	// builtins are added inside the ExecuteTemplate; our mods are already in extraFuncs
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
		"upper": {Func: func(value string) string { return strings.ToUpper(value) }, Count: 0},
		"lower": {Func: func(value string) string { return strings.ToLower(value) }, Count: 0},
		"wrap":  {Func: func(v, l, r string) string { return l + v + r }, Count: 2},
		"gender_select": {
			Func: func(v any, forms ...string) string {
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

// ---------- CLI render ----------
func render(in, dataFile, out, projectRoot string, download, pdfOut bool) error {
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

	if pdfOut {
		var buf bytes.Buffer
		if err := doc.SaveToWriter(&buf); err != nil {
			return err
		}
		pdfData, err := convertToPDF(buf.Bytes())
		if err != nil {
			return err
		}
		if download {
			_, err = os.Stdout.Write(pdfData)
			return err
		}
		pdfPath := strings.TrimSuffix(out, filepath.Ext(out)) + ".pdf"
		return os.WriteFile(pdfPath, pdfData, 0644)
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

// ---------- demon ----------
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
			// you need a docx "skeleton"; use any valid in the project
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

		// Common fonts/modifiers and execution
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

		// Send the file directly
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

var pdfEngineFlag string

// Engine Order: From Best to Worst
var pdfEngines = []string{
	"soffice", // LibreOffice headless
	"libreoffice",
	"lowriter",
	"unoconv", // fallback, но крайне ненадёжный
}

func findExec(bin string) (string, bool) {
	p, err := exec.LookPath(bin)
	return p, err == nil
}

func runEngine(engine string, docx, pdf string) error {
	fmt.Printf("📑  пробуем конвертацию в pdf через: %s\n", engine)
	switch engine {

	case "soffice", "libreoffice":
		return exec.Command(engine,
			"--headless",
			"--convert-to", "pdf:writer_pdf_Export",
			"--outdir", filepath.Dir(pdf),
			docx,
		).Run()

	case "lowriter":
		return exec.Command("lowriter",
			"--convert-to", "pdf",
			"--outdir", filepath.Dir(pdf),
			docx,
		).Run()

	case "unoconv":
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		// unoconv требует basename без расширения
		outNoExt := strings.TrimSuffix(pdf, filepath.Ext(pdf))

		cmd := exec.CommandContext(ctx,
			"unoconv",
			"-f", "pdf",
			"-o", outNoExt, // <--- ВАЖНО!
			docx,
		)

		if err := cmd.Run(); err != nil {
			if errors.Is(ctx.Err(), context.DeadlineExceeded) {
				return fmt.Errorf("unoconv timeout")
			}
			return fmt.Errorf("unoconv failed: %w", err)
		}

		return nil
	}

	return fmt.Errorf("unknown engine: %s", engine)
}

func convertToPDF(docxBytes []byte) ([]byte, error) {

	tmpDocx := filepath.Join(os.TempDir(), fmt.Sprintf("doc_%d.docx", time.Now().UnixNano()))
	tmpPDF := strings.TrimSuffix(tmpDocx, ".docx") + ".pdf"

	if err := os.WriteFile(tmpDocx, docxBytes, 0644); err != nil {
		return nil, err
	}
	defer func(name string) {
		err := os.Remove(name)
		if err != nil {
			fmt.Printf("не удалился файл %s, ошибка: %v", name, err)
		}
	}(tmpDocx)

	// preferred engine
	if pdfEngineFlag != "" {
		if _, ok := findExec(pdfEngineFlag); ok {
			if err := runEngine(pdfEngineFlag, tmpDocx, tmpPDF); err == nil {
				data, _ := os.ReadFile(tmpPDF)
				_ = os.Remove(tmpPDF)
				return data, nil
			}
		}
	}

	// try engines in order
	for _, engine := range pdfEngines {
		_, ok := findExec(engine)
		if !ok {
			continue
		}

		err := runEngine(engine, tmpDocx, tmpPDF)
		if err != nil {
			// skip silently → continue to next engine
			continue
		}

		// success
		data, err := os.ReadFile(tmpPDF)
		_ = os.Remove(tmpPDF)
		return data, err
	}

	return nil, fmt.Errorf("no available PDF engines found")
}

// ---------- helpers ----------
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

func prettyOutputPath(out string, pdfOut bool, baseDir string) string {
	// Choosing the real file name
	result := out
	if pdfOut {
		result = strings.TrimSuffix(out, filepath.Ext(out)) + ".pdf"
	}

	// Removing the absolute path for privacy
	pretty := strings.TrimPrefix(result, baseDir)
	if strings.HasPrefix(pretty, "/") {
		pretty = pretty[1:]
	}

	return pretty
}
