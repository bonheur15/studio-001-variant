package main

import (
	"embed"
	"io/fs"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"

	"github.com/bonheur/db-studio/internal/handler"
	"github.com/bonheur/db-studio/internal/session"
	"github.com/bonheur/db-studio/internal/template"
)

//go:embed all:web/templates
var templateFS embed.FS

//go:embed all:web/static
var staticFS embed.FS

func main() {
	dev := os.Getenv("DEV") == "true"

	tmpl := template.NewEngine(templateFS, dev)
	sess := session.NewManager()
	defer sess.Stop()
	h := handler.New(tmpl, sess)

	r := chi.NewRouter()
	r.Use(chimw.Logger)
	r.Use(chimw.Recoverer)
	r.Use(chimw.Timeout(60 * time.Second))
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			next.ServeHTTP(w, r)
		})
	})

	staticSub, err := fs.Sub(staticFS, "web/static")
	if err != nil {
		log.Fatal(err)
	}
	r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))

	r.Get("/", h.Index)

	addr := ":8080"
	log.Printf("DB Studio starting on http://localhost%s", addr)
	if err := http.ListenAndServe(addr, r); err != nil {
		log.Fatal(err)
	}
}
