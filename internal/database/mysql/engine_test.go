package mysql

import (
	"database/sql"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/go-sql-driver/mysql"

	"github.com/bonheur/db-studio/internal/model"
)

func setupMock(t *testing.T) (*engine, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	e := &engine{db: db}
	return e, mock
}

func setupMockWithPing(t *testing.T) (*engine, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New(sqlmock.MonitorPingsOption(true))
	if err != nil {
		t.Fatalf("failed to create sqlmock: %v", err)
	}
	e := &engine{db: db}
	return e, mock
}

func TestName(t *testing.T) {
	e := &engine{}
	if e.Name() != "mysql" {
		t.Errorf("Name() = %q, want %q", e.Name(), "mysql")
	}
}

func hasMySQL() bool {
	db, err := sql.Open("mysql", "root:@tcp(127.0.0.1:3306)/")
	if err != nil {
		return false
	}
	defer db.Close()
	return db.Ping() == nil
}

func TestConnect(t *testing.T) {
	if !hasMySQL() {
		t.Skip("MySQL not available")
	}
	e := &engine{}
	err := e.Connect(model.ConnectionConfig{
		Host:     "127.0.0.1",
		Port:     3306,
		User:     "root",
		Password: "",
		Database: "test",
	})
	if err != nil {
		t.Fatalf("Connect() error = %v", err)
	}
	defer e.Close()
}

func TestConnect_ConnStringDSN(t *testing.T) {
	if !hasMySQL() {
		t.Skip("MySQL not available")
	}
	e := &engine{}
	err := e.Connect(model.ConnectionConfig{
		ConnString: "root:@tcp(127.0.0.1:3306)/test?parseTime=true",
	})
	if err != nil {
		t.Fatalf("Connect with DSN error = %v", err)
	}
	defer e.Close()
}

func TestConnect_ConnStringURL(t *testing.T) {
	if !hasMySQL() {
		t.Skip("MySQL not available")
	}
	e := &engine{}
	err := e.Connect(model.ConnectionConfig{
		ConnString: "mysql://root:@127.0.0.1:3306/test",
	})
	if err != nil {
		t.Fatalf("Connect with URL error = %v", err)
	}
	defer e.Close()
}

func TestClose(t *testing.T) {
	e, _ := setupMock(t)
	e.db.Close()

	err := e.Close()
	if err != nil {
		t.Errorf("Close() error = %v", err)
	}
}

func TestPing(t *testing.T) {
	e, mock := setupMockWithPing(t)

	mock.ExpectPing()

	err := e.Ping()
	if err != nil {
		t.Errorf("Ping() error = %v", err)
	}
}

func TestPing_Error(t *testing.T) {
	e, mock := setupMockWithPing(t)

	mock.ExpectPing().WillReturnError(sql.ErrConnDone)

	err := e.Ping()
	if err == nil {
		t.Error("Ping() should return error when mock expects error")
	}
}

func TestGetDatabases(t *testing.T) {
	e, mock := setupMock(t)

	rows := sqlmock.NewRows([]string{"SCHEMA_NAME"}).
		AddRow("testdb").
		AddRow("mydb").
		AddRow("analytics")

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT SCHEMA_NAME FROM information_schema.SCHEMATA
		WHERE SCHEMA_NAME NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys')
		ORDER BY SCHEMA_NAME`)).
		WillReturnRows(rows)

	databases, err := e.GetDatabases()
	if err != nil {
		t.Fatalf("GetDatabases() error = %v", err)
	}
	if len(databases) != 3 {
		t.Errorf("GetDatabases() returned %d databases, want 3", len(databases))
	}
	if databases[0] != "testdb" {
		t.Errorf("databases[0] = %q, want %q", databases[0], "testdb")
	}
}

func TestGetDatabases_Empty(t *testing.T) {
	e, mock := setupMock(t)

	rows := sqlmock.NewRows([]string{"SCHEMA_NAME"})
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT SCHEMA_NAME`)).
		WillReturnRows(rows)

	databases, _ := e.GetDatabases()
	if len(databases) != 0 {
		t.Errorf("GetDatabases() returned %d databases, want 0", len(databases))
	}
}

func TestGetTables(t *testing.T) {
	e, mock := setupMock(t)

	rows := sqlmock.NewRows([]string{"TABLE_NAME", "TABLE_TYPE", "TABLE_ROWS", "TABLE_COMMENT"}).
		AddRow("users", "BASE TABLE", int64(100), "User accounts").
		AddRow("posts", "BASE TABLE", int64(500), "").
		AddRow("active_users", "VIEW", int64(0), "Active user view")

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT TABLE_NAME, TABLE_TYPE, IFNULL(TABLE_ROWS, 0), IFNULL(TABLE_COMMENT, '')
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME`)).
		WithArgs("testdb").
		WillReturnRows(rows)

	tables, err := e.GetTables("testdb")
	if err != nil {
		t.Fatalf("GetTables() error = %v", err)
	}
	if len(tables) != 3 {
		t.Errorf("GetTables() returned %d tables, want 3", len(tables))
	}
	if tables[0].Name != "users" || tables[0].Type != "TABLE" || tables[0].Rows != 100 {
		t.Errorf("users table info mismatch: %+v", tables[0])
	}
	if tables[2].Name != "active_users" || tables[2].Type != "VIEW" {
		t.Errorf("view info mismatch: %+v", tables[2])
	}
}

func TestGetColumns(t *testing.T) {
	e, mock := setupMock(t)

	rows := sqlmock.NewRows([]string{
		"COLUMN_NAME", "DATA_TYPE", "CHARACTER_MAXIMUM_LENGTH",
		"NUMERIC_PRECISION", "NUMERIC_SCALE",
		"IS_NULLABLE", "IS_PK", "COLUMN_DEFAULT", "COLUMN_COMMENT", "COLUMN_TYPE",
	}).
		AddRow("id", "int", int64(0), int64(11), int64(0), 0, 1, "", "", "int(11)").
		AddRow("name", "varchar", int64(255), int64(0), int64(0), 1, 0, "", "User's name", "varchar(255)").
		AddRow("email", "varchar", int64(255), int64(0), int64(0), 1, 0, "", "", "varchar(255)")

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT COLUMN_NAME, DATA_TYPE,
		IFNULL(CHARACTER_MAXIMUM_LENGTH, 0), IFNULL(NUMERIC_PRECISION, 0), IFNULL(NUMERIC_SCALE, 0),
		IF(IS_NULLABLE = 'YES', 1, 0),
		IF(COLUMN_KEY = 'PRI', 1, 0),
		IFNULL(COLUMN_DEFAULT, ''),
		IFNULL(COLUMN_COMMENT, ''),
		COLUMN_TYPE
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`)).
		WithArgs("testdb", "users").
		WillReturnRows(rows)

	columns, err := e.GetColumns("testdb", "users")
	if err != nil {
		t.Fatalf("GetColumns() error = %v", err)
	}
	if len(columns) != 3 {
		t.Errorf("GetColumns() returned %d columns, want 3", len(columns))
	}

	if !columns[0].PrimaryKey {
		t.Error("id should be primary key")
	}
	if columns[1].Name != "name" || !columns[1].Nullable {
		t.Errorf("name column mismatch: %+v", columns[1])
	}
}

func TestGetIndexes(t *testing.T) {
	e, mock := setupMock(t)

	rows := sqlmock.NewRows([]string{"INDEX_NAME", "COLUMN_NAME", "NON_UNIQUE", "INDEX_TYPE"}).
		AddRow("PRIMARY", "id", 0, "BTREE").
		AddRow("idx_email", "email", 0, "BTREE").
		AddRow("idx_name_status", "name", 1, "BTREE").
		AddRow("idx_name_status", "status", 1, "BTREE")

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE, INDEX_TYPE
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX`)).
		WithArgs("testdb", "users").
		WillReturnRows(rows)

	indexes, err := e.GetIndexes("testdb", "users")
	if err != nil {
		t.Fatalf("GetIndexes() error = %v", err)
	}
	if len(indexes) != 3 {
		t.Errorf("GetIndexes() returned %d indexes, want 3", len(indexes))
	}

	if !indexes[0].Primary {
		t.Error("PRIMARY should be primary")
	}
	if !indexes[1].Unique {
		t.Error("idx_email should be unique")
	}
	if indexes[2].Unique {
		t.Error("idx_name_status should not be unique")
	}
	if len(indexes[2].Columns) != 2 {
		t.Errorf("idx_name_status should have 2 columns, got %d", len(indexes[2].Columns))
	}
}

func TestExecuteQuery(t *testing.T) {
	e, mock := setupMock(t)

	columns := []string{"id", "name", "email"}
	rows := sqlmock.NewRows(columns).
		AddRow(int64(1), "Alice", "alice@test.com").
		AddRow(int64(2), "Bob", "bob@test.com")

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, name, email FROM users LIMIT 10")).
		WillReturnRows(rows)

	result, err := e.ExecuteQuery("SELECT id, name, email FROM users LIMIT 10")
	if err != nil {
		t.Fatalf("ExecuteQuery() error = %v", err)
	}
	if result.Error != "" {
		t.Fatalf("ExecuteQuery() result error = %s", result.Error)
	}
	if len(result.Columns) != 3 {
		t.Errorf("got %d columns, want 3", len(result.Columns))
	}
	if result.Total != 2 {
		t.Errorf("got %d rows, want 2", result.Total)
	}
	if result.Time == "" {
		t.Error("result.Time should not be empty")
	}
}

func TestExecuteQuery_WithNull(t *testing.T) {
	e, mock := setupMock(t)

	rows := sqlmock.NewRows([]string{"id", "name", "deleted_at"}).
		AddRow(int64(1), "Alice", nil)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM users WHERE id = 1")).
		WillReturnRows(rows)

	result, err := e.ExecuteQuery("SELECT * FROM users WHERE id = 1")
	if err != nil {
		t.Fatalf("ExecuteQuery() error = %v", err)
	}
	if result.Total != 1 {
		t.Errorf("got %d rows, want 1", result.Total)
	}
	if result.Rows[0][2] != nil {
		t.Errorf("deleted_at should be nil, got %v", result.Rows[0][2])
	}
}

func TestExecuteQuery_WithTime(t *testing.T) {
	e, mock := setupMock(t)

	now := time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC)
	rows := sqlmock.NewRows([]string{"id", "created_at"}).
		AddRow(int64(1), now)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT id, created_at FROM users")).
		WillReturnRows(rows)

	result, err := e.ExecuteQuery("SELECT id, created_at FROM users")
	if err != nil {
		t.Fatalf("ExecuteQuery() error = %v", err)
	}
	if result.Rows[0][1] != "2024-01-15 10:30:00" {
		t.Errorf("time value = %v, want 2024-01-15 10:30:00", result.Rows[0][1])
	}
}

func TestExecuteQuery_Error(t *testing.T) {
	e, mock := setupMock(t)

	mock.ExpectQuery(regexp.QuoteMeta("SELECT * FROM nonexistent")).
		WillReturnError(sql.ErrNoRows)

	result, err := e.ExecuteQuery("SELECT * FROM nonexistent")
	if err != nil {
		t.Fatalf("ExecuteQuery() should not return error for query errors: %v", err)
	}
	if result.Error == "" {
		t.Error("result.Error should not be empty for query errors")
	}
}

func TestExecuteUpdate(t *testing.T) {
	e, mock := setupMock(t)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE users SET name = 'Test' WHERE id = 1")).
		WillReturnResult(sqlmock.NewResult(1, 1))

	affected, err := e.ExecuteUpdate("UPDATE users SET name = 'Test' WHERE id = 1")
	if err != nil {
		t.Fatalf("ExecuteUpdate() error = %v", err)
	}
	if affected != 1 {
		t.Errorf("affected = %d, want 1", affected)
	}
}

func TestExecuteUpdate_Error(t *testing.T) {
	e, mock := setupMock(t)

	mock.ExpectExec(regexp.QuoteMeta("UPDATE nonexistent SET x = 1")).
		WillReturnError(sql.ErrNoRows)

	_, err := e.ExecuteUpdate("UPDATE nonexistent SET x = 1")
	if err == nil {
		t.Error("ExecuteUpdate() should return error")
	}
}

func TestGetServerVersion(t *testing.T) {
	e, mock := setupMock(t)

	rows := sqlmock.NewRows([]string{"VERSION()"}).AddRow("8.0.35")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT VERSION()")).WillReturnRows(rows)

	version, err := e.GetServerVersion()
	if err != nil {
		t.Fatalf("GetServerVersion() error = %v", err)
	}
	if version != "8.0.35" {
		t.Errorf("version = %q, want %q", version, "8.0.35")
	}
}

func TestFormatValue(t *testing.T) {
	tests := []struct {
		input any
		want  any
	}{
		{nil, nil},
		{int64(42), int64(42)},
		{float64(3.14), float64(3.14)},
		{"hello", "hello"},
		{[]byte("bytes"), "bytes"},
		{time.Date(2024, 1, 15, 10, 30, 0, 0, time.UTC), "2024-01-15 10:30:00"},
		{true, true},
	}

	for _, tt := range tests {
		got := formatValue(tt.input)
		if got != tt.want {
			t.Errorf("formatValue(%v) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestBuildConfig_Defaults(t *testing.T) {
	cfg, err := buildConfig(model.ConnectionConfig{
		User:     "root",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("buildConfig() error = %v", err)
	}
	if cfg.Addr != "127.0.0.1:3306" {
		t.Errorf("Addr = %q, want %q", cfg.Addr, "127.0.0.1:3306")
	}
	if cfg.User != "root" {
		t.Errorf("User = %q, want %q", cfg.User, "root")
	}
}

func TestBuildConfig_WithPort(t *testing.T) {
	cfg, err := buildConfig(model.ConnectionConfig{
		Host: "db.example.com",
		Port: 3307,
		User: "admin",
	})
	if err != nil {
		t.Fatalf("buildConfig() error = %v", err)
	}
	if cfg.Addr != "db.example.com:3307" {
		t.Errorf("Addr = %q, want %q", cfg.Addr, "db.example.com:3307")
	}
}

func TestParseConnString_DSN(t *testing.T) {
	cfg, err := parseConnString("user:pass@tcp(localhost:3306)/testdb?parseTime=true")
	if err != nil {
		t.Fatalf("parseConnString() error = %v", err)
	}
	if cfg.User != "user" {
		t.Errorf("User = %q, want %q", cfg.User, "user")
	}
	if cfg.Passwd != "pass" {
		t.Errorf("Passwd = %q, want %q", cfg.Passwd, "pass")
	}
}

func TestParseConnString_URL(t *testing.T) {
	cfg, err := parseConnString("mysql://admin:secret@dbhost:3306/mydb")
	if err != nil {
		t.Fatalf("parseConnString() error = %v", err)
	}
	if cfg.User != "admin" {
		t.Errorf("User = %q, want %q", cfg.User, "admin")
	}
	if cfg.Passwd != "secret" {
		t.Errorf("Passwd = %q, want %q", cfg.Passwd, "secret")
	}
	if cfg.Addr != "dbhost:3306" {
		t.Errorf("Addr = %q, want %q", cfg.Addr, "dbhost:3306")
	}
	if cfg.DBName != "mydb" {
		t.Errorf("DBName = %q, want %q", cfg.DBName, "mydb")
	}
}

func TestParseConnString_URLWithParams(t *testing.T) {
	cfg, err := parseConnString("mysql://user:pass@localhost:3306/test?parseTime=true&charset=utf8")
	if err != nil {
		t.Fatalf("parseConnString() error = %v", err)
	}
	if cfg.Params["parseTime"] != "true" {
		t.Errorf("parseTime param = %q", cfg.Params["parseTime"])
	}
}

func TestParseConnString_Invalid(t *testing.T) {
	_, err := parseConnString("://invalid")
	if err == nil {
		t.Error("parseConnString() should return error for invalid URL")
	}
}

func TestParseConnString_BadDSN(t *testing.T) {
	_, err := parseConnString("not a valid dsn @@@")
	if err == nil {
		t.Error("parseConnString() should return error for bad DSN")
	}
}
