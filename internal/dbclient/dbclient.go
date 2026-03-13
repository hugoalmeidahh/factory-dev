package dbclient

import (
	"database/sql"
	"fmt"
	"strings"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"

	"github.com/seuusuario/factorydev/internal/storage"
)

// Open abre uma conexão com o banco usando os dados de DBConnection.
func Open(c storage.DBConnection) (*sql.DB, error) {
	dsn, err := buildDSN(c)
	if err != nil {
		return nil, err
	}
	driver := c.Driver
	if driver == "postgres" {
		driver = "postgres"
	}
	db, err := sql.Open(driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("erro ao abrir conexão: %w", err)
	}
	if err := db.Ping(); err != nil {
		db.Close()
		return nil, fmt.Errorf("erro ao conectar: %w", err)
	}
	return db, nil
}

func buildDSN(c storage.DBConnection) (string, error) {
	switch c.Driver {
	case "sqlite":
		return c.Database, nil
	case "postgres":
		ssl := c.SSLMode
		if ssl == "" {
			ssl = "disable"
		}
		port := c.Port
		if port == 0 {
			port = 5432
		}
		return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
			c.Host, port, c.User, c.Password, c.Database, ssl), nil
	case "mysql":
		port := c.Port
		if port == 0 {
			port = 3306
		}
		// user:password@tcp(host:port)/dbname
		return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?parseTime=true",
			c.User, c.Password, c.Host, port, c.Database), nil
	default:
		return "", fmt.Errorf("driver não suportado: %s", c.Driver)
	}
}

// TableInfo contém nome e tipo (table/view).
type TableInfo struct {
	Name string
	Type string // "table" ou "view"
}

// ListTables retorna as tabelas do banco.
func ListTables(db *sql.DB, driver string) ([]TableInfo, error) {
	var query string
	switch driver {
	case "sqlite":
		query = `SELECT name, type FROM sqlite_master WHERE type IN ('table','view') AND name NOT LIKE 'sqlite_%' ORDER BY name`
	case "postgres":
		query = `SELECT table_name, table_type FROM information_schema.tables WHERE table_schema = 'public' ORDER BY table_name`
	case "mysql":
		query = `SELECT table_name, table_type FROM information_schema.tables WHERE table_schema = DATABASE() ORDER BY table_name`
	default:
		return nil, fmt.Errorf("driver não suportado: %s", driver)
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []TableInfo
	for rows.Next() {
		var t TableInfo
		if err := rows.Scan(&t.Name, &t.Type); err != nil {
			return nil, err
		}
		t.Type = strings.ToLower(t.Type)
		if t.Type == "base table" {
			t.Type = "table"
		}
		tables = append(tables, t)
	}
	return tables, rows.Err()
}

// ColumnInfo descreve uma coluna de uma tabela.
type ColumnInfo struct {
	Name     string
	Type     string
	Nullable string
	Key      string
	Default  string
}

// DescribeTable retorna as colunas de uma tabela.
func DescribeTable(db *sql.DB, driver, table string) ([]ColumnInfo, error) {
	var query string
	switch driver {
	case "sqlite":
		query = fmt.Sprintf("PRAGMA table_info(%s)", quoteIdent(table))
	case "postgres":
		query = fmt.Sprintf(`SELECT column_name, data_type, is_nullable, COALESCE(column_default,'')
			FROM information_schema.columns WHERE table_schema='public' AND table_name='%s' ORDER BY ordinal_position`, table)
	case "mysql":
		query = fmt.Sprintf(`SELECT column_name, column_type, is_nullable, COALESCE(column_key,''), COALESCE(column_default,'')
			FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='%s' ORDER BY ordinal_position`, table)
	default:
		return nil, fmt.Errorf("driver não suportado: %s", driver)
	}

	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var cols []ColumnInfo
	switch driver {
	case "sqlite":
		for rows.Next() {
			var cid int
			var c ColumnInfo
			var notNull int
			var dflt sql.NullString
			var pk int
			if err := rows.Scan(&cid, &c.Name, &c.Type, &notNull, &dflt, &pk); err != nil {
				return nil, err
			}
			if notNull == 1 {
				c.Nullable = "NO"
			} else {
				c.Nullable = "YES"
			}
			if pk > 0 {
				c.Key = "PK"
			}
			if dflt.Valid {
				c.Default = dflt.String
			}
			cols = append(cols, c)
		}
	case "postgres":
		for rows.Next() {
			var c ColumnInfo
			if err := rows.Scan(&c.Name, &c.Type, &c.Nullable, &c.Default); err != nil {
				return nil, err
			}
			cols = append(cols, c)
		}
	case "mysql":
		for rows.Next() {
			var c ColumnInfo
			if err := rows.Scan(&c.Name, &c.Type, &c.Nullable, &c.Key, &c.Default); err != nil {
				return nil, err
			}
			cols = append(cols, c)
		}
	}
	return cols, rows.Err()
}

// QueryResult contém o resultado de uma query.
type QueryResult struct {
	Columns  []string
	Rows     [][]string
	RowCount int
	Error    string
}

// RunQuery executa uma query e retorna os resultados (máx 500 rows).
func RunQuery(db *sql.DB, query string) QueryResult {
	rows, err := db.Query(query)
	if err != nil {
		return QueryResult{Error: err.Error()}
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return QueryResult{Error: err.Error()}
	}

	var result [][]string
	values := make([]sql.RawBytes, len(columns))
	scanArgs := make([]any, len(columns))
	for i := range values {
		scanArgs[i] = &values[i]
	}

	for rows.Next() && len(result) < 500 {
		if err := rows.Scan(scanArgs...); err != nil {
			return QueryResult{Columns: columns, Error: err.Error()}
		}
		row := make([]string, len(columns))
		for i, v := range values {
			if v == nil {
				row[i] = "NULL"
			} else {
				row[i] = string(v)
			}
		}
		result = append(result, row)
	}

	return QueryResult{
		Columns:  columns,
		Rows:     result,
		RowCount: len(result),
	}
}

func quoteIdent(s string) string {
	return `"` + strings.ReplaceAll(s, `"`, `""`) + `"`
}
