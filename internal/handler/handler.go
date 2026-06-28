package handler

import (
	"net/http"

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
