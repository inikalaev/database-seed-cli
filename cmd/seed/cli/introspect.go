package cli

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ivannikolaev/seed-cli/cli/internal/introspect"
	"github.com/spf13/cobra"
)

type introspectOpts struct {
	dsn        string
	schemaFile string
	schemas    []string
	schemaAll  bool
}

func newIntrospectCmd() *cobra.Command {
	var opts introspectOpts
	cmd := &cobra.Command{
		Use:   "introspect",
		Short: "Dump the resolved schema model as JSON (debug helper).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireSource(opts.dsn, opts.schemaFile); err != nil {
				return err
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
			enc := json.NewEncoder(cmd.OutOrStdout())
			enc.SetIndent("", "  ")
			return enc.Encode(model)
		},
	}
	flags := cmd.Flags()
	flags.StringVar(&opts.dsn, "dsn", "", "PostgreSQL DSN")
	flags.StringVar(&opts.schemaFile, "schema-file", "", "Path to SQL DDL file (e.g. from pg_dump --schema-only)")
	flags.StringSliceVar(&opts.schemas, "schema", nil, "PG schema to include (repeatable)")
	flags.BoolVar(&opts.schemaAll, "schema-all", false, "Include every non-system schema")
	_ = fmt.Sprint // silence
	return cmd
}
