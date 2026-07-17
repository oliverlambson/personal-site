package server

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"os"
	"path"
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
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock | parser.Footnotes
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

func getIndexPage(content fs.FS, newPost PostData) (Page, error) {
	indexTemplate, err := template.ParseFS(content, "templates/base.html.tmpl", "templates/page-blank.html.tmpl", "templates/index.html.tmpl")
	if err != nil {
		return Page{}, fmt.Errorf("parse index templates: %w", err)
	}
	indexData := IndexData{Title: "index", NewPost: newPost}
	var indexHTML bytes.Buffer
	err = indexTemplate.Execute(&indexHTML, indexData)
	if err != nil {
		return Page{}, fmt.Errorf("execute index template: %w", err)
	}
	return Page{Title: "index", HTML: indexHTML.Bytes(), URL: "/"}, nil
}

// TODO: refactor
func getPages(content fs.FS) (map[string]Page, PostsData, error) {
	pages := make(map[string]Page)

	// templates
	// TODO: is this how you do templates?
	postTemplate, err := template.ParseFS(content, "templates/base.html.tmpl", "templates/post.html.tmpl")
	if err != nil {
		return nil, PostsData{}, fmt.Errorf("parse post templates: %w", err)
	}
	postsTemplate, err := template.ParseFS(content, "templates/base.html.tmpl", "templates/posts.html.tmpl")
	if err != nil {
		return nil, PostsData{}, fmt.Errorf("parse posts templates: %w", err)
	}
	pageBlankTemplate, err := template.ParseFS(content, "templates/base.html.tmpl", "templates/page-blank.html.tmpl")
	if err != nil {
		return nil, PostsData{}, fmt.Errorf("parse blank page templates: %w", err)
	}
	pageProseTemplate, err := template.ParseFS(content, "templates/base.html.tmpl", "templates/page-prose.html.tmpl")
	if err != nil {
		return nil, PostsData{}, fmt.Errorf("parse prose page templates: %w", err)
	}

	// /{post}
	postsDir := "content/posts"
	postFiles, err := fs.ReadDir(content, postsDir)
	if err != nil {
		return nil, PostsData{}, fmt.Errorf("read posts directory: %w", err)
	}
	postMdFiles := []string{}
	for _, file := range postFiles {
		if !file.IsDir() && path.Ext(file.Name()) == ".md" {
			postMdFiles = append(postMdFiles, path.Join(postsDir, file.Name()))
		}
	}
	postsData := PostsData{Title: "posts", Posts: []PostData{}}
	for _, file := range postMdFiles {
		contents, err := fs.ReadFile(content, file)
		if err != nil {
			return nil, PostsData{}, fmt.Errorf("read post %q: %w", file, err)
		}

		filename := path.Base(file)
		ext := path.Ext(filename)
		url := "/" + strings.TrimSuffix(filename[len("YYYYMMDD-"):], ext)
		title := strings.TrimSuffix(filename[len("YYYYMMDD-"):], ext) // TODO: extract first H1 from md

		c, err := mdToHTML(contents)
		if err != nil {
			return nil, PostsData{}, fmt.Errorf("convert post %q to HTML: %w", file, err)
		}

		var fullHTML bytes.Buffer
		postData := PostData{Title: c.Title, Body: c.HTML, URL: url}
		err = postTemplate.Execute(&fullHTML, postData)
		if err != nil {
			return nil, PostsData{}, fmt.Errorf("execute post template for %q: %w", file, err)
		}

		postsData.Posts = append([]PostData{postData}, postsData.Posts...) // prepend to sort desc
		pages[title] = Page{Title: title, HTML: fullHTML.Bytes(), URL: url}
	}
	if len(postsData.Posts) == 0 {
		return nil, PostsData{}, errors.New("no posts found")
	}

	// /{page}
	pagesDir := "content"
	pageFiles, err := fs.ReadDir(content, pagesDir)
	if err != nil {
		return nil, PostsData{}, fmt.Errorf("read pages directory: %w", err)
	}
	pageProseFiles := []string{}
	pageBlankFiles := []string{}
	for _, file := range pageFiles {
		if file.IsDir() {
			continue
		}
		if path.Ext(file.Name()) == ".md" {
			pageProseFiles = append(pageProseFiles, path.Join(pagesDir, file.Name()))
		} else if path.Ext(file.Name()) == ".html" {
			pageBlankFiles = append(pageBlankFiles, path.Join(pagesDir, file.Name()))
		}
	}

	for _, file := range pageProseFiles {
		contents, err := fs.ReadFile(content, file)
		if err != nil {
			return nil, PostsData{}, fmt.Errorf("read page %q: %w", file, err)
		}

		filename := path.Base(file)
		ext := path.Ext(filename)
		title := strings.TrimSuffix(filename, ext)

		url := "/" + title

		b, err := mdToHTML(contents)
		if err != nil {
			return nil, PostsData{}, fmt.Errorf("convert page %q to HTML: %w", file, err)
		}

		var fullHTML bytes.Buffer
		err = pageProseTemplate.Execute(&fullHTML, PageData{Title: b.Title, Body: b.HTML})
		if err != nil {
			return nil, PostsData{}, fmt.Errorf("execute prose page template for %q: %w", file, err)
		}

		pages[title] = Page{Title: title, HTML: fullHTML.Bytes(), URL: url}
	}

	for _, file := range pageBlankFiles {
		contents, err := fs.ReadFile(content, file)
		if err != nil {
			return nil, PostsData{}, fmt.Errorf("read page %q: %w", file, err)
		}

		filename := path.Base(file)
		ext := path.Ext(filename)
		title := strings.TrimSuffix(filename, ext)

		url := "/" + title

		body := contents

		var fullHTML bytes.Buffer
		err = pageBlankTemplate.Execute(&fullHTML, PageData{Title: title, Body: body})
		if err != nil {
			return nil, PostsData{}, fmt.Errorf("execute blank page template for %q: %w", file, err)
		}

		pages[title] = Page{Title: title, HTML: fullHTML.Bytes(), URL: url}
	}

	// /posts
	var fullHTML bytes.Buffer
	err = postsTemplate.Execute(&fullHTML, postsData)
	if err != nil {
		return nil, PostsData{}, fmt.Errorf("execute posts template: %w", err)
	}
	pages["posts"] = Page{Title: "posts", HTML: fullHTML.Bytes(), URL: "/posts"}

	return pages, postsData, nil
}

func NewServer(addr string, content fs.FS) (*http.Server, error) {
	log.SetOutput(os.Stdout)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	mux := http.NewServeMux()

	static, err := fs.Sub(content, "static")
	if err != nil {
		return nil, fmt.Errorf("open static filesystem: %w", err)
	}
	mux.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(static))))

	pages, postsData, err := getPages(content)
	if err != nil {
		return nil, err
	}
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

	indexPage, err := getIndexPage(content, postsData.Posts[0])
	if err != nil {
		return nil, err
	}
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
		Addr:    addr,
		Handler: loggedMux,
	}
	return server, nil
}
