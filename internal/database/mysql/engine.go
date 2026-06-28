package mysql

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
	mysqlDriver "github.com/go-sql-driver/mysql"

	"github.com/bonheur/db-studio/internal/database"
	"github.com/bonheur/db-studio/internal/model"
)

func init() {
	database.Register("mysql", func(cfg model.ConnectionConfig) (database.Engine, error) {
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

func (e *engine) Name() string { return "mysql" }

func (e *engine) Connect(cfg model.ConnectionConfig) error {
	config, err := buildConfig(cfg)
	if err != nil {
		return fmt.Errorf("invalid configuration: %w", err)
	}

	dsn := config.FormatDSN()
	db, err := sql.Open("mysql", dsn)
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

func buildConfig(cfg model.ConnectionConfig) (*mysqlDriver.Config, error) {
	if cfg.ConnString != "" {
		return parseConnString(cfg.ConnString)
	}

	host := cfg.Host
	if host == "" {
		host = "127.0.0.1"
	}
	port := cfg.Port
	if port == 0 {
		port = 3306
	}

	config := mysqlDriver.NewConfig()
	config.User = cfg.User
	config.Passwd = cfg.Password
	config.Net = "tcp"
	config.Addr = fmt.Sprintf("%s:%d", host, port)
	config.DBName = cfg.Database
	config.ParseTime = true
	config.MultiStatements = false
	config.Loc = time.UTC
	config.Collation = "utf8mb4_general_ci"
	if config.Params == nil {
		config.Params = make(map[string]string)
	}
	config.Params["charset"] = "utf8mb4"

	return config, nil
}

func parseConnString(s string) (*mysqlDriver.Config, error) {
	if strings.HasPrefix(s, "mysql://") {
		u, err := url.Parse(s)
		if err != nil {
			return nil, fmt.Errorf("invalid connection URL: %w", err)
		}

		password, _ := u.User.Password()
		config := mysqlDriver.NewConfig()
		config.User = u.User.Username()
		config.Passwd = password
		config.Net = "tcp"
		config.Addr = u.Host
		config.DBName = strings.TrimPrefix(u.Path, "/")
		config.ParseTime = true
		config.MultiStatements = false
		config.Loc = time.UTC
		config.Collation = "utf8mb4_general_ci"
		if config.Params == nil {
			config.Params = make(map[string]string)
		}
		config.Params["charset"] = "utf8mb4"

		for key, vals := range u.Query() {
			if len(vals) > 0 {
				config.Params[key] = vals[0]
			}
		}

		return config, nil
	}

	config, err := mysqlDriver.ParseDSN(s)
	if err != nil {
		return nil, fmt.Errorf("invalid DSN: %w", err)
	}
	if config.Params == nil {
		config.Params = make(map[string]string)
	}
	if _, ok := config.Params["charset"]; !ok {
		config.Params["charset"] = "utf8mb4"
	}
	return config, nil
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
	query := `SELECT SCHEMA_NAME FROM information_schema.SCHEMATA
		WHERE SCHEMA_NAME NOT IN ('mysql', 'information_schema', 'performance_schema', 'sys')
		ORDER BY SCHEMA_NAME`

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := e.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("failed to list databases: %w", err)
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

func (e *engine) GetTables(database string) ([]model.TableInfo, error) {
	query := `SELECT TABLE_NAME, TABLE_TYPE, IFNULL(TABLE_ROWS, 0), IFNULL(TABLE_COMMENT, '')
		FROM information_schema.TABLES
		WHERE TABLE_SCHEMA = ?
		ORDER BY TABLE_NAME`

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := e.db.QueryContext(ctx, query, database)
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
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

func (e *engine) GetColumns(database, table string) ([]model.ColumnInfo, error) {
	query := `SELECT COLUMN_NAME, DATA_TYPE,
		IFNULL(CHARACTER_MAXIMUM_LENGTH, 0), IFNULL(NUMERIC_PRECISION, 0), IFNULL(NUMERIC_SCALE, 0),
		IF(IS_NULLABLE = 'YES', 1, 0),
		IF(COLUMN_KEY = 'PRI', 1, 0),
		IFNULL(COLUMN_DEFAULT, ''),
		IFNULL(COLUMN_COMMENT, ''),
		COLUMN_TYPE
		FROM information_schema.COLUMNS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY ORDINAL_POSITION`

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := e.db.QueryContext(ctx, query, database, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get columns: %w", err)
	}
	defer rows.Close()

	var columns []model.ColumnInfo
	for rows.Next() {
		var c model.ColumnInfo
		var maxLen, prec, scale int64
		var nullBool, pkBool int
		var colType string
		if err := rows.Scan(&c.Name, &colType, &maxLen, &prec, &scale,
			&nullBool, &pkBool, &c.Default, &c.Comment, &c.Type); err != nil {
			return nil, err
		}
		c.Nullable = nullBool == 1
		c.PrimaryKey = pkBool == 1
		columns = append(columns, c)
	}
	return columns, rows.Err()
}

func (e *engine) GetIndexes(database, table string) ([]model.IndexInfo, error) {
	query := `SELECT INDEX_NAME, COLUMN_NAME, NON_UNIQUE, INDEX_TYPE
		FROM information_schema.STATISTICS
		WHERE TABLE_SCHEMA = ? AND TABLE_NAME = ?
		ORDER BY INDEX_NAME, SEQ_IN_INDEX`

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	rows, err := e.db.QueryContext(ctx, query, database, table)
	if err != nil {
		return nil, fmt.Errorf("failed to get indexes: %w", err)
	}
	defer rows.Close()

	indexMap := make(map[string]*model.IndexInfo)
	var indexOrder []string

	for rows.Next() {
		var indexName, columnName, indexType string
		var nonUnique int
		if err := rows.Scan(&indexName, &columnName, &nonUnique, &indexType); err != nil {
			return nil, err
		}

		idx, exists := indexMap[indexName]
		if !exists {
			primary := indexName == "PRIMARY"
			idx = &model.IndexInfo{
				Name:    indexName,
				Columns: []string{},
				Unique:  nonUnique == 0,
				Primary: primary,
				Type:    indexType,
			}
			indexMap[indexName] = idx
			indexOrder = append(indexOrder, indexName)
		}
		idx.Columns = append(idx.Columns, columnName)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	indexes := make([]model.IndexInfo, len(indexOrder))
	for i, name := range indexOrder {
		indexes[i] = *indexMap[name]
	}
	return indexes, nil
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
	err := e.db.QueryRowContext(ctx, "SELECT VERSION()").Scan(&version)
	if err != nil {
		return "", fmt.Errorf("failed to get version: %w", err)
	}
	return version, nil
}
