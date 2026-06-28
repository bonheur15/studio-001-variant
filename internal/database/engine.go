package database

import "github.com/bonheur/db-studio/internal/model"

type Engine interface {
	Name() string
	Connect(config model.ConnectionConfig) error
	Close() error
	Ping() error

	GetDatabases() ([]string, error)
	GetTables(database string) ([]model.TableInfo, error)
	GetColumns(database, table string) ([]model.ColumnInfo, error)
	GetIndexes(database, table string) ([]model.IndexInfo, error)

	ExecuteQuery(query string) (*model.QueryResult, error)
	ExecuteUpdate(query string) (int64, error)
	GetServerVersion() (string, error)
}
