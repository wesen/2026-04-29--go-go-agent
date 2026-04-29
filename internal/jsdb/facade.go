package jsdb

import (
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/go-go-golems/geppetto/pkg/inference/tools/scopedjs"
	gojengine "github.com/go-go-golems/go-go-goja/engine"
)

// Facade is a small JavaScript-facing SQLite API.
type Facade struct {
	Name     string
	DB       *sql.DB
	Readonly bool
	Tables   []string
}

type ExecResult struct {
	RowsAffected int64 `json:"rowsAffected"`
	LastInsertID int64 `json:"lastInsertId"`
}

type SchemaSummary struct {
	Name     string   `json:"name"`
	Readonly bool     `json:"readonly"`
	Tables   []string `json:"tables"`
}

func (f *Facade) Query(query string, args ...any) ([]map[string]any, error) {
	if f == nil || f.DB == nil {
		return nil, fmt.Errorf("%s is not configured", f.name())
	}
	if f.Readonly && !isReadQuery(query) {
		return nil, fmt.Errorf("%s only allows SELECT/WITH queries", f.name())
	}
	rows, err := f.DB.QueryContext(context.Background(), query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}
	ret := []map[string]any{}
	for rows.Next() {
		vals := make([]any, len(cols))
		scan := make([]any, len(cols))
		for i := range vals {
			scan[i] = &vals[i]
		}
		if err := rows.Scan(scan...); err != nil {
			return nil, err
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = normalizeSQLiteValue(vals[i])
		}
		ret = append(ret, row)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return ret, nil
}

func (f *Facade) Exec(query string, args ...any) (ExecResult, error) {
	if f == nil || f.DB == nil {
		return ExecResult{}, fmt.Errorf("%s is not configured", f.name())
	}
	if f.Readonly {
		return ExecResult{}, fmt.Errorf("%s is read-only", f.name())
	}
	res, err := f.DB.ExecContext(context.Background(), query, args...)
	if err != nil {
		return ExecResult{}, err
	}
	rowsAffected, _ := res.RowsAffected()
	lastID, _ := res.LastInsertId()
	return ExecResult{RowsAffected: rowsAffected, LastInsertID: lastID}, nil
}

func (f *Facade) Schema() SchemaSummary {
	if f == nil {
		return SchemaSummary{}
	}
	return SchemaSummary{
		Name:     f.name(),
		Readonly: f.Readonly,
		Tables:   append([]string(nil), f.Tables...),
	}
}

func (f *Facade) BindGlobal(globalName string, doc scopedjs.GlobalDoc) (string, scopedjs.GlobalBinding, scopedjs.GlobalDoc) {
	if strings.TrimSpace(doc.Type) == "" {
		doc.Type = "object"
	}
	if strings.TrimSpace(doc.Description) == "" {
		doc.Description = fmt.Sprintf("SQLite facade %s", globalName)
	}
	return globalName, func(ctx *gojengine.RuntimeContext) error {
		obj := ctx.VM.NewObject()
		if err := obj.Set("query", f.Query); err != nil {
			return err
		}
		if err := obj.Set("exec", f.Exec); err != nil {
			return err
		}
		if err := obj.Set("schema", f.Schema); err != nil {
			return err
		}
		return ctx.VM.Set(globalName, obj)
	}, doc
}

func (f *Facade) name() string {
	if f == nil || strings.TrimSpace(f.Name) == "" {
		return "db"
	}
	return f.Name
}

func normalizeSQLiteValue(v any) any {
	switch x := v.(type) {
	case []byte:
		return string(x)
	default:
		return x
	}
}

func isReadQuery(query string) bool {
	q := strings.TrimSpace(strings.TrimPrefix(query, "\ufeff"))
	if q == "" {
		return false
	}
	q = strings.ToLower(q)
	return strings.HasPrefix(q, "select") || strings.HasPrefix(q, "with")
}
