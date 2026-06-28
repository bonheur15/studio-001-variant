package handler

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"github.com/bonheur/db-studio/internal/session"
	"github.com/bonheur/db-studio/internal/template"
)

type Handler struct {
	tmpl *template.Engine
	sess *session.Manager
}

func New(tmpl *template.Engine, sess *session.Manager) *Handler {
	return &Handler{
		tmpl: tmpl,
		sess: sess,
	}
}

func (h *Handler) RegisterRoutes(r chi.Router) {
	r.Get("/", h.Index)
	r.Post("/api/connect", h.Connect)
	r.Post("/api/disconnect", h.Disconnect)
	r.Get("/api/databases", h.ListDatabases)
	r.Get("/api/tables", h.ListTables)
	r.Get("/api/table", h.TableDetail)
	r.Get("/api/columns", h.ListColumns)
	r.Get("/api/indexes", h.ListIndexes)
	r.Get("/api/data", h.TableData)
}

func (h *Handler) Index(w http.ResponseWriter, r *http.Request) {
	sessionID := r.Header.Get("X-Session-Id")
	if sessionID == "" {
		sessionID = session.GenerateID()
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := h.tmpl.Render(w, "index", map[string]interface{}{
		"Title":     "DB Studio",
		"SessionID": sessionID,
		"Page":      "index",
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) getSession(r *http.Request) *session.Session {
	sessionID := r.Header.Get("X-Session-Id")
	if sessionID == "" {
		sessionID = r.FormValue("session_id")
	}
	if sessionID == "" {
		return nil
	}
	s, _ := h.sess.Get(sessionID)
	return s
}

func (h *Handler) getOrCreateSession(r *http.Request) *session.Session {
	sessionID := r.Header.Get("X-Session-Id")
	if sessionID == "" {
		sessionID = r.FormValue("session_id")
	}
	if sessionID == "" {
		sessionID = session.GenerateID()
	}
	return h.sess.GetOrCreate(sessionID)
}

func quoteIdentifier(name string) string {
	return `"` + strings.ReplaceAll(name, `"`, `""`) + `"`
}
