package model

import "time"

type ConnectionConfig struct {
	Driver     string `json:"driver"`
	Host       string `json:"host"`
	Port       int    `json:"port"`
	User       string `json:"user"`
	Password   string `json:"password"`
	Database   string `json:"database"`
	ConnString string `json:"conn_string"`
}

type ConnectionInfo struct {
	ID        string    `json:"id"`
	Driver    string    `json:"driver"`
	Label     string    `json:"label"`
	Version   string    `json:"version"`
	CreatedAt time.Time `json:"created_at"`
}

type TableInfo struct {
	Name    string `json:"name"`
	Type    string `json:"type"`
	Schema  string `json:"schema"`
	Rows    int64  `json:"rows"`
	Comment string `json:"comment"`
}

type ColumnInfo struct {
	Name       string `json:"name"`
	Type       string `json:"type"`
	Nullable   bool   `json:"nullable"`
	Default    string `json:"default"`
	PrimaryKey bool   `json:"primary_key"`
	Comment    string `json:"comment"`
}

type IndexInfo struct {
	Name    string   `json:"name"`
	Columns []string `json:"columns"`
	Unique  bool     `json:"unique"`
	Primary bool     `json:"primary"`
	Type    string   `json:"type"`
}

type QueryResult struct {
	Columns []string   `json:"columns"`
	Rows    [][]any    `json:"rows"`
	Total   int64      `json:"total"`
	Time    string     `json:"time"`
	Error   string     `json:"error,omitempty"`
}

type PageData struct {
	Title     string
	SessionID string
	Page      string
	Data      any
}
