# DB Studio

An open-source database viewer with server-driven UI, built with Go, HTMX, Tailwind CSS, and Go templating. Supports MySQL and PostgreSQL with per-tab session isolation and an extensible driver architecture.

## Architecture

```
┌────────────────────────────────────────────────────────────┐
│                    Browser (HTMX)                          │
│  HTML over the wire · Server-driven UI · Minimal JS        │
└────────────────────────┬───────────────────────────────────┘
                         │ GET/POST partial HTML
                         ▼
┌────────────────────────────────────────────────────────────┐
│                     Go Server (chi router)                  │
│                                                            │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────────┐ │
│  │   Handler     │  │   Template   │  │  Session Manager │ │
│  │  (routes)     │──│   Engine     │  │  (per-tab, TTL)  │ │
│  └──────┬───────┘  └──────────────┘  └──────────────────┘ │
│         │                                                   │
│  ┌──────▼────────────────────────────────────────────────┐ │
│  │               Database Engine (interface)              │ │
│  │  ┌─────────┐  ┌────────────┐  ┌──────────────────┐    │ │
│  │  │  MySQL   │  │ PostgreSQL │  │  MongoDB (future)  │   │ │
│  │  └─────────┘  └────────────┘  └──────────────────┘    │ │
│  └─────────────────────────────────────────────────────────┘ │
└────────────────────────────────────────────────────────────┘
```

### Core pattern

- **HTMX** — all UI is HTML fragments served over the wire. The browser never builds DOM; the server sends partial HTML that HTMX swaps into the page.
- **Ephemeral sessions** — each browser tab gets a unique session ID via `sessionStorage`. Connections are stored in-memory with a 30-minute TTL. Credentials are never persisted to disk.
- **Driver registry** — new database engines register themselves via `init()` functions. The `DatabaseEngine` interface enforces a consistent contract for all drivers.
- **Template engine** — Go `html/template` with layout/page/partial pattern. Base layout is cloned per page to avoid conflicting `{{define}}` blocks. Prod mode caches parsed templates; dev mode reads from disk on every request.

## Project structure

```
.
├── main.go                              # Entry point, embed FS, chi router
├── go.mod / go.sum
├── Makefile                             # dev, build, test, css, clean
├── package.json                         # Tailwind + PostCSS
├── postcss.config.js
├── tailwind.config.js
├── .air.toml                            # Hot reload config
├── .gitignore
│
├── bin/                                 # Build output
│   └── db-studio
│
├── internal/
│   ├── database/
│   │   ├── engine.go                    # DatabaseEngine interface
│   │   ├── registry.go                  # Factory registry (thread-safe)
│   │   ├── mysql/
│   │   │   ├── engine.go                # MySQL driver (sql.Open)
│   │   │   └── engine_test.go           # 27 tests (go-sqlmock)
│   │   └── postgres/
│   │       ├── engine.go                # PostgreSQL driver (pgx/v5)
│   │       └── engine_test.go           # 25 tests (go-sqlmock)
│   │
│   ├── handler/
│   │   ├── handler.go                   # Handler struct, routes, helpers
│   │   ├── connection.go                # POST /api/connect, /api/disconnect
│   │   ├── connection_test.go           # Connection handler tests
│   │   ├── schema.go                    # Table/column/index/data endpoints
│   │   └── schema_test.go               # Schema handler tests
│   │
│   ├── model/
│   │   ├── types.go                     # All data types
│   │   └── types_test.go               # Model unit tests
│   │
│   ├── session/
│   │   ├── manager.go                   # Per-tab session with TTL cleanup
│   │   └── manager_test.go             # Session manager tests
│   │
│   └── template/
│       ├── engine.go                    # Template engine (layout/page/partial)
│       └── engine_test.go              # Template engine tests
│
└── web/
    ├── static/
    │   ├── css/
    │   │   ├── input.css                # Tailwind source + custom CSS
    │   │   └── output.css               # Built CSS (gitignored)
    │   └── js/
    │       └── app.js                   # Session mgmt, sidebar toggle
    │
    └── templates/
        ├── base.html                    # Base layout (DOCTYPE, head, scripts)
        ├── pages/
        │   └── index.html               # Landing page (wraps connection_form)
        └── partials/
            ├── connection_form.html      # Connect form (fields/string toggle)
            ├── connection_error.html     # Error display
            ├── connected.html            # Connected view (header + sidebar + main)
            ├── database_list.html        # Schema list sidebar
            ├── tables_list.html          # Table cards with loading indicator
            ├── table_detail.html         # Table detail with tab navigation
            ├── columns_list.html         # Columns table
            ├── indexes_list.html         # Indexes table
            └── table_data.html           # Paginated data table
```

## API endpoints

| Method | Path | Description | Returns |
|--------|------|-------------|---------|
| `GET` | `/` | Landing page | Full HTML page |
| `POST` | `/api/connect` | Connect to database | `connected` partial or `connection_error` |
| `POST` | `/api/disconnect` | Disconnect | `connection_form` partial |
| `GET` | `/api/databases` | List schemas | `database_list` partial |
| `GET` | `/api/tables?db=X` | List tables in schema | `tables_list` partial |
| `GET` | `/api/table?db=X&table=Y` | Table detail with tabs | `table_detail` partial |
| `GET` | `/api/columns?db=X&table=Y` | List columns | `columns_list` partial |
| `GET` | `/api/indexes?db=X&table=Y` | List indexes | `indexes_list` partial |
| `GET` | `/api/data?db=X&table=Y&offset=0&limit=50` | Paginated data | `table_data` partial |

All endpoints (except `/`) require the `X-Session-Id` header set to the tab's session ID.

## Templates (Go html/template)

### Template functions

Available in all templates:

| Function | Signature | Description |
|----------|-----------|-------------|
| `add` | `(a, b int) int` | Addition |
| `sub` | `(a, b int) int` | Subtraction |
| `seq` | `(n int) []int` | Generate sequence 1..n |
| `hasPrefix` | `(s, prefix string) bool` | String prefix check |
| `trimSuffix` | `(s, suffix string) string` | String suffix trim |
| `mod` | `(a, b int) int` | Modulo |
| `even` | `(n int) bool` | Even number check |
| `isNil` | `(v interface{}) bool` | Nil check |

### Partial rendering

Partials are rendered via `RenderPartial(name, data)` which clones the common template set and executes the named template. All partials share the same template pool — any partial can `{{template "other_partial" .}}` to embed another.

## Models (data types)

### ConnectionConfig
```go
type ConnectionConfig struct {
    Driver     string   // "mysql", "postgres", "mongodb"
    Host       string
    Port       int
    User       string
    Password   string
    Database   string
    ConnString string   // alternative to individual fields
}
```

### TableInfo
```go
type TableInfo struct {
    Name    string   // table name
    Type    string   // "TABLE" or "VIEW"
    Schema  string   // schema/database name
    Rows    int64    // estimated row count
    Comment string   // table comment
}
```

### ColumnInfo
```go
type ColumnInfo struct {
    Name       string
    Type       string   // data type (e.g. "integer", "character varying(255)")
    Nullable   bool
    Default    string
    PrimaryKey bool
    Comment    string
}
```

### IndexInfo
```go
type IndexInfo struct {
    Name    string
    Columns []string
    Unique  bool
    Primary bool
    Type    string   // index type (btree, hash, etc.)
}
```

### QueryResult
```go
type QueryResult struct {
    Columns []string
    Rows    [][]any
    Total   int64
    Time    string
    Error   string   // non-empty on query error
}
```

## Session management

Sessions are per-tab, ephemeral, in-memory only.

**Lifecycle:**
1. Page load → `meta[name="session-id"]` is set server-side
2. JS copies it to `sessionStorage`
3. Every HTMX request includes `X-Session-Id` header
4. Server stores `database.Engine` in the session's `Connections` map

**Cleanup:** A goroutine runs every 5 minutes, deleting sessions inactive for >30 minutes. Closed connections are properly cleaned up.

## Database interface

```go
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
```

New database engines register themselves in `init()`:

```go
func init() {
    database.Register("myengine", func(cfg model.ConnectionConfig) (database.Engine, error) {
        e := &engine{cfg: cfg}
        if err := e.Connect(cfg); err != nil {
            return nil, err
        }
        return e, nil
    })
}
```

### MySQL driver

- Uses `github.com/go-sql-driver/mysql` (go-sql-driver)
- DSN building from fields or URL/raw DSN parsing
- Connection pool: max 5 open, 2 idle, 5min lifetime
- `information_schema` queries for schema browsing
- `information_schema.STATISTICS` with column grouping for indexes
- `COLUMN_TYPE` for detailed type display
- 10s query timeout for metadata, 30s for arbitrary queries

### PostgreSQL driver

- Uses `github.com/jackc/pgx/v5` through `pgx/v5/stdlib` (`database/sql` compatibility)
- URL-based connection string normalization
- Connection pool: max 5 open, 2 idle, 5min lifetime
- `information_schema` + `pg_catalog` queries for schema browsing
- Regex-based index definition parsing from `pg_indexes`
- `obj_description()` and `col_description()` for comments
- 10s query timeout for metadata, 30s for arbitrary queries
- `sslmode=disable` by default

## CSS components

All custom components are defined in `web/static/css/input.css` using Tailwind's `@apply`:

| Component | Classes | Usage |
|-----------|---------|-------|
| Card | `.card`, `.card-float` | Containers with rounded corners, shadow, border |
| Buttons | `.btn`, `.btn-primary`, `.btn-secondary`, `.btn-ghost` | Interactive elements |
| Inputs | `.input`, `.select` | Form controls |
| Labels | `.label` | Form labels |
| Badges | `.badge`, `.badge-blue`, `.badge-green`, `.badge-gray` | Status indicators |
| Table | `.table-header`, `.table-cell` | Data tables |
| Tabs | `.tab`, `.tab-active`, `.tab-inactive` | Tab navigation |
| Skeleton | `.skeleton` | Loading placeholder |

Custom animations and utilities:
- `.fade-in` — 200ms opacity + vertical slide
- `.slide-in-right` — 200ms horizontal slide
- `.spinner` — 0.6s rotating border spinner
- `.htmx-indicator` — opacity 0→1 when parent has `.htmx-request`
- `.sidebar-panel` — mobile-first fixed sidebar with slide transition
- `.sidebar-overlay` — backdrop for mobile sidebar

## JavaScript

Only ~50 lines of vanilla JS. No framework.

**`app.js` responsibilities:**
1. Copy `session-id` from meta tag to `sessionStorage`
2. Add `X-Session-Id` header to all HTMX requests
3. Toggle sidebar `open` class for mobile responsive
4. Close sidebar on HTMX content swap (mobile)
5. Auto-update port field when driver selection changes

## Getting started

### Prerequisites

- Go 1.25+
- Node.js 18+ (for Tailwind)
- A MySQL or PostgreSQL database

### Development

```bash
# Terminal 1: CSS watcher (auto-rebuilds on template changes)
npm run dev:css

# Terminal 2: Go server with hot reload
make dev

# Or without hot reload
DEV=true go run .
```

### Production build

```bash
make build
./bin/db-studio
```

Starts on `http://localhost:8080`. Set `DEV=true` to read templates from disk (for development without rebuilding).

### Docker test database (PostgreSQL)

```bash
docker run -d \
  --name test-pg \
  -e POSTGRES_PASSWORD=postgres \
  -e POSTGRES_DB=hubfly_dashboard_test \
  -p 55432:5432 \
  postgres:17-alpine
```

### Testing

```bash
make test          # all tests
go test ./... -v   # verbose
```

Coverage: 54+ tests across all packages. Database drivers use go-sqlmock for unit tests. Connection tests skip without a real database (detected via `DB_MYSQL_DSN`/`DB_POSTGRES_DSN` env vars).

## Theme and design decisions

- **Light mode only** (for now). Clean, minimal UI.
- **No emojis**. Icons from Heroicons via inline SVGs.
- **No purple**. Blue primary, emerald green for success, gray for neutral, red for errors.
- **No gradients**. Flat colors, subtle shadows.
- **Grid background** on the landing page for visual interest.
- **Floaty UI** — cards with rounded corners, shadows, and hover effects.
- **`max-w-[200px] truncate`** on data cells to handle long values without breaking layout.

## What has been implemented (Phases 1-5)

### Phase 1: Scaffolding
- Go module setup, chi router, embedded FS
- Tailwind + PostCSS with JIT compilation
- Hot reload with air
- Directory structure and Makefile

### Phase 2: Base UI
- Landing page with grid background
- Connection form (fields/string toggle)
- Error display partial
- Base layout with HTMX + Tailwind

### Phase 3: MySQL driver
- Full `DatabaseEngine` implementation
- DSN building (fields, URL, raw DSN)
- Schema browsing via `information_schema`
- Connection pool configuration
- 27 unit/integration tests with go-sqlmock

### Phase 4: PostgreSQL driver
- Full `DatabaseEngine` implementation via pgx/v5
- URL-based connection string normalization
- Schema browsing via `information_schema` + `pg_catalog`
- Index definition regex parsing
- 25 unit/integration tests with go-sqlmock

### Phase 5: Connection handler + UI
- Connect/disconnect with HTMX partial responses
- Connected view: header (driver badge, label, version), sidebar (schemas), main area
- Schema selection loads table list
- Per-tab session isolation

### Phase 5b: Table detail + Polish
- **Table detail view** — click any table card to see full detail with three tabs:
  - **Columns** — name, type, nullable, default, PK badge, comment
  - **Indexes** — name, columns, unique flag, type (btree etc.)
  - **Data** — paginated row data with Previous/Load More buttons, NULL display
- **Responsive sidebar** — hamburger menu on mobile, overlay with slide-in animation, closes on schema click
- **Loading states** — spinner on connect button, loading indicator when clicking a table
- **Animations** — `fade-in` on all content swaps, smooth transitions
- **Custom scrollbar** — thin, themed to match the design
- **Utility templates** — `even`, `mod`, `isNil` functions for template logic
- **Refined UI** — better hover states, card shadows, tab styling, responsive breakpoints

## What is planned (Phases 6-10)

### Phase 6: Query editor
- SQL textarea with keyboard shortcut (Cmd+Enter to execute)
- HTMX execution, paginated results table, EXPLAIN support
- Query history per session

### Phase 7: MongoDB driver
- Collection browsing, document view, JSON query support
- Aggregation pipeline builder

### Phase 8: Export
- CSV and JSON export for query results and table data
- Streaming export for large datasets

### Phase 9: Dark mode
- CSS variables for theme switching
- Toggle in header, persisted to localStorage
- Dark variant for all components

### Phase 10: Polish & advanced features
- Inline cell editing, read-only mode enforcement
- Query timeout configuration in UI
- Table search/filter, column sorting
- Keyboard shortcuts, command palette
- Bulk row deletion, schema diff viewer

## Known limitations

- PostgreSQL row counts are hardcoded to 0 (currently no separate count query)
- No authentication — session-based auth planned
- No HTTPS support (reverse proxy expected)
- Connection pooling is hardcoded (5 max open, 2 max idle)
- MongoDB driver exists in the driver selector but is not implemented
- Template functions use `%v` formatting for arbitrary data — dates may not format consistently across drivers
- Identifier quoting uses double quotes (works for PostgreSQL; MySQL requires ANSI_QUOTES mode)

## Improvements backlog

- [ ] Real PostgreSQL row counts via `pg_stat_user_tables` or `COUNT(*)`
- [ ] Per-driver identifier quoting (backticks for MySQL, double quotes for PostgreSQL)
- [ ] URL encoding for table/schema names with special characters
- [ ] Column sorting in data table
- [ ] Table search filter in sidebar
- [ ] Query timeout configurable from UI
- [ ] Session-based authentication
- [ ] Connection pool configuration in UI
- [ ] Server-side caching for metadata queries
- [ ] WebSocket-based real-time updates
- [ ] Save/restore tabs across browser sessions
- [ ] LLM integration for natural language queries
