package handler

import (
	"embed"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/bonheur/db-studio/internal/session"
	"github.com/bonheur/db-studio/internal/template"
)

//go:embed all:testdata
var testFS embed.FS

func testHandler(t *testing.T) *Handler {
	t.Helper()
	sub, err := fs.Sub(testFS, "testdata")
	if err != nil {
		t.Fatalf("fs.Sub failed: %v", err)
	}
	tmpl := template.NewEngine(sub, false)
	sess := session.NewManager()
	return New(tmpl, sess)
}

func TestNew(t *testing.T) {
	h := testHandler(t)
	if h == nil {
		t.Fatal("New returned nil")
	}
	h.sess.Stop()
}

func TestIndexHandler(t *testing.T) {
	h := testHandler(t)
	defer h.sess.Stop()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	h.Index(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Index handler status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType != "text/html; charset=utf-8" {
		t.Errorf("Content-Type = %q, want %q", contentType, "text/html; charset=utf-8")
	}

	body := w.Body.String()
	if body == "" {
		t.Error("Index handler returned empty body")
	}
}

func TestIndexWithSession(t *testing.T) {
	h := testHandler(t)
	defer h.sess.Stop()

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Session-Id", "test-session-id")
	w := httptest.NewRecorder()

	h.Index(w, req)

	resp := w.Result()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Index handler status = %d, want %d", resp.StatusCode, http.StatusOK)
	}
}
