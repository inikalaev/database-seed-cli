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

	"github.com/ivannikolaev/seed-cli/cli/internal/config"
	"github.com/ivannikolaev/seed-cli/cli/internal/mechanisms"
	"github.com/ivannikolaev/seed-cli/cli/internal/registry"
	"github.com/ivannikolaev/seed-cli/cli/internal/relations"
	"github.com/ivannikolaev/seed-cli/cli/internal/sqlemit"
	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type Config = config.Config

func Load(path string) (*Config, error) { return config.Load(path) }

// Generate produces the SQL seed script in memory using the built-in mechanisms
// plus anything user code has registered via seedapi.Register.
func Generate(cfg *Config) ([]byte, error) {
	reg := buildRegistry()
	g, err := relations.Build(cfg)
	if err != nil {
		return nil, err
	}
	plan := g.PlanFor(cfg)
	em := sqlemit.New(cfg, reg, plan, sqlemit.Options{Locale: cfg.Defaults.Locale, Seed: cfg.Defaults.Seed})
	var buf bytes.Buffer
	if err := em.Emit(&buf); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Apply generates the SQL and executes it against db inside a single transaction.
func Apply(ctx context.Context, db *sql.DB, cfg *Config) error {
	script, err := Generate(cfg)
	if err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, string(script)); err != nil {
		_ = tx.Rollback()
		return fmt.Errorf("exec seed script: %w", err)
	}
	return tx.Commit()
}

func buildRegistry() *registry.Registry {
	mechs := append([]seedapi.Mechanism{}, mechanisms.All()...)
	mechs = append(mechs, seedapi.Default().All()...)
	return registry.New(dedup(mechs))
}

func dedup(in []seedapi.Mechanism) []seedapi.Mechanism {
	seen := map[string]bool{}
	out := make([]seedapi.Mechanism, 0, len(in))
	for _, m := range in {
		if seen[m.Name()] {
			continue
		}
		seen[m.Name()] = true
		out = append(out, m)
	}
	return out
}
