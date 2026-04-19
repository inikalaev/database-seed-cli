package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/ivannikolaev/seed-cli/cli/internal/config"
	"github.com/ivannikolaev/seed-cli/cli/internal/introspect"
	"github.com/ivannikolaev/seed-cli/cli/internal/mechanisms"
	"github.com/ivannikolaev/seed-cli/cli/internal/registry"
	"github.com/spf13/cobra"
)

type syncOpts struct {
	dsn        string
	schemaFile string
	config     string
	schemas    []string
	schemaAll  bool
}

func newSyncCmd() *cobra.Command {
	var opts syncOpts
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Re-introspect the database and merge changes into an existing config.",
		Long: "Idempotent merge: schema-derived fields are refreshed, user-authored fields " +
			"(mechanism, params, row_count, tags) are preserved. Removed tables and " +
			"columns are marked with `removed: true` rather than deleted.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireSource(opts.dsn, opts.schemaFile); err != nil {
				return err
			}
			existing, err := config.Load(opts.config)
			if err != nil {
				return fmt.Errorf("load %s: %w", opts.config, err)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			model, err := introspect.Run(ctx, existing.Database.Dialect, introspect.Options{
				DSN:        opts.dsn,
				SchemaFile: opts.schemaFile,
				Schemas:    opts.schemas,
				SchemaAll:  opts.schemaAll,
			})
			if err != nil {
				return err
			}
			reg := registry.New(mechanisms.All())
			incoming := config.FromModel(model, reg, existing.Defaults)
			merged := config.Merge(existing, incoming)
			if err := config.Save(opts.config, merged); err != nil {
				return err
			}
			warnUnresolved(cmd.OutOrStderr(), merged)
			fmt.Fprintf(cmd.OutOrStdout(), "updated %s (%d tables)\n", opts.config, len(merged.Tables))
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.dsn, "dsn", "", "PostgreSQL DSN")
	flags.StringVar(&opts.schemaFile, "schema-file", "", "Path to SQL DDL file (e.g. from pg_dump --schema-only)")
	flags.StringVarP(&opts.config, "config", "c", "seed.yaml", "Path to the existing config file")
	flags.StringSliceVar(&opts.schemas, "schema", nil, "PG schema to include (repeatable)")
	flags.BoolVar(&opts.schemaAll, "schema-all", false, "Include every non-system schema")
	return cmd
}
