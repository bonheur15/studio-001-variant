package database

import (
	"testing"

	"github.com/bonheur/db-studio/internal/model"
)

func TestRegisterAndCreate(t *testing.T) {
	// Reset global registry for test
	old := global
	global = &Registry{factories: make(map[string]Factory)}
	defer func() { global = old }()

	Register("testdb", func(config model.ConnectionConfig) (Engine, error) {
		return &mockEngine{name: "testdb"}, nil
	})

	available := Available()
	if len(available) != 1 || available[0] != "testdb" {
		t.Errorf("Available = %v, want [testdb]", available)
	}

	engine, err := Create("testdb", model.ConnectionConfig{})
	if err != nil {
		t.Fatalf("Create failed: %v", err)
	}
	if engine.Name() != "testdb" {
		t.Errorf("Engine.Name = %q, want %q", engine.Name(), "testdb")
	}
}

func TestCreateUnregistered(t *testing.T) {
	old := global
	global = &Registry{factories: make(map[string]Factory)}
	defer func() { global = old }()

	_, err := Create("nonexistent", model.ConnectionConfig{})
	if err == nil {
		t.Error("Create with unregistered driver should return error")
	}
}

func TestRegisterDuplicate(t *testing.T) {
	old := global
	global = &Registry{factories: make(map[string]Factory)}
	defer func() { global = old }()

	Register("testdb", func(config model.ConnectionConfig) (Engine, error) {
		return &mockEngine{name: "testdb"}, nil
	})

	defer func() {
		if r := recover(); r == nil {
			t.Error("Register duplicate should panic")
		}
	}()

	Register("testdb", func(config model.ConnectionConfig) (Engine, error) {
		return &mockEngine{name: "testdb"}, nil
	})
}

func TestAvailableEmpty(t *testing.T) {
	old := global
	global = &Registry{factories: make(map[string]Factory)}
	defer func() { global = old }()

	available := Available()
	if len(available) != 0 {
		t.Errorf("Available = %v, want empty", available)
	}
}

type mockEngine struct {
	name string
}

func (m *mockEngine) Name() string                                       { return m.name }
func (m *mockEngine) Connect(config model.ConnectionConfig) error        { return nil }
func (m *mockEngine) Close() error                                       { return nil }
func (m *mockEngine) Ping() error                                        { return nil }
func (m *mockEngine) GetDatabases() ([]string, error)                    { return nil, nil }
func (m *mockEngine) GetTables(database string) ([]model.TableInfo, error) { return nil, nil }
func (m *mockEngine) GetColumns(database, table string) ([]model.ColumnInfo, error) { return nil, nil }
func (m *mockEngine) GetIndexes(database, table string) ([]model.IndexInfo, error) { return nil, nil }
func (m *mockEngine) ExecuteQuery(query string) (*model.QueryResult, error) { return nil, nil }
func (m *mockEngine) ExecuteUpdate(query string) (int64, error)          { return 0, nil }
func (m *mockEngine) GetServerVersion() (string, error)                  { return "", nil }
