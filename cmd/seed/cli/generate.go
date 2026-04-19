package cli

import (
	"fmt"
	"os"

	"github.com/ivannikolaev/seed-cli/cli/internal/buildplugins"
	"github.com/ivannikolaev/seed-cli/cli/internal/config"
	"github.com/ivannikolaev/seed-cli/cli/internal/mechanisms"
	"github.com/ivannikolaev/seed-cli/cli/internal/registry"
	"github.com/ivannikolaev/seed-cli/cli/internal/relations"
	"github.com/ivannikolaev/seed-cli/cli/internal/sqlemit"
	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
	"github.com/spf13/cobra"
)

type generateOpts struct {
	config     string
	output     string
	generators string
	seed       int64
}

func newGenerateCmd() *cobra.Command {
	var opts generateOpts
	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Emit a SQL seed script from the config.",
		Long: "Reads the YAML config, resolves the FK graph, runs every mechanism and writes " +
			"a single SQL file honoring insert order. MVP does not apply the script to the DB — " +
			"run it with `psql -f seed.sql` yourself.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			// Delegate to augmented binary when user plugins are provided.
			if opts.generators != "" && !inAugmented() {
				args := []string{"generate", "-c", opts.config, "-o", opts.output}
				if opts.seed != 0 {
					args = append(args, fmt.Sprintf("--seed=%d", opts.seed))
				}
				return buildplugins.RunWithGenerators(opts.generators, args)
			}

			cfg, err := config.Load(opts.config)
			if err != nil {
				return fmt.Errorf("load %s: %w", opts.config, err)
			}
			// Builtins + anything user plugins registered via seedapi.Register in init().
			mechs := append([]seedapi.Mechanism{}, mechanisms.All()...)
			mechs = append(mechs, seedapi.Default().All()...)
			reg := registry.New(dedupMechanisms(mechs))

			g, err := relations.Build(cfg)
			if err != nil {
				return err
			}
			plan := g.PlanFor(cfg)

			f, err := os.Create(opts.output)
			if err != nil {
				return err
			}
			defer f.Close()

			seed := opts.seed
			if seed == 0 {
				seed = cfg.Defaults.Seed
			}
			em := sqlemit.New(cfg, reg, plan, sqlemit.Options{Locale: cfg.Defaults.Locale, Seed: seed})
			if err := em.Emit(f); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "wrote %s\n", opts.output)
			return nil
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&opts.config, "config", "c", "seed.yaml", "Path to the config file")
	flags.StringVarP(&opts.output, "output", "o", "seed.sql", "Path to write the SQL script")
	flags.StringVar(&opts.generators, "generators", "", "Directory with user-provided Go mechanism files")
	flags.Int64Var(&opts.seed, "seed", 0, "Override deterministic seed from config.defaults.seed")
	return cmd
}

func inAugmented() bool {
	// The augmented binary sets this before exec'ing us; the marker prevents recursion.
	return os.Getenv("SEED_CLI_AUGMENTED") == "1"
}

func dedupMechanisms(in []seedapi.Mechanism) []seedapi.Mechanism {
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
