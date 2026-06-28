package template

import (
	"html/template"
	"io"
	"io/fs"
	"os"
	"strings"
	"sync"
)

type Engine struct {
	fs        fs.FS
	common    *template.Template
	pageCache map[string]*template.Template
	mu        sync.RWMutex
	dev       bool
}

func NewEngine(fsys fs.FS, dev bool) *Engine {
	e := &Engine{
		pageCache: make(map[string]*template.Template),
		dev:       dev,
	}
	if dev {
		e.fs = os.DirFS(".")
	} else {
		e.fs = fsys
	}
	e.loadCommon()
	return e
}

func (e *Engine) loadCommon() {
	funcs := template.FuncMap{
		"add": func(a, b int) int { return a + b },
		"sub": func(a, b int) int { return a - b },
		"seq": func(n int) []int {
			s := make([]int, n)
			for i := range s {
				s[i] = i + 1
			}
			return s
		},
		"hasPrefix":  strings.HasPrefix,
		"trimSuffix": strings.TrimSuffix,
		"mod":        func(a, b int) int { return a % b },
		"even":       func(n int) bool { return n%2 == 0 },
		"isNil":      func(v interface{}) bool { return v == nil },
	}

	tmpl := template.New("").Funcs(funcs)

	var paths []string
	fs.WalkDir(e.fs, "web/templates", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".html") {
			return nil
		}
		if strings.HasPrefix(path, "web/templates/pages/") {
			return nil
		}
		paths = append(paths, path)
		return nil
	})

	for _, p := range paths {
		content, err := fs.ReadFile(e.fs, p)
		if err != nil {
			panic(err)
		}
		tmpl.New(p).Parse(string(content))
	}

	e.common = tmpl
}

func (e *Engine) Render(w io.Writer, page string, data any) error {
	pageTmpl, err := e.getPageTemplate(page)
	if err != nil {
		return err
	}
	return pageTmpl.ExecuteTemplate(w, "base", data)
}

func (e *Engine) RenderPartial(w io.Writer, name string, data any) error {
	tmpl, err := e.common.Clone()
	if err != nil {
		return err
	}
	return tmpl.ExecuteTemplate(w, name, data)
}

func (e *Engine) getPageTemplate(page string) (*template.Template, error) {
	if !e.dev {
		e.mu.RLock()
		cached, ok := e.pageCache[page]
		e.mu.RUnlock()
		if ok {
			return cached, nil
		}
	}

	tmpl, err := e.common.Clone()
	if err != nil {
		return nil, err
	}

	pagePath := "web/templates/pages/" + page + ".html"
	content, err := fs.ReadFile(e.fs, pagePath)
	if err != nil {
		return nil, err
	}
	if _, err := tmpl.New(pagePath).Parse(string(content)); err != nil {
		return nil, err
	}

	if !e.dev {
		e.mu.Lock()
		e.pageCache[page] = tmpl
		e.mu.Unlock()
	}

	return tmpl, nil
}
