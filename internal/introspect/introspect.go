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

	"github.com/ivannikolaev/seed-cli/cli/internal/schema"
)

type Options struct {
	DSN        string
	SchemaFile string // path to SQL DDL file; mutually exclusive with DSN
	Schemas    []string
	SchemaAll  bool
}

type Driver interface {
	Introspect(ctx context.Context, opts Options) (*schema.Model, error)
}

func Run(ctx context.Context, dialect string, opts Options) (*schema.Model, error) {
	if opts.SchemaFile != "" {
		defaultSchema := "public"
		if len(opts.Schemas) > 0 {
			defaultSchema = opts.Schemas[0]
		}
		return (&DDL{}).Introspect(opts.SchemaFile, defaultSchema)
	}
	switch dialect {
	case "", "postgres", "postgresql", "pg":
		return (&Postgres{}).Introspect(ctx, opts)
	default:
		return nil, fmt.Errorf("unsupported dialect %q (only postgres in MVP)", dialect)
	}
}
