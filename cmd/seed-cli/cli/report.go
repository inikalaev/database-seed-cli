package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/fatih/color"
	"github.com/inikalaev/database-seed-cli/internal/validate"
)

var (
	colorErr  = color.New(color.FgRed, color.Bold)
	colorWarn = color.New(color.FgYellow, color.Bold)
	colorInfo = color.New(color.FgCyan)
	colorOK   = color.New(color.FgGreen, color.Bold)
	colorDim  = color.New(color.Faint)
	colorHint = color.New(color.FgMagenta)
)

const maxLocWidth = 45

// printIssues renders validate.Issue list with colored severity tag, dimmed
// location, and a hint column. When fixable issues exist it prints a trailing
// "run seed-cli fix" pointer. Empty list prints "ok" to stdout.
func printIssues(out, errOut io.Writer, issues []validate.Issue, configPath string) {
	if len(issues) == 0 {
		colorOK.Fprintln(out, "ok")
		return
	}

	maxLoc := 0
	maxMsg := 0
	for _, i := range issues {
		if l := len(i.Location); l > maxLoc {
			maxLoc = l
		}
		if l := len(i.Message); l > maxMsg {
			maxMsg = l
		}
	}
	if maxLoc > maxLocWidth {
		maxLoc = maxLocWidth
	}
	// Cap message width so long messages don't push hints to column 120+.
	if maxMsg > 50 {
		maxMsg = 50
	}

	for _, i := range issues {
		var clr *color.Color
		var tag string
		switch i.Level {
		case validate.LevelErr:
			clr, tag = colorErr, "ERR "
		case validate.LevelWarn:
			clr, tag = colorWarn, "WARN"
		case validate.LevelInfo:
			clr, tag = colorInfo, "INFO"
		}
		loc := i.Location
		if len(loc) > maxLocWidth {
			loc = "…" + loc[len(loc)-maxLocWidth+1:]
		}
		clr.Fprintf(errOut, "%-4s", tag)
		colorDim.Fprintf(errOut, "  %-*s  ", maxLoc, loc)
		if i.Hint != "" {
			fmt.Fprintf(errOut, "%-*s  ", maxMsg, i.Message)
			colorHint.Fprintf(errOut, "→ %s", i.Hint)
			fmt.Fprintln(errOut)
		} else {
			fmt.Fprintln(errOut, i.Message)
		}
	}

	errs, warns, infos := validate.Counts(issues)
	fmt.Fprintln(errOut)
	var parts []string
	if errs > 0 {
		parts = append(parts, colorErr.Sprintf("%d error(s)", errs))
	}
	if warns > 0 {
		parts = append(parts, colorWarn.Sprintf("%d warning(s)", warns))
	}
	if infos > 0 {
		parts = append(parts, colorInfo.Sprintf("%d info", infos))
	}
	fmt.Fprintln(errOut, strings.Join(parts, "  ·  "))

	if validate.HasFixable(issues) {
		fmt.Fprintln(errOut)
		colorHint.Fprintf(errOut, "→ run `seed-cli fix -c %s` to resolve interactively\n", configPath)
	}
}
