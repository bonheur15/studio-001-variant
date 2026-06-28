package handler

import (
	"fmt"
	"net/http"

	"github.com/bonheur/db-studio/internal/database"
	"github.com/bonheur/db-studio/internal/model"
)

func (h *Handler) getEngine(r *http.Request) database.Engine {
	sess := h.getSession(r)
	if sess == nil {
		return nil
	}
	v, ok := sess.GetConnection("default")
	if !ok {
		return nil
	}
	eng, ok := v.(database.Engine)
	if !ok {
		return nil
	}
	return eng
}

func (h *Handler) ListDatabases(w http.ResponseWriter, r *http.Request) {
	engine := h.getEngine(r)
	if engine == nil {
		http.Error(w, "not connected", http.StatusBadRequest)
		return
	}

	databases, err := engine.GetDatabases()
	if err != nil {
		h.renderError(w, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.RenderPartial(w, "database_list", map[string]interface{}{
		"Databases": databases,
	})
}

func (h *Handler) ListTables(w http.ResponseWriter, r *http.Request) {
	engine := h.getEngine(r)
	if engine == nil {
		http.Error(w, "not connected", http.StatusBadRequest)
		return
	}

	db := r.URL.Query().Get("db")
	if db == "" {
		h.renderError(w, "database parameter required")
		return
	}

	tables, err := engine.GetTables(db)
	if err != nil {
		h.renderError(w, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.RenderPartial(w, "tables_list", map[string]interface{}{
		"Database": db,
		"Tables":   tables,
	})
}

func (h *Handler) TableDetail(w http.ResponseWriter, r *http.Request) {
	engine := h.getEngine(r)
	if engine == nil {
		http.Error(w, "not connected", http.StatusBadRequest)
		return
	}

	db := r.URL.Query().Get("db")
	table := r.URL.Query().Get("table")
	if db == "" || table == "" {
		h.renderError(w, "database and table parameters required")
		return
	}

	var tableInfo model.TableInfo
	tables, err := engine.GetTables(db)
	if err == nil {
		for _, t := range tables {
			if t.Name == table {
				tableInfo = t
				break
			}
		}
	}
	if tableInfo.Name == "" {
		tableInfo = model.TableInfo{Name: table, Type: "TABLE"}
	}

	columns, _ := engine.GetColumns(db, table)
	indexes, _ := engine.GetIndexes(db, table)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.RenderPartial(w, "table_detail", map[string]interface{}{
		"Database": db,
		"Table":    tableInfo,
		"Columns":  columns,
		"Indexes":  indexes,
	})
}

func (h *Handler) ListColumns(w http.ResponseWriter, r *http.Request) {
	engine := h.getEngine(r)
	if engine == nil {
		http.Error(w, "not connected", http.StatusBadRequest)
		return
	}

	db := r.URL.Query().Get("db")
	table := r.URL.Query().Get("table")
	if db == "" || table == "" {
		h.renderError(w, "database and table parameters required")
		return
	}

	columns, err := engine.GetColumns(db, table)
	if err != nil {
		h.renderError(w, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.RenderPartial(w, "columns_list", map[string]interface{}{
		"Database": db,
		"Table":    table,
		"Columns":  columns,
	})
}

func (h *Handler) ListIndexes(w http.ResponseWriter, r *http.Request) {
	engine := h.getEngine(r)
	if engine == nil {
		http.Error(w, "not connected", http.StatusBadRequest)
		return
	}

	db := r.URL.Query().Get("db")
	table := r.URL.Query().Get("table")
	if db == "" || table == "" {
		h.renderError(w, "database and table parameters required")
		return
	}

	indexes, err := engine.GetIndexes(db, table)
	if err != nil {
		h.renderError(w, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.RenderPartial(w, "indexes_list", map[string]interface{}{
		"Database": db,
		"Table":    table,
		"Indexes":  indexes,
	})
}

func (h *Handler) TableData(w http.ResponseWriter, r *http.Request) {
	engine := h.getEngine(r)
	if engine == nil {
		http.Error(w, "not connected", http.StatusBadRequest)
		return
	}

	db := r.URL.Query().Get("db")
	table := r.URL.Query().Get("table")
	offset := parseInt(r.URL.Query().Get("offset"), 0)
	limit := parseInt(r.URL.Query().Get("limit"), 50)
	if limit < 1 || limit > 200 {
		limit = 50
	}
	if offset < 0 {
		offset = 0
	}

	if db == "" || table == "" {
		h.renderError(w, "database and table parameters required")
		return
	}

	actualLimit := limit + 1
	query := fmt.Sprintf("SELECT * FROM %s.%s LIMIT %d OFFSET %d",
		quoteIdentifier(db), quoteIdentifier(table), actualLimit, offset)

	result, err := engine.ExecuteQuery(query)
	if err != nil {
		h.renderError(w, err.Error())
		return
	}

	rows := result.Rows
	hasMore := len(rows) > limit
	if hasMore {
		rows = rows[:limit]
	}

	end := offset + len(rows)

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	h.tmpl.RenderPartial(w, "table_data", map[string]interface{}{
		"Database":   db,
		"Table":      table,
		"Columns":    result.Columns,
		"Rows":       rows,
		"Offset":     offset,
		"Limit":      limit,
		"End":        end,
		"HasMore":    hasMore,
		"HasPrev":    offset > 0,
		"NextOffset": offset + limit,
		"PrevOffset": offset - limit,
	})
}
