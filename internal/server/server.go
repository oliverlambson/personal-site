package server

import (
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
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

// factory so i don't need a global htmlFormatter var
func makeHTMLHighligher(htmlFormatter *html.Formatter, highlightStyle *chroma.Style) func(w io.Writer, source, lang, defaultLang string) error {
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

// factory so i don't need a global htmlHighlight var
func makeCodeBlockRenderHook(htmlHighlight func(w io.Writer, source, lang, defaultLang string) error) func(w io.Writer, node ast.Node, entering bool) (ast.WalkStatus, bool) {
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

	highlightStyle := styles.Get("monokailight")
	formatter := html.New(html.WithClasses(true), html.TabWidth(2))
	// TODO: formatter.WriteCSS(w io.Writer, style *chroma.Style)
	htmlHighlight := makeHTMLHighligher(formatter, highlightStyle)
	codeBlockRenderHook := makeCodeBlockRenderHook(htmlHighlight)
	opts := mdhtml.RendererOptions{
		Flags:          mdhtml.CommonFlags | mdhtml.HrefTargetBlank,
		RenderNodeHook: codeBlockRenderHook,
	}
	renderer := mdhtml.NewRenderer(opts)
	return markdown.Render(doc, renderer)
}

func markdownHandler(w http.ResponseWriter, r *http.Request) {
	htmlFormatter := html.New(html.WithClasses(true), html.TabWidth(2))
	if htmlFormatter == nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Println("couldn't create html formatter")
		return
	}

	filePath := filepath.Join("web/content/in/", strings.TrimPrefix(r.URL.Path, "/content/in/"))

	md, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		log.Printf("failed to read file: %v", err)
		return
	}

	html := mdToHTML(md)

	w.Header().Set("Content-Type", "text/html")
	_, err = w.Write(html)
	if err != nil {
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		log.Printf("failed to write response: %v", err)
	}
}

func NewServer(Addr string) *http.Server {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	mux := http.NewServeMux()

	mux.Handle("/", http.FileServer(http.Dir("web/static")))

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	// Add handler for web/content/in/ which contains .md files
	mux.HandleFunc("/content/in/", markdownHandler)

	loggedMux := loggingMiddleware(mux)

	server := &http.Server{
		Addr:    Addr,
		Handler: loggedMux,
	}
	return server
}
