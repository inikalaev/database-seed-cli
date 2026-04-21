package cli

import (
	"context"
	"fmt"
	"time"

	"github.com/inikalaev/database-seed-cli/internal/config"
	"github.com/inikalaev/database-seed-cli/internal/introspect"
	"github.com/inikalaev/database-seed-cli/internal/registry"
	"github.com/spf13/cobra"
)

type syncOpts struct {
	dsn        string
	schemaFile string
	config     string
	schemas    []string
	schemaAll  bool
	only       []string
	exclude    []string
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
				Only:       opts.only,
				Exclude:    opts.exclude,
			})
			if err != nil {
				return err
			}
			reg := registry.Default()
			incoming := config.FromModel(model, reg, existing.Defaults)
			// Pull out-of-scope tables aside so Merge doesn't flag them
			// `removed: true` just because --only/--exclude hid them from this
			// sync. They rejoin the merged config unchanged. A scoped copy
			// avoids mutating the loaded config in place.
			scoped := &config.Config{
				Version:  existing.Version,
				Database: existing.Database,
				Defaults: existing.Defaults,
				Tables:   map[string]*config.Table{},
			}
			// Any schema not covered by this introspection is out of scope — the
			// incoming model only knows about schemas it was asked to inspect, so
			// marking tables in unvisited schemas as Removed would be wrong.
			inScopeSchemas := map[string]bool{}
			for _, s := range incoming.Database.Schemas {
				inScopeSchemas[s] = true
			}
			outOfScope := map[string]*config.Table{}
			for key, t := range existing.Tables {
				if !inScopeSchemas[t.Schema] || !introspect.InScope(t.Schema, t.Name, opts.only, opts.exclude) {
					outOfScope[key] = t
					continue
				}
				scoped.Tables[key] = t
			}
			merged := config.Merge(scoped, incoming)
			for key, t := range outOfScope {
				merged.Tables[key] = t
			}
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
	flags.StringSliceVar(&opts.only, "only", nil, "Include only these tables (comma-separated or repeatable).")
	flags.StringSliceVar(&opts.exclude, "exclude", nil, "Exclude these tables (comma-separated or repeatable).")
	return cmd
}
