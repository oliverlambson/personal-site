package server

import (
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
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

func MdToHTML(md []byte) []byte {
	// see: https://blog.kowalczyk.info/article/cxn3/advanced-markdown-processing-in-go.html
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(md)

	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)

	return markdown.Render(doc, renderer)
}

func markdownHandler(w http.ResponseWriter, r *http.Request) {
	filePath := filepath.Join("web/content/in/", strings.TrimPrefix(r.URL.Path, "/content/in/"))

	md, err := os.ReadFile(filePath)
	if err != nil {
		http.Error(w, "File not found", http.StatusNotFound)
		return
	}

	html := MdToHTML(md)

	w.Header().Set("Content-Type", "text/html")
	w.Write(html)
	// TODO: add title from markdown frontmatter
	// TODO: add style.css
	// TODO: add code highlighting
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
