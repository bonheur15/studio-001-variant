package model

import (
	"encoding/json"
	"testing"
)

func TestConnectionConfigJSON(t *testing.T) {
	cfg := ConnectionConfig{
		Driver:   "mysql",
		Host:     "localhost",
		Port:     3306,
		User:     "root",
		Password: "secret",
		Database: "testdb",
	}

	data, err := json.Marshal(cfg)
	if err != nil {
		t.Fatalf("json.Marshal failed: %v", err)
	}

	var decoded ConnectionConfig
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("json.Unmarshal failed: %v", err)
	}

	if decoded.Driver != "mysql" {
		t.Errorf("Driver = %q, want %q", decoded.Driver, "mysql")
	}
	if decoded.Port != 3306 {
		t.Errorf("Port = %d, want %d", decoded.Port, 3306)
	}
}

func TestConnectionConfigConnString(t *testing.T) {
	cfg := ConnectionConfig{
		ConnString: "mysql://user:pass@localhost:3306/db",
	}

	if cfg.ConnString != "mysql://user:pass@localhost:3306/db" {
		t.Errorf("ConnString = %q", cfg.ConnString)
	}
}

func TestQueryResult(t *testing.T) {
	r := QueryResult{
		Columns: []string{"id", "name"},
		Rows: [][]any{
			{1, "Alice"},
			{2, "Bob"},
		},
		Total: 2,
		Time:  "1.2ms",
	}

	if len(r.Columns) != 2 {
		t.Errorf("len(Columns) = %d, want 2", len(r.Columns))
	}
	if r.Total != 2 {
		t.Errorf("Total = %d, want 2", r.Total)
	}
}

func TestQueryResultError(t *testing.T) {
	r := QueryResult{
		Error: "connection refused",
	}

	if r.Error != "connection refused" {
		t.Errorf("Error = %q", r.Error)
	}
}

func TestPageData(t *testing.T) {
	pd := PageData{
		Title:     "Test",
		SessionID: "abc123",
		Page:      "index",
		Data:      map[string]string{"key": "value"},
	}

	if pd.Title != "Test" {
		t.Errorf("Title = %q", pd.Title)
	}
	if pd.SessionID != "abc123" {
		t.Errorf("SessionID = %q", pd.SessionID)
	}
}

func TestTableInfoDefaults(t *testing.T) {
	ti := TableInfo{}
	if ti.Type != "" {
		t.Errorf("Type should be empty by default")
	}
}

func TestColumnInfoPrimaryKey(t *testing.T) {
	ci := ColumnInfo{Name: "id", PrimaryKey: true}
	if !ci.PrimaryKey {
		t.Error("PrimaryKey should be true")
	}
}
