package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/inikalaev/database-seed-cli/internal/buildplugins"
	"github.com/inikalaev/database-seed-cli/internal/config"
	"github.com/inikalaev/database-seed-cli/internal/registry"
	"github.com/inikalaev/database-seed-cli/internal/relations"
	"github.com/inikalaev/database-seed-cli/internal/sqlemit"
	"github.com/inikalaev/database-seed-cli/internal/validate"
	"github.com/spf13/cobra"
)

type generateOpts struct {
	config    string
	output    string
	factories string
	seed      int64
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
			return runGenerate(cmd, opts)
		},
	}
	flags := cmd.Flags()
	flags.StringVarP(&opts.config, "config", "c", "seed.yaml", "Path to the config file")
	flags.StringVarP(&opts.output, "output", "o", "seed.sql", "Path to write the SQL script")
	flags.StringVar(&opts.factories, "factories", "", "Directory with user-provided Go factory files (requires SEED_CLI_SRC env var)")
	_ = flags.MarkHidden("factories")
	flags.Int64Var(&opts.seed, "seed", 0, "Override deterministic seed from config.defaults.seed")
	return cmd
}

func runGenerate(cmd *cobra.Command, opts generateOpts) error {
	if opts.factories != "" && !inAugmented() {
		args := []string{"generate", "-c", opts.config, "-o", opts.output}
		if opts.seed != 0 {
			args = append(args, fmt.Sprintf("--seed=%d", opts.seed))
		}
		return buildplugins.RunWithFactories(opts.factories, args)
	}

	cfg, err := config.Load(opts.config)
	if err != nil {
		return fmt.Errorf("load %s: %w", opts.config, err)
	}
	reg := registry.Default()

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

	warnings := collectGenerateWarnings(plan, em.Drops())
	if len(warnings) > 0 {
		printIssues(cmd.OutOrStdout(), cmd.ErrOrStderr(), warnings, opts.config)
	}

	_, warns, _ := validate.Counts(warnings)
	suffix := ""
	if warns > 0 {
		suffix = colorWarn.Sprintf("  (%d warning(s))", warns)
	}
	colorOK.Fprintf(cmd.OutOrStdout(), "→ wrote %s", opts.output)
	fmt.Fprintln(cmd.OutOrStdout(), suffix)
	return nil
}

func collectGenerateWarnings(plan *relations.Plan, drops map[string]sqlemit.DropInfo) []validate.Issue {
	var warnings []validate.Issue
	for _, c := range plan.Cycles {
		if bad := plan.NonDeferrableEdgesIn(c); len(bad) > 0 {
			names := make([]string, len(bad))
			for i, e := range bad {
				names[i] = e.From.Key() + "." + e.Column
			}
			warnings = append(warnings, validate.Issue{
				Level:    validate.LevelWarn,
				Location: "fk cycle",
				Message:  fmt.Sprintf("non-deferrable edges %v — apply will fail", names),
				Hint:     "run `seed-cli validate` for fix options",
			})
		}
	}
	for key, causes := range plan.CascadedFrom {
		parents := make([]string, 0, len(causes))
		for _, c := range causes {
			parents = append(parents, c.Parent+"."+c.Column)
		}
		warnings = append(warnings, validate.Issue{
			Level:    validate.LevelWarn,
			Location: key,
			Message:  fmt.Sprintf("row_count → 0 (empty parent: %s)", strings.Join(parents, ", ")),
			Hint:     "raise parent row_count to generate rows for this table",
		})
	}
	for key, d := range drops {
		warnings = append(warnings, validate.Issue{
			Level:    validate.LevelWarn,
			Location: key,
			Message:  fmt.Sprintf("emitted %d/%d rows (%s)", d.Emitted, d.Requested, d.Reason),
			Hint:     "relax unique/row_count constraints or inspect the factory",
		})
	}
	return warnings
}

func inAugmented() bool {
	return os.Getenv("SEED_CLI_AUGMENTED") == "1"
}
