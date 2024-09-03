package server

import (
	"bytes"
	"errors"
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

type MarkdownHTML struct {
	HTML  []byte
	Title string
}

// see: https://blog.kowalczyk.info/article/cxn3/advanced-markdown-processing-in-go.html

func extractTitle(node ast.Node) (string, error) {
	var title string
	ast.WalkFunc(node, func(node ast.Node, entering bool) ast.WalkStatus {
		if entering {
			if heading, ok := node.(*ast.Heading); ok {
				if heading.Level == 1 {
					var buf bytes.Buffer
					for _, child := range heading.Children {
						if text, ok := child.(*ast.Text); ok {
							buf.Write(text.Literal)
						}
					}
					title = buf.String()
					return ast.Terminate
				}
			}
		}
		return ast.GoToNext
	})
	if title == "" {
		return "", errors.New("no title found")
	}
	return title, nil
}

func mdToHTML(md []byte) (MarkdownHTML, error) {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(md)

	title, err := extractTitle(doc)
	if err != nil {
		return MarkdownHTML{}, err
	}

	highlightStyle := styles.Get("monokai")
	formatter := html.New(html.Standalone(false), html.TabWidth(4))
	htmlHighlight := NewHTMLHighligher(formatter, highlightStyle)
	codeBlockRenderHook := NewCodeBlockRenderHook(htmlHighlight)
	opts := mdhtml.RendererOptions{
		Flags:          mdhtml.CommonFlags | mdhtml.HrefTargetBlank,
		RenderNodeHook: codeBlockRenderHook,
	}
	renderer := mdhtml.NewRenderer(opts)
	return MarkdownHTML{HTML: markdown.Render(doc, renderer), Title: title}, nil
}

// for rendered full pages
type Page struct {
	Title string
	URL   string
	HTML  []byte
}

// for use with index.html.tmpl
type IndexData struct {
	Title   string
	Body    []byte
	NewPost PostData
}

// for use with page-[blank/prose].html.tmpl
type PageData struct {
	Title string
	Body  []byte
}

// for use with post.html.tmpl
type PostData struct {
	Title string
	Body  []byte
	URL   string
}

// for use with posts.html.tmpl
type PostsData struct {
	Title string
	Posts []PostData
}

func getIndexPage(newPost PostData) Page {
	indexTemplate := template.Must(template.ParseFiles("web/templates/base.html.tmpl", "web/templates/page-blank.html.tmpl", "web/templates/index.html.tmpl"))
	indexData := IndexData{Title: "index", NewPost: newPost}
	var indexHTML bytes.Buffer
	err := indexTemplate.Execute(&indexHTML, indexData)
	if err != nil {
		log.Fatalf("failed to execute template: %v", err)
	}
	return Page{Title: "index", HTML: indexHTML.Bytes(), URL: "/"}
}

// TODO: refactor
func getPages() (map[string]Page, PostsData) {
	pages := make(map[string]Page)

	// templates
	// TODO: is this how you do templates?
	postTemplate := template.Must(template.ParseFiles("web/templates/base.html.tmpl", "web/templates/post.html.tmpl"))
	postsTemplate := template.Must(template.ParseFiles("web/templates/base.html.tmpl", "web/templates/posts.html.tmpl"))
	pageBlankTemplate := template.Must(template.ParseFiles("web/templates/base.html.tmpl", "web/templates/page-blank.html.tmpl"))
	pageProseTemplate := template.Must(template.ParseFiles("web/templates/base.html.tmpl", "web/templates/page-prose.html.tmpl"))

	// /{post}
	postsDir := "web/content/posts"
	postFiles, err := os.ReadDir(postsDir)
	if err != nil {
		log.Fatalf("failed to read posts directory: %v", err)
	}
	postMdFiles := []string{}
	for _, file := range postFiles {
		if !file.IsDir() && filepath.Ext(file.Name()) == ".md" {
			postMdFiles = append(postMdFiles, filepath.Join(postsDir, file.Name()))
		}
	}
	postsData := PostsData{Title: "posts", Posts: []PostData{}}
	for _, file := range postMdFiles {
		contents, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("failed to read file: %v", err)
		}

		filename := filepath.Base(file)
		ext := filepath.Ext(filename)
		url := "/" + strings.TrimSuffix(filename[len("YYYYMMDD-"):], ext)
		title := strings.TrimSuffix(filename[len("YYYYMMDD-"):], ext) // TODO: extract first H1 from md

		c, err := mdToHTML(contents)
		if err != nil {
			log.Fatalf("failed to convert markdown to html for file=%s: %v", file, err)
		}

		var fullHTML bytes.Buffer
		postData := PostData{Title: c.Title, Body: c.HTML, URL: url}
		err = postTemplate.Execute(&fullHTML, postData)
		if err != nil {
			log.Fatalf("failed to execute template: %v", err)
		}

		postsData.Posts = append([]PostData{postData}, postsData.Posts...) // prepend to sort desc
		pages[title] = Page{Title: title, HTML: fullHTML.Bytes(), URL: url}
	}

	// /{page}
	pagesDir := "web/content/"
	pageFiles, err := os.ReadDir(pagesDir)
	if err != nil {
		log.Fatalf("failed to read posts directory: %v", err)
	}
	pageProseFiles := []string{}
	pageBlankFiles := []string{}
	for _, file := range pageFiles {
		if file.IsDir() {
			continue
		}
		if filepath.Ext(file.Name()) == ".md" {
			pageProseFiles = append(pageProseFiles, filepath.Join(pagesDir, file.Name()))
		} else if filepath.Ext(file.Name()) == ".html" {
			pageBlankFiles = append(pageBlankFiles, filepath.Join(pagesDir, file.Name()))
		}
	}

	for _, file := range pageProseFiles {
		contents, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("failed to read file: %v", err)
		}

		filename := filepath.Base(file)
		ext := filepath.Ext(filename)
		title := strings.TrimSuffix(filename, ext)

		url := "/" + title

		b, err := mdToHTML(contents)
		if err != nil {
			log.Fatalf("failed to convert markdown to html for file=%s: %v", file, err)
		}

		var fullHTML bytes.Buffer
		err = pageProseTemplate.Execute(&fullHTML, PageData{Title: b.Title, Body: b.HTML})
		if err != nil {
			log.Fatalf("failed to execute template: %v", err)
		}

		pages[title] = Page{Title: title, HTML: fullHTML.Bytes(), URL: url}
	}

	for _, file := range pageBlankFiles {
		contents, err := os.ReadFile(file)
		if err != nil {
			log.Fatalf("failed to read file: %v", err)
		}

		filename := filepath.Base(file)
		ext := filepath.Ext(filename)
		title := strings.TrimSuffix(filename, ext)

		url := "/" + title

		body := contents

		var fullHTML bytes.Buffer
		err = pageBlankTemplate.Execute(&fullHTML, PageData{Title: title, Body: body})
		if err != nil {
			log.Fatalf("failed to execute template: %v", err)
		}

		pages[title] = Page{Title: title, HTML: fullHTML.Bytes(), URL: url}
	}

	// /posts
	var fullHTML bytes.Buffer
	err = postsTemplate.Execute(&fullHTML, postsData)
	if err != nil {
		log.Fatalf("failed to execute template: %v", err)
	}
	pages["posts"] = Page{Title: "posts", HTML: fullHTML.Bytes(), URL: "/posts"}

	return pages, postsData
}

func NewServer(Addr string) *http.Server {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	mux := http.NewServeMux()

	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("web/static/"))))

	pages, postsData := getPages()
	for title, page := range pages {
		mux.HandleFunc("/"+title, func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			_, err := w.Write(page.HTML)
			if err != nil {
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
				log.Printf("failed to write response: %v", err)
			}
		})
		log.Printf("registered page: %s to %s", title, page.URL)
	}

	mux.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("OK"))
	})

	indexPage := getIndexPage(postsData.Posts[0])
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html")
		_, err := w.Write(indexPage.HTML)
		if err != nil {
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			log.Printf("failed to write response: %v", err)
		}
	})

	loggedMux := loggingMiddleware(mux)

	server := &http.Server{
		Addr:    Addr,
		Handler: loggedMux,
	}
	return server
}
