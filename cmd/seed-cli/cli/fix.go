package cli

import (
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/AlecAivazis/survey/v2/terminal"
	"github.com/inikalaev/database-seed-cli/internal/config"
	"github.com/inikalaev/database-seed-cli/internal/registry"
	"github.com/inikalaev/database-seed-cli/internal/validate"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

type fixOpts struct {
	config string
	dryRun bool
}

// fixResult tracks the outcome of a single issue prompt. It drives the outer
// loop's counters and messaging.
type fixResult int

const (
	fixApplied fixResult = iota
	fixSkipped
	fixUnfixable // no flow for this Kind
)

func newFixCmd() *cobra.Command {
	var opts fixOpts
	cmd := &cobra.Command{
		Use:   "fix",
		Short: "Interactively resolve validate findings: pick factories, set FK targets, adjust row counts.",
		Long: "Walks through every fixable issue reported by `validate` and prompts for a " +
			"resolution. Each accepted fix is saved to the config file immediately, so you " +
			"can Ctrl+C at any point and re-run `seed-cli fix` later to continue. Note: " +
			"saving re-orders columns alphabetically and drops comments, same as `sync`.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runFix(cmd, opts)
		},
	}
	cmd.Flags().StringVarP(&opts.config, "config", "c", "seed.yaml", "Path to the config file")
	cmd.Flags().BoolVar(&opts.dryRun, "dry-run", false, "Walk through issues without writing changes to disk")
	return cmd
}

func runFix(cmd *cobra.Command, opts fixOpts) error {
	if !isatty.IsTerminal(os.Stdin.Fd()) && !isatty.IsCygwinTerminal(os.Stdin.Fd()) {
		return fmt.Errorf("fix requires an interactive terminal (stdin is not a TTY)")
	}
	cfg, err := config.Load(opts.config)
	if err != nil {
		return err
	}
	reg := registry.Default()
	issues, err := validate.Check(cfg, reg)
	if err != nil {
		return err
	}

	fixable := make([]validate.Issue, 0, len(issues))
	for _, i := range issues {
		if i.Fix != nil {
			fixable = append(fixable, i)
		}
	}
	sort.SliceStable(fixable, func(i, j int) bool {
		if fixable[i].Level != fixable[j].Level {
			return fixable[i].Level < fixable[j].Level
		}
		return fixable[i].Location < fixable[j].Location
	})

	out := cmd.OutOrStdout()
	errOut := cmd.ErrOrStderr()

	if len(fixable) == 0 {
		colorOK.Fprintln(out, "nothing to fix")
		return nil
	}

	fmt.Fprintf(errOut, "Found ")
	colorWarn.Fprintf(errOut, "%d", len(fixable))
	fmt.Fprintf(errOut, " fixable issue(s). Ctrl+C at any time — your edits are saved after each fix.\n")
	if opts.dryRun {
		colorHint.Fprintln(errOut, "dry-run: config will NOT be written to disk")
	}
	fmt.Fprintln(errOut)

	applied, skipped := 0, 0
	interrupted := false
	for idx, issue := range fixable {
		printHeader(errOut, idx+1, len(fixable), issue)
		result, err := runFixFlow(cfg, reg, issue)
		if errors.Is(err, terminal.InterruptErr) {
			interrupted = true
			break
		}
		if err != nil {
			fmt.Fprintf(errOut, "  error: %v\n\n", err)
			skipped++
			continue
		}
		switch result {
		case fixApplied:
			applied++
			if !opts.dryRun {
				if err := config.Save(opts.config, cfg); err != nil {
					return fmt.Errorf("save %s: %w", opts.config, err)
				}
			}
			colorOK.Fprintln(errOut, "  ✓ applied")
		case fixSkipped:
			skipped++
			colorDim.Fprintln(errOut, "  skipped")
		}
		fmt.Fprintln(errOut)
	}

	remainingAuto := len(fixable) - applied - skipped
	nonAuto := len(issues) - len(fixable)

	fmt.Fprintln(errOut)
	summary := []string{}
	if applied > 0 {
		summary = append(summary, colorOK.Sprintf("%d fixed", applied))
	}
	if skipped > 0 {
		summary = append(summary, colorDim.Sprintf("%d skipped", skipped))
	}
	if remainingAuto > 0 {
		summary = append(summary, colorWarn.Sprintf("%d remaining (interrupted)", remainingAuto))
	}
	if nonAuto > 0 {
		summary = append(summary, colorInfo.Sprintf("%d non-auto", nonAuto))
	}
	fmt.Fprintln(errOut, strings.Join(summary, "  ·  "))

	if interrupted {
		fmt.Fprintln(errOut)
		colorHint.Fprintf(errOut, "→ run `seed-cli fix -c %s` again to continue\n", opts.config)
		return nil
	}
	if applied > 0 {
		fmt.Fprintln(errOut)
		colorHint.Fprintf(errOut, "→ run `seed-cli validate -c %s` to re-check\n", opts.config)
	}
	return nil
}

func printHeader(w interface{ Write(p []byte) (int, error) }, idx, total int, issue validate.Issue) {
	var clr, tag = colorWarn, "WARN"
	switch issue.Level {
	case validate.LevelErr:
		clr, tag = colorErr, "ERR "
	case validate.LevelInfo:
		clr, tag = colorInfo, "INFO"
	}
	colorDim.Fprintf(w, "[%d/%d] ", idx, total)
	clr.Fprintf(w, "%s  ", tag)
	fmt.Fprintf(w, "%s  %s", issue.Location, issue.Message)
	if issue.Hint != "" {
		colorHint.Fprintf(w, "  → %s", issue.Hint)
	}
	fmt.Fprintln(w)
}

// runFixFlow dispatches to the right flow based on Kind. Returns fixUnfixable
// for Kinds that reached here without a flow (programming error).
func runFixFlow(cfg *config.Config, reg *registry.Registry, issue validate.Issue) (fixResult, error) {
	if issue.Fix == nil {
		return fixUnfixable, nil
	}
	switch issue.Fix.Kind {
	case validate.KindUnresolved, validate.KindNoFactory:
		return flowChooseFactory(cfg, reg, issue)
	case validate.KindUnknownFactory:
		return flowReplaceFactory(cfg, reg, issue)
	case validate.KindValueTypeMismatch:
		return flowValueType(cfg, issue)
	case validate.KindFKRefMissingTarget, validate.KindFKRefTargetNotFound:
		return flowFKTarget(cfg, issue)
	case validate.KindRowCountPerMissing:
		return flowRowCountPer(cfg, issue)
	case validate.KindFKRefEmptyPool:
		return flowFKEmptyPool(cfg, issue)
	case validate.KindFKRefInCycle:
		return flowFKInCycle(cfg, issue)
	case validate.KindUniqueUnsafeFactory:
		return flowUniqueFactory(cfg, reg, issue)
	}
	return fixUnfixable, nil
}
