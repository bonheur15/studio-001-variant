package handler

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/bonheur/db-studio/internal/database"
	"github.com/bonheur/db-studio/internal/model"
)

func (h *Handler) Connect(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	cfg := model.ConnectionConfig{
		Driver:     r.FormValue("driver"),
		Host:       r.FormValue("host"),
		Port:       parseInt(r.FormValue("port"), 0),
		User:       r.FormValue("user"),
		Password:   r.FormValue("password"),
		Database:   r.FormValue("database"),
		ConnString: r.FormValue("conn_string"),
	}

	if cfg.Driver == "" {
		h.renderError(w, "Database driver is required")
		return
	}

	engine, err := database.Create(cfg.Driver, cfg)
	if err != nil {
		h.renderError(w, err.Error())
		return
	}

	sess := h.getOrCreateSession(r)
	connID := "default"
	sess.AddConnection(connID, engine)

	version, _ := engine.GetServerVersion()
	databases, err := engine.GetDatabases()
	if err != nil {
		databases = []string{}
	}

	label := cfg.Host
	if label == "" {
		label = fmt.Sprintf("%s:%d", cfg.Host, cfg.Port)
	}
	if cfg.Database != "" {
		label = label + "/" + cfg.Database
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err = h.tmpl.RenderPartial(w, "connected", map[string]interface{}{
		"Driver":    cfg.Driver,
		"Host":      cfg.Host,
		"Port":      cfg.Port,
		"Database":  cfg.Database,
		"Label":     label,
		"Version":   version,
		"Databases": databases,
		"ConnID":    connID,
	})
	if err != nil {
		h.renderError(w, err.Error())
	}
}

func (h *Handler) Disconnect(w http.ResponseWriter, r *http.Request) {
	sess := h.getOrCreateSession(r)
	sess.RemoveConnection("default")

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := h.tmpl.RenderPartial(w, "connection_form", map[string]interface{}{
		"SessionID": sess.ID,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) renderError(w http.ResponseWriter, msg string) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := h.tmpl.RenderPartial(w, "connection_error", map[string]string{
		"Error": msg,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func parseInt(s string, defaultVal int) int {
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}
