package postgres

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"

	"github.com/bonheur/db-studio/internal/database"
	"github.com/bonheur/db-studio/internal/model"
)

func init() {
	database.Register("postgres", func(cfg model.ConnectionConfig) (database.Engine, error) {
		e := &engine{cfg: cfg}
		if err := e.Connect(cfg); err != nil {
			return nil, err
		}
		return e, nil
	})
}

type engine struct {
	db  *sql.DB
	cfg model.ConnectionConfig
}

func (e *engine) Name() string { return "postgres" }

func (e *engine) Connect(cfg model.ConnectionConfig) error {
	connStr, err := buildConnStr(cfg)
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	db, err := sql.Open("pgx", connStr)
	if err != nil {
		return fmt.Errorf("failed to open connection: %w", err)
	}

	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)
	db.SetConnMaxIdleTime(1 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return fmt.Errorf("failed to connect: %w", err)
	}

	e.db = db
	return nil
}

func buildConnStr(cfg model.ConnectionConfig) (string, error) {
	if cfg.ConnString != "" {
		return normalizeConnString(cfg.ConnString)
	}

	host := cfg.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := cfg.Port
	if port == 0 {
		port = 5432
	}

	params := url.Values{}
	params.Set("sslmode", "disable")

	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(cfg.User, cfg.Password),
		Host:     fmt.Sprintf("%s:%d", host, port),
		Path:     cfg.Database,
		RawQuery: params.Encode(),
	}

	return u.String(), nil
}

func normalizeConnString(s string) (string, error) {
	if !strings.HasPrefix(s, "postgres://") && !strings.HasPrefix(s, "postgresql://") {
		s = "postgres://" + s
	}

	u, err := url.Parse(s)
	if err != nil {
		return "", fmt.Errorf("invalid connection string: %w", err)
	}

	q := u.Query()
	if q.Get("sslmode") == "" {
		q.Set("sslmode", "disable")
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}

func (e *engine) Close() error {
	if e.db != nil {
		return e.db.Close()
	}
	return nil
}

func (e *engine) Ping() error {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return e.db.PingContext(ctx)
}

func (e *engine) GetDatabases() ([]string, error) {
	query := `SELECT schema_name FROM information_schema.schemata
		WHERE schema_name NOT IN ('pg_catalog', 'information_schema', 'pg_toast')
		ORDER BY schema_name`

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := e.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list schemas: %w", err)
	}
	defer rows.Close()

	var databases []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			return nil, err
		}
		databases = append(databases, name)
	}
	return databases, rows.Err()
}

func (e *engine) GetTables(schema string) ([]model.TableInfo, error) {
	query := `SELECT table_name, table_type, 0,
		COALESCE(obj_description(
			(quote_ident(table_schema) || '.' || quote_ident(table_name))::regclass,
			'pg_class'
		), '')
		FROM information_schema.tables
		WHERE table_schema = $1
		ORDER BY table_name`

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := e.db.QueryContext(ctx, query, schema)
	if err != nil {
		return nil, fmt.Errorf("failed to list tables: %w", err)
	}
	defer rows.Close()

	var tables []model.TableInfo
	for rows.Next() {
		var t model.TableInfo
		var tableType string
		if err := rows.Scan(&t.Name, &tableType, &t.Rows, &t.Comment); err != nil {
			return nil, err
		}
		switch tableType {
		case "BASE TABLE":
			t.Type = "TABLE"
		case "VIEW":
			t.Type = "VIEW"
		default:
			t.Type = tableType
		}
		t.Schema = schema
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func (e *engine) GetColumns(schema, table string) ([]model.ColumnInfo, error) {
	query := `SELECT
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
		ORDER BY c.ordinal_position`

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := e.db.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	defer rows.Close()

	var columns []model.ColumnInfo
	for rows.Next() {
		var c model.ColumnInfo
		var maxLen, prec, scale int
		var nullBool, pkBool int
		if err := rows.Scan(&c.Name, &c.Type, &maxLen, &prec, &scale,
			&nullBool, &pkBool, &c.Default, &c.Comment, &c.Type); err != nil {
			return nil, err
		}
		c.Nullable = nullBool == 1
		c.PrimaryKey = pkBool == 1
		columns = append(columns, c)
	}
	return columns, rows.Err()
}

var indexDefRe = regexp.MustCompile(`CREATE\s+(UNIQUE\s+)?INDEX\s+(?:"?([^"\s]+)"?\s+)?ON\s+(?:\S+\s+)?USING\s+(\w+)\s+\(([^)]+)\)`)

func (e *engine) GetIndexes(schema, table string) ([]model.IndexInfo, error) {
	query := `SELECT indexname, indexdef FROM pg_indexes
		WHERE schemaname = $1 AND tablename = $2
		ORDER BY indexname`

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := e.db.QueryContext(ctx, query, schema, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %w", err)
	}
	defer rows.Close()

	var indexes []model.IndexInfo
	for rows.Next() {
		var idxName, idxDef string
		if err := rows.Scan(&idxName, &idxDef); err != nil {
			return nil, err
		}

		ii := model.IndexInfo{
			Name:    idxName,
			Primary: idxName == "PRIMARY" || strings.HasPrefix(idxName, "PK_"),
			Columns: []string{},
		}

		matches := indexDefRe.FindStringSubmatch(idxDef)
		if len(matches) >= 5 {
			ii.Unique = strings.TrimSpace(matches[1]) != ""
			if matches[2] != "" {
				ii.Name = matches[2]
			}
			ii.Type = matches[3]
			cols := strings.Split(matches[4], ",")
			for _, col := range cols {
				ii.Columns = append(ii.Columns, strings.TrimSpace(col))
			}
		}

		indexes = append(indexes, ii)
	}
	return indexes, rows.Err()
}

func (e *engine) ExecuteQuery(query string) (*model.QueryResult, error) {
	start := time.Now()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	rows, err := e.db.QueryContext(ctx, query)
	if err != nil {
		return &model.QueryResult{
			Error: err.Error(),
			Time:  time.Since(start).String(),
		}, nil
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	result := &model.QueryResult{
		Columns: columns,
		Time:    time.Since(start).String(),
	}

	for rows.Next() {
		values := make([]any, len(columns))
		valuePtrs := make([]any, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		row := make([]any, len(columns))
		for i, v := range values {
			row[i] = formatValue(v)
		}
		result.Rows = append(result.Rows, row)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	result.Total = int64(len(result.Rows))
	result.Time = time.Since(start).String()

	return result, nil
}

func formatValue(v any) any {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case time.Time:
		return val.Format("2006-01-02 15:04:05")
	case []byte:
		return string(val)
	default:
		return v
	}
}

func (e *engine) ExecuteUpdate(query string) (int64, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	result, err := e.db.ExecContext(ctx, query)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

func (e *engine) GetServerVersion() (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var version string
	err := e.db.QueryRowContext(ctx, "SELECT version()").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}
	return version, nil
}
