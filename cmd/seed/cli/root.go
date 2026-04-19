package cli

import (
	"github.com/spf13/cobra"
)

const (
	appName    = "seed-cli"
	appShort   = "Generate idempotent seed configs and SQL scripts from a live database schema."
	appVersion = "0.0.0-dev"
)

func NewRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           appName,
		Short:         appShort,
		Version:       appVersion,
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	root.AddCommand(
		newInitCmd(),
		newSyncCmd(),
		newIntrospectCmd(),
		newGenerateCmd(),
		newValidateCmd(),
	)

	return root
}
