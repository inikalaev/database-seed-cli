// Package seed is the public library surface of the seed CLI.
//
// Anything a host app (Go service, wrapper for Ruby/Python, embedding product)
// needs to read a seed config and produce SQL lives here. internal/* packages
// are not importable across module boundaries; pkg/seed re-exports the pieces
// the outside world may depend on.
package seed

import (
	"bytes"
	"context"
	"database/sql"
	"fmt"
	"strings"

	"github.com/inikalaev/database-seed-cli/internal/config"
	"github.com/inikalaev/database-seed-cli/internal/registry"
	"github.com/inikalaev/database-seed-cli/internal/relations"
	"github.com/inikalaev/database-seed-cli/internal/sqlemit"
)

type (
	Config             = config.Config
	Table              = config.Table
	ColumnSpec         = config.ColumnSpec
	CheckConstraint    = config.CheckConstraint
	ExcludeConstraint  = config.ExcludeConstraint
	PartialUniqueKey   = config.PartialUniqueKey
	PolymorphicKey     = config.PolymorphicKey
	PolymorphCandidate = config.PolymorphCandidate
	DefaultsSection    = config.DefaultsSection
	DatabaseSection    = config.DatabaseSection
)

// Options controls non-fatal behaviour of the public seed API. The zero value
// matches the historical CLI behaviour (warnings on stderr). Library
// consumers embedding seed inside another process can set Logger to capture
// messages programmatically.
type Options struct {
	// Logger receives human-readable warnings from config loading and SQL
	// emission (e.g., missing version field, unknown factory fallback).
	// nil = write to stderr.
	Logger func(message string)
}

// Load reads a config from disk. Variadic opts allows callers to supply an
// Options value without touching existing two-argument call sites.
func Load(path string, opts ...Options) (*Config, error) {
	o := firstOptions(opts)
	var copts []config.Option
	if o.Logger != nil {
		copts = append(copts, config.WithLogger(o.Logger))
	}
	return config.Load(path, copts...)
}

// Generate produces the SQL seed script in memory using the built-in mechanisms
// plus anything user code has registered via seedapi.Register.
func Generate(cfg *Config, opts ...Options) ([]byte, error) {
	o := firstOptions(opts)
	g, err := relations.Build(cfg)
	if err != nil {
		return nil, err
	}
	plan := g.PlanFor(cfg)
	em := sqlemit.New(cfg, registry.Default(), plan, sqlemit.Options{
		Locale: cfg.Defaults.Locale,
		Seed:   cfg.Defaults.Seed,
		Logger: o.Logger,
	})
	var buf bytes.Buffer
	if err := em.Emit(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Apply generates the SQL and executes it against db, one statement at a time.
// This is compatible with any database/sql driver regardless of multi-statement
// support. Each statement is executed in sequence; the generated script opens
// its own BEGIN/COMMIT so do not wrap Apply in another transaction.
func Apply(ctx context.Context, db *sql.DB, cfg *Config, opts ...Options) error {
	script, err := Generate(cfg, opts...)
	if err != nil {
		return err
	}
	for _, stmt := range splitStatements(string(script)) {
		if _, err := db.ExecContext(ctx, stmt); err != nil {
			return fmt.Errorf("exec seed script: %w", err)
		}
	}
	return nil
}

func firstOptions(opts []Options) Options {
	if len(opts) == 0 {
		return Options{}
	}
	return opts[0]
}

// splitStatements splits a SQL script into individual semicolon-terminated
// statements. Handles single-quoted string literals (including ” escapes) and
// skips -- line comments so semicolons inside comments are not treated as
// statement terminators.
func splitStatements(sql string) []string {
	var out []string
	var buf strings.Builder
	inStr := false
	for i := 0; i < len(sql); i++ {
		ch := sql[i]
		switch {
		case !inStr && ch == '-' && i+1 < len(sql) && sql[i+1] == '-':
			for i < len(sql) && sql[i] != '\n' {
				i++
			}
		case inStr && ch == '\'' && i+1 < len(sql) && sql[i+1] == '\'':
			buf.WriteByte(ch)
			buf.WriteByte(sql[i+1])
			i++
		case inStr && ch == '\'':
			buf.WriteByte(ch)
			inStr = false
		case !inStr && ch == '\'':
			buf.WriteByte(ch)
			inStr = true
		case !inStr && ch == ';':
			if s := strings.TrimSpace(buf.String()); s != "" {
				out = append(out, s)
			}
			buf.Reset()
		default:
			buf.WriteByte(ch)
		}
	}
	if s := strings.TrimSpace(buf.String()); s != "" {
		out = append(out, s)
	}
	return out
}
