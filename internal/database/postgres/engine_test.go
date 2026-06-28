package postgres

import (
	"database/sql"
	"net/url"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	_ "github.com/jackc/pgx/v5/stdlib"

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
	if e.Name() != "postgres" {
		t.Errorf("Name() = %q, want %q", e.Name(), "postgres")
	}
}

func TestConnect(t *testing.T) {
	e := &engine{}
	err := e.Connect(model.ConnectionConfig{
		Host:     "127.0.0.1",
		Port:     5432,
		User:     "postgres",
		Password: "postgres",
		Database: "test",
	})
	if err != nil {
		t.Logf("Connect failed (expected without real server): %v", err)
	}
}

func TestConnect_ConnString(t *testing.T) {
	e := &engine{}
	err := e.Connect(model.ConnectionConfig{
		ConnString: "postgres://postgres:postgres@127.0.0.1:5432/test?sslmode=disable",
	})
	if err != nil {
		t.Logf("Connect with URL failed (expected without real server): %v", err)
	}
}

func TestConnect_ConnStringWithoutPrefix(t *testing.T) {
	e := &engine{}
	err := e.Connect(model.ConnectionConfig{
		ConnString: "postgres:pass@127.0.0.1:5432/db",
	})
	if err != nil {
		t.Logf("Connect without prefix failed (expected without real server): %v", err)
	}
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

	rows := sqlmock.NewRows([]string{"schema_name"}).
		AddRow("public").
		AddRow("analytics").
		AddRow("app")

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT schema_name FROM information_schema.schemata
		WHERE schema_name NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		ORDER BY schema_name`)).
		WillReturnRows(rows)

	databases, err := e.GetDatabases()
	if err != nil {
		t.Fatalf("GetDatabases() error = %v", err)
	}
	if len(databases) != 3 {
		t.Errorf("GetDatabases() returned %d schemas, want 3", len(databases))
	}
	if databases[0] != "public" {
		t.Errorf("databases[0] = %q, want %q", databases[0], "public")
	}
}

func TestGetTables(t *testing.T) {
	e, mock := setupMock(t)

	rows := sqlmock.NewRows([]string{"table_name", "table_type", "rows", "comment"}).
		AddRow("users", "BASE TABLE", int64(0), "User accounts").
		AddRow("posts", "BASE TABLE", int64(0), "").
		AddRow("active_users", "VIEW", int64(0), "")

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT table_name, table_type, 0,
		COALESCE(obj_description(
			(quote_ident(table_schema) || '.' || quote_ident(table_name))::regclass,
			'pg_class'
		), '')
		FROM information_schema.tables
		WHERE table_schema = $1
		ORDER BY table_name`)).
		WithArgs("public").
		WillReturnRows(rows)

	tables, err := e.GetTables("public")
	if err != nil {
		t.Fatalf("GetTables() error = %v", err)
	}
	if len(tables) != 3 {
		t.Errorf("GetTables() returned %d tables, want 3", len(tables))
	}
	if tables[0].Name != "users" || tables[0].Type != "TABLE" {
		t.Errorf("users table info mismatch: %+v", tables[0])
	}
	if tables[2].Type != "VIEW" {
		t.Errorf("active_users should be VIEW, got %q", tables[2].Type)
	}
}

func TestGetColumns(t *testing.T) {
	e, mock := setupMock(t)

	rows := sqlmock.NewRows([]string{
		"column_name", "data_type", "char_max_len",
		"num_prec", "num_scale",
		"nullable", "is_pk", "col_default", "comment", "full_type",
	}).
		AddRow("id", "integer", 0, 32, 0, 0, 1, "", "", "integer").
		AddRow("name", "character varying", 255, 0, 0, 1, 0, "", "User's name", "character varying").
		AddRow("email", "character varying", 255, 0, 0, 1, 0, "''::text", "", "character varying")

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT
		c.column_name,
		c.data_type,
		COALESCE(c.character_maximum_length::int, 0),
		COALESCE(c.numeric_precision::int, 0),
		COALESCE(c.numeric_scale::int, 0),
		CASE WHEN c.is_nullable = 'YES' THEN 1 ELSE 0 END,
		CASE WHEN EXISTS (
			SELECT 1 FROM information_schema.key_column_usage kcu
			JOIN information_schema.table_constraints tc
				ON kcu.constraint_name = tc.constraint_name
				AND kcu.table_schema = tc.table_schema
				AND kcu.table_name = tc.table_name
			WHERE tc.constraint_type = 'PRIMARY KEY'
				AND kcu.table_schema = c.table_schema
				AND kcu.table_name = c.table_name
				AND kcu.column_name = c.column_name
		) THEN 1 ELSE 0 END,
		COALESCE(c.column_default::text, ''),
		COALESCE(pg_catalog.col_description(
			(quote_ident(c.table_schema) || '.' || quote_ident(c.table_name))::regclass,
			c.ordinal_position
		), ''),
		c.data_type
		FROM information_schema.columns c
		WHERE c.table_schema = $1 AND c.table_name = $2
		ORDER BY c.ordinal_position`)).
		WithArgs("public", "users").
		WillReturnRows(rows)

	columns, err := e.GetColumns("public", "users")
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

	rows := sqlmock.NewRows([]string{"indexname", "indexdef"}).
		AddRow("users_pkey", `CREATE UNIQUE INDEX users_pkey ON public.users USING btree (id)`).
		AddRow("idx_email", `CREATE UNIQUE INDEX idx_email ON public.users USING btree (email)`).
		AddRow("idx_name_status", `CREATE INDEX idx_name_status ON public.users USING btree (name, status)`)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT indexname, indexdef FROM pg_indexes
		WHERE schemaname = $1 AND tablename = $2
		ORDER BY indexname`)).
		WithArgs("public", "users").
		WillReturnRows(rows)

	indexes, err := e.GetIndexes("public", "users")
	if err != nil {
		t.Fatalf("GetIndexes() error = %v", err)
	}
	if len(indexes) != 3 {
		t.Errorf("GetIndexes() returned %d indexes, want 3", len(indexes))
	}

	if !indexes[0].Unique {
		t.Error("users_pkey should be unique")
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

	rows := sqlmock.NewRows([]string{"id", "name", "email"}).
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
	if result.Total != 2 {
		t.Errorf("got %d rows, want 2", result.Total)
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

	rows := sqlmock.NewRows([]string{"version"}).AddRow("PostgreSQL 16.1 on x86_64")
	mock.ExpectQuery(regexp.QuoteMeta("SELECT version()")).WillReturnRows(rows)

	version, err := e.GetServerVersion()
	if err != nil {
		t.Fatalf("GetServerVersion() error = %v", err)
	}
	if version != "PostgreSQL 16.1 on x86_64" {
		t.Errorf("version = %q", version)
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

func TestBuildConnStr_Defaults(t *testing.T) {
	cs, err := buildConnStr(model.ConnectionConfig{
		User:     "postgres",
		Password: "secret",
	})
	if err != nil {
		t.Fatalf("buildConnStr() error = %v", err)
	}

	u, err := url.Parse(cs)
	if err != nil {
		t.Fatalf("invalid URL: %v", err)
	}
	if u.Host != "127.0.0.1:5432" {
		t.Errorf("Host = %q, want %q", u.Host, "127.0.0.1:5432")
	}
	if u.User.Username() != "postgres" {
		t.Errorf("User = %q", u.User.Username())
	}
	if u.Query().Get("sslmode") != "disable" {
		t.Errorf("sslmode = %q, want disable", u.Query().Get("sslmode"))
	}
}

func TestBuildConnStr_WithPort(t *testing.T) {
	cs, err := buildConnStr(model.ConnectionConfig{
		Host:     "db.example.com",
		Port:     5433,
		User:     "admin",
		Password: "pass",
		Database: "mydb",
	})
	if err != nil {
		t.Fatalf("buildConnStr() error = %v", err)
	}

	u, err := url.Parse(cs)
	if err != nil {
		t.Fatalf("invalid URL: %v", err)
	}
	if u.Host != "db.example.com:5433" {
		t.Errorf("Host = %q", u.Host)
	}
	if u.Path != "/mydb" {
		t.Errorf("Path = %q, want /mydb", u.Path)
	}
}

func TestNormalizeConnString_URL(t *testing.T) {
	cs, err := normalizeConnString("postgres://user:pass@localhost:5432/mydb")
	if err != nil {
		t.Fatalf("normalizeConnString() error = %v", err)
	}

	u, _ := url.Parse(cs)
	if u.User.Username() != "user" {
		t.Errorf("User = %q", u.User.Username())
	}
	pass, _ := u.User.Password()
	if pass != "pass" {
		t.Errorf("Password = %q", pass)
	}
	if u.Query().Get("sslmode") != "disable" {
		t.Error("sslmode should be added")
	}
}

func TestNormalizeConnString_WithoutPrefix(t *testing.T) {
	cs, err := normalizeConnString("user:pass@localhost:5432/db")
	if err != nil {
		t.Fatalf("normalizeConnString() error = %v", err)
	}
	if !strings.HasPrefix(cs, "postgres://") {
		t.Errorf("Result should start with postgres://, got %q", cs)
	}
}

func TestNormalizeConnString_WithSSL(t *testing.T) {
	cs, err := normalizeConnString("postgres://user:pass@localhost/db?sslmode=require")
	if err != nil {
		t.Fatalf("normalizeConnString() error = %v", err)
	}

	u, _ := url.Parse(cs)
	if u.Query().Get("sslmode") != "require" {
		t.Errorf("sslmode = %q, want require", u.Query().Get("sslmode"))
	}
}

func TestIndexDefRe(t *testing.T) {
	tests := []struct {
		def    string
		unique bool
		cols   int
	}{
		{`CREATE UNIQUE INDEX users_pkey ON public.users USING btree (id)`, true, 1},
		{`CREATE UNIQUE INDEX idx_email ON public.users USING btree (email)`, true, 1},
		{`CREATE INDEX idx_name_status ON public.users USING btree (name, status)`, false, 2},
		{`CREATE INDEX "custom_idx" ON public."table" USING gin (data)`, false, 1},
	}

	for _, tt := range tests {
		m := indexDefRe.FindStringSubmatch(tt.def)
		if m == nil {
			t.Errorf("regex didn't match: %s", tt.def)
			continue
		}
		unique := strings.TrimSpace(m[1]) != ""
		if unique != tt.unique {
			t.Errorf("unique = %v, want %v for %s", unique, tt.unique, tt.def)
		}
		cols := strings.Split(m[4], ",")
		if len(cols) != tt.cols {
			t.Errorf("columns = %d, want %d for %s", len(cols), tt.cols, tt.def)
		}
	}
}
