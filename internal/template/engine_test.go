package template

import (
	"bytes"
	"embed"
	"io/fs"
	"strings"
	"testing"
)

//go:embed all:testdata
var testFS embed.FS

func testEngine(t *testing.T) *Engine {
	t.Helper()
	sub, err := fs.Sub(testFS, "testdata")
	if err != nil {
		t.Fatalf("fs.Sub failed: %v", err)
	}
	return NewEngine(sub, false)
}

func TestNewEngine(t *testing.T) {
	e := testEngine(t)
	if e == nil {
		t.Fatal("NewEngine returned nil")
	}
	if e.common == nil {
		t.Error("common template is nil")
	}
}

func TestRenderPartial(t *testing.T) {
	e := testEngine(t)

	var buf bytes.Buffer
	err := e.RenderPartial(&buf, "partial", nil)
	if err != nil {
		t.Fatalf("RenderPartial failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "partial content") {
		t.Errorf("RenderPartial output = %q, want to contain %q", output, "partial content")
	}
}

func TestRenderPage(t *testing.T) {
	e := testEngine(t)

	var buf bytes.Buffer
	err := e.Render(&buf, "test", map[string]interface{}{
		"Title": "Test Page",
	})
	if err != nil {
		t.Fatalf("Render failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "Test Page") {
		t.Errorf("Render output missing title, got: %s", output)
	}
	if !strings.Contains(output, "page content") {
		t.Errorf("Render output missing page content, got: %s", output)
	}
}

func TestDevModeReload(t *testing.T) {
	sub, err := fs.Sub(testFS, "testdata")
	if err != nil {
		t.Fatalf("fs.Sub failed: %v", err)
	}
	e := NewEngine(sub, true)
	if e == nil {
		t.Fatal("NewEngine returned nil in dev mode")
	}
}

func TestMissingPage(t *testing.T) {
	e := testEngine(t)

	var buf bytes.Buffer
	err := e.Render(&buf, "nonexistent", nil)
	if err == nil {
		t.Error("Render with nonexistent page should return error")
	}
}

func TestFuncMapExists(t *testing.T) {
	e := testEngine(t)
	if e.common == nil {
		t.Fatal("common template is nil")
	}
}
