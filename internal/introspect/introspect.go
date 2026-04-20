// Package introspect loads live database schemas into the neutral schema.Model.
//
// Drivers live in their own files (postgres.go). Each driver is responsible for
// producing a schema.Model that is fully self-contained — no lazy callbacks
// across the boundary, because the caller may pickle the model to YAML without
// holding the DB connection open.
package introspect

import (
	"context"
	"fmt"
	"strings"

	"github.com/ivannikolaev/seed-cli/cli/internal/schema"
)

type Options struct {
	DSN        string
	SchemaFile string // path to SQL DDL file; mutually exclusive with DSN
	Schemas    []string
	SchemaAll  bool
	// Only keeps only the listed tables. Accepts "table" or "schema.table".
	Only    []string
	// Exclude removes the listed tables. Accepts "table" or "schema.table".
	Exclude []string
}

func Run(ctx context.Context, dialect string, opts Options) (*schema.Model, error) {
	var (
		m   *schema.Model
		err error
	)
	if opts.SchemaFile != "" {
		defaultSchema := "public"
		if len(opts.Schemas) > 0 {
			defaultSchema = opts.Schemas[0]
		}
		m, err = (&DDL{}).Introspect(opts.SchemaFile, defaultSchema)
	} else {
		switch dialect {
		case "", "postgres", "postgresql", "pg":
			m, err = (&Postgres{}).Introspect(ctx, opts)
		default:
			return nil, fmt.Errorf("unsupported dialect %q (only postgres in MVP)", dialect)
		}
	}
	if err != nil {
		return nil, err
	}
	filterTables(m, opts.Only, opts.Exclude)
	return m, nil
}

// filterTables applies --only and --exclude to the model in-place.
// Names are matched as "table" (any schema) or "schema.table" (exact).
func filterTables(m *schema.Model, only, exclude []string) {
	if len(only) == 0 && len(exclude) == 0 {
		return
	}
	onlySet := parseTableSet(only)
	excludeSet := parseTableSet(exclude)

	var kept []schema.Table
	for _, t := range m.Tables {
		fq := t.Schema + "." + t.Name
		short := t.Name
		if len(onlySet) > 0 {
			if !onlySet[fq] && !onlySet[short] {
				continue
			}
		}
		if excludeSet[fq] || excludeSet[short] {
			continue
		}
		kept = append(kept, t)
	}
	m.Tables = kept
}

func parseTableSet(names []string) map[string]bool {
	s := make(map[string]bool, len(names))
	for _, n := range names {
		s[strings.TrimSpace(n)] = true
	}
	return s
}

// InScope reports whether a (schema,name) table passes the same --only/--exclude
// filter `filterTables` applies to the introspected model. Exposed so `sync`
// can scope its merge symmetrically and avoid flagging out-of-scope tables as
// removed — merge would otherwise interpret an excluded table as deleted.
func InScope(schemaName, tableName string, only, exclude []string) bool {
	if len(only) == 0 && len(exclude) == 0 {
		return true
	}
	onlySet := parseTableSet(only)
	excludeSet := parseTableSet(exclude)
	fq := schemaName + "." + tableName
	if len(onlySet) > 0 {
		if !onlySet[fq] && !onlySet[tableName] {
			return false
		}
	}
	if excludeSet[fq] || excludeSet[tableName] {
		return false
	}
	return true
}
