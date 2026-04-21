package cli

import (
	"fmt"

	"github.com/inikalaev/database-seed-cli/internal/config"
	"github.com/inikalaev/database-seed-cli/internal/registry"
	"github.com/inikalaev/database-seed-cli/internal/validate"
	"github.com/spf13/cobra"
)

type validateOpts struct {
	config string
}

func newValidateCmd() *cobra.Command {
	var opts validateOpts
	cmd := &cobra.Command{
		Use:   "validate",
		Short: "Lint the config: unresolved columns, unknown mechanisms, FK cycles, missing sources.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			cfg, err := config.Load(opts.config)
			if err != nil {
				return err
			}
			reg := registry.Default()
			issues, err := validate.Check(cfg, reg)
			if err != nil {
				return err
			}
			printIssues(cmd.OutOrStdout(), cmd.ErrOrStderr(), issues, opts.config)
			if validate.HasErrors(issues) {
				errs, _, _ := validate.Counts(issues)
				return fmt.Errorf("%d error(s) found", errs)
			}
			return nil
		},
	}
	cmd.Flags().StringVarP(&opts.config, "config", "c", "seed.yaml", "Path to the config file")
	return cmd
}
