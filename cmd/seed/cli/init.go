package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	"github.com/ivannikolaev/seed-cli/cli/internal/config"
	"github.com/ivannikolaev/seed-cli/cli/internal/introspect"
	"github.com/ivannikolaev/seed-cli/cli/internal/mechanisms"
	"github.com/ivannikolaev/seed-cli/cli/internal/registry"
	"github.com/spf13/cobra"
)

type initOpts struct {
	dsn        string
	schemaFile string
	output     string
	schemas    []string
	schemaAll  bool
	locale     string
	seed       int64
}

func newInitCmd() *cobra.Command {
	var opts initOpts
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Introspect the target database and write a fresh seed config.",
		Long: "Introspects the database pointed to by --dsn (or parses --schema-file) and writes a YAML seed config. " +
			"If --output already exists, use `seed-cli sync` instead — `init` refuses to overwrite.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireSource(opts.dsn, opts.schemaFile); err != nil {
				return err
			}
			if _, err := os.Stat(opts.output); err == nil {
				return fmt.Errorf("%s already exists — use `seed-cli sync` to merge", opts.output)
			}
			ctx, cancel := context.WithTimeout(cmd.Context(), 60*time.Second)
			defer cancel()
			model, err := introspect.Run(ctx, "postgres", introspect.Options{
				DSN:        opts.dsn,
				SchemaFile: opts.schemaFile,
				Schemas:    opts.schemas,
				SchemaAll:  opts.schemaAll,
			})
			if err != nil {
				return err
			}
			reg := registry.New(mechanisms.All())
			cfg := config.FromModel(model, reg, config.DefaultsSection{Locale: opts.locale, Seed: opts.seed})
			if err := config.Save(opts.output, cfg); err != nil {
				return err
			}
			warnUnresolved(cmd.OutOrStderr(), cfg)
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s (%d tables)\n", opts.output, len(cfg.Tables))
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.dsn, "dsn", "", "PostgreSQL DSN (e.g. postgres://user:pass@host/db)")
	flags.StringVar(&opts.schemaFile, "schema-file", "", "Path to SQL DDL file (e.g. from pg_dump --schema-only)")
	flags.StringVarP(&opts.output, "output", "o", "seed.yaml", "Path to the config file to create")
	flags.StringSliceVar(&opts.schemas, "schema", nil, "PG schema to include (repeatable). Defaults to `public`.")
	flags.BoolVar(&opts.schemaAll, "schema-all", false, "Include every non-system schema")
	flags.StringVar(&opts.locale, "locale", "en_US", "Default locale for mechanisms")
	flags.Int64Var(&opts.seed, "seed", 0, "Deterministic seed for generators (0 = random)")
	return cmd
}

func warnUnresolved(w interface{ Write(p []byte) (int, error) }, cfg *config.Config) {
	total := 0
	for _, t := range cfg.Tables {
		for _, c := range t.Columns {
			if c.Unresolved {
				total++
			}
		}
	}
	if total > 0 {
		fmt.Fprintf(w, "warning: %d columns need review (search for `unresolved: true`)\n", total)
	}
}

// requireSource ensures exactly one of dsn or schemaFile is provided.
func requireSource(dsn, schemaFile string) error {
	if dsn == "" && schemaFile == "" {
		return errors.New("one of --dsn or --schema-file is required")
	}
	if dsn != "" && schemaFile != "" {
		return errors.New("--dsn and --schema-file are mutually exclusive")
	}
	return nil
}

var errPlaceholder = errors.New("placeholder")

var _ = errPlaceholder
