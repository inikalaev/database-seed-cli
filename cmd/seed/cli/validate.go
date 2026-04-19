package cli

import (
	"fmt"

	"github.com/ivannikolaev/seed-cli/cli/internal/config"
	"github.com/ivannikolaev/seed-cli/cli/internal/mechanisms"
	"github.com/ivannikolaev/seed-cli/cli/internal/registry"
	"github.com/ivannikolaev/seed-cli/cli/internal/relations"
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
			reg := registry.New(mechanisms.All())
			issues := 0
			for key, t := range cfg.Tables {
				if t.Removed {
					continue
				}
				for cname, col := range t.Columns {
					if col.Removed {
						continue
					}
					if col.Unresolved {
						fmt.Fprintf(cmd.OutOrStdout(), "unresolved: %s.%s\n", key, cname)
						issues++
					}
					if _, ok := reg.Get(col.Mechanism); !ok {
						fmt.Fprintf(cmd.OutOrStdout(), "unknown mechanism: %s.%s uses %q\n", key, cname, col.Mechanism)
						issues++
					}
					if col.Mechanism == "fkref" {
						if target, _ := col.Params["target"].(string); target == "" {
							fmt.Fprintf(cmd.OutOrStdout(), "fkref missing target: %s.%s\n", key, cname)
							issues++
						}
					}
				}
			}
			g, err := relations.Build(cfg)
			if err != nil {
				return err
			}
			plan := g.PlanFor(cfg)
			if len(plan.Cycles) > 0 {
				for _, c := range plan.Cycles {
					fmt.Fprintf(cmd.OutOrStdout(), "fk cycle: %v (will use DEFERRABLE)\n", refsKeys(c))
				}
			}
			if issues > 0 {
				return fmt.Errorf("%d issue(s) found", issues)
			}
			fmt.Fprintln(cmd.OutOrStdout(), "ok")
			return nil
		},
	}
	cmd.Flags().StringVarP(&opts.config, "config", "c", "seed.yaml", "Path to the config file")
	return cmd
}

func refsKeys(refs []relations.TableRef) []string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = r.Key()
	}
	return out
}
