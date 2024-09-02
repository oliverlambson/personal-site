package server

import (
	"bytes"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/alecthomas/chroma"
	"github.com/alecthomas/chroma/formatters/html"
	"github.com/alecthomas/chroma/lexers"
	"github.com/alecthomas/chroma/styles"
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/ast"
	mdhtml "github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
)

type CustomResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (w *CustomResponseWriter) WriteHeader(code int) {
	w.statusCode = code
	w.ResponseWriter.WriteHeader(code)
}

func NewCustomResponseWriter(w http.ResponseWriter) *CustomResponseWriter {
	return &CustomResponseWriter{w, http.StatusOK} // default 200
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		writer := NewCustomResponseWriter(w)
		next.ServeHTTP(writer, r)

		duration := time.Since(start)
		log.Printf("%s %s %d (%s)", r.Method, r.URL.Path, writer.statusCode, duration)
	})
}

func NewHTMLHighligher(htmlFormatter *html.Formatter, highlightStyle *chroma.Style) func(w io.Writer, source, lang, defaultLang string) error {
	return func(w io.Writer, source, lang, defaultLang string) error {
		if lang == "" {
			lang = defaultLang
		}
		l := lexers.Get(lang)
		if l == nil {
			l = lexers.Analyse(source)
		}
		if l == nil {
			l = lexers.Fallback
		}
		l = chroma.Coalesce(l)

		it, err := l.Tokenise(nil, source)
		if err != nil {
			return err
		}

		return htmlFormatter.Format(w, highlightStyle, it)
	}
}

func NewCodeBlockRenderHook(htmlHighlight func(w io.Writer, source, lang, defaultLang string) error) func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
	return func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
		if code, ok := node.(*ast.CodeBlock); ok {
			defaultLang := ""
			lang := string(code.Info)
			htmlHighlight(w, string(code.Literal), lang, defaultLang)
			return ast.GoToNext, true
		}
		return ast.GoToNext, false
	}
}

// see: https://blog.kowalczyk.info/article/cxn3/advanced-markdown-processing-in-go.html
func mdToHTML(md []byte) []byte {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(md)

	highlightStyle := styles.Get("monokai")
	formatter := html.New(html.Standalone(false), html.TabWidth(4))
	htmlHighlight := NewHTMLHighligher(formatter, highlightStyle)
	codeBlockRenderHook := NewCodeBlockRenderHook(htmlHighlight)
	opts := mdhtml.RendererOptions{
		Flags:          mdhtml.CommonFlags | mdhtml.HrefTargetBlank,
		RenderNodeHook: codeBlockRenderHook,
	}
	renderer := mdhtml.NewRenderer(opts)
	return markdown.Render(doc, renderer)
}

type Page struct {
	title string
	body  []byte
}

func NewServer(Addr string) *http.Server {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	mux := http.NewServeMux()

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static/"))))

	// process all content and save in memory
	postTemplate := template.Must(template.ParseFiles("web/templates/base.html.tmpl", "web/templates/post.html.tmpl"))
	pageTemplate := template.Must(template.ParseFiles("web/templates/base.html.tmpl", "web/templates/page.html.tmpl"))

	pages := make(map[string]Page)

	postsDir := "web/content/posts"
	files, err := os.ReadDir(postsDir)
	if err != nil {
		log.Fatalf("failed to read posts directory: %v", err)
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if filepath.Ext(file.Name()) != ".md" {
			continue
		}

		path := filepath.Join(postsDir, file.Name())
		contents, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("failed to read file: %v", err)
		}

		title := strings.TrimSuffix(file.Name()[9:], ".md")
		body := mdToHTML(contents)

		var fullHTML bytes.Buffer
		data := struct {
			Title string
			Body  []byte
		}{
			Title: title,
			Body:  body,
		}
		err = postTemplate.Execute(&fullHTML, data)

		pages[title] = Page{title: title, body: fullHTML.Bytes()}
	}

	pagesDir := "web/content/"
	files, err = os.ReadDir(pagesDir)
	if err != nil {
		log.Fatalf("failed to read pages directory: %v", err)
	}
	for _, file := range files {
		if file.IsDir() {
			continue
		}

		path := filepath.Join(pagesDir, file.Name())
		contents, err := os.ReadFile(path)
		if err != nil {
			log.Fatalf("failed to read file: %v", err)
		}
		ext := filepath.Ext(file.Name())
		title := strings.TrimSuffix(file.Name(), ext)
		body := []byte{}
		tmpl := pageTemplate
		if ext == ".md" {
			body = mdToHTML(contents)
			tmpl = postTemplate
		} else if ext == ".html" {
			body = contents
		}

		data := struct {
			Title string
			Body  []byte
		}{
			Title: title,
			Body:  body,
		}
		var fullHTML bytes.Buffer
		err = tmpl.Execute(&fullHTML, data)
		if err != nil {
			log.Fatalf("failed to execute template: %v", err)
		}
		pageHTML := fullHTML.Bytes()

		pages[title] = Page{title: title, body: pageHTML}
	}
	for title := range pages {
		log.Printf("loaded page: %s", title)
	}

	for title, page := range pages {
		if title == "index" {
			mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path != "/" {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Content-Type", "text/html")
				_, err := w.Write(page.body)
				if err != nil {
					http.Error(w, "Internal Server Error", http.StatusInternalServerError)
					log.Printf("failed to write response: %v", err)
				}
			})
			continue
		}
		mux.HandleFunc("/"+title, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, err := w.Write(page.body)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				log.Printf("failed to write response: %v", err)
			}
		})
	}

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	loggedMux := loggingMiddleware(mux)

	server := &http.Server{
		Addr:    Addr,
		Handler: loggedMux,
	}
	return server
}
