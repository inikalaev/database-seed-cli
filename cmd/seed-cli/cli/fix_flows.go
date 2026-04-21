package cli

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/AlecAivazis/survey/v2"
	"github.com/inikalaev/database-seed-cli/internal/config"
	"github.com/inikalaev/database-seed-cli/internal/factories"
	"github.com/inikalaev/database-seed-cli/internal/registry"
	"github.com/inikalaev/database-seed-cli/internal/validate"
	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

// Common sentinel option labels. Kept as constants so each flow reads the same
// wording and we can spot user-visible strings in one place.
const (
	optSkip        = "skip for now"
	optManualEntry = "enter value manually"
	optKeepAsIs    = "keep as-is"
)

// flowChooseFactory covers both KindUnresolved (low-confidence inference) and
// KindNoFactory (no factory at all). Both resolve the same way — pick a factory
// and clear Unresolved.
func flowChooseFactory(cfg *config.Config, reg *registry.Registry, issue validate.Issue) (fixResult, error) {
	col, err := lookupColumn(cfg, issue.Fix.Table, issue.Fix.Column)
	if err != nil {
		return fixSkipped, err
	}

	apiCol := buildAPIColumn(cfg, issue.Fix.Table, issue.Fix.Column)
	ranked := rankFactories(reg, apiCol)
	top := ranked
	if len(top) > 8 {
		top = top[:8]
	}

	options := make([]string, 0, len(top)+3)
	labelToName := map[string]string{}
	for _, rf := range top {
		label := fmt.Sprintf("%s  (score %d)", rf.name, rf.score)
		if rf.name == col.Factory {
			label += "  [current]"
		}
		options = append(options, label)
		labelToName[label] = rf.name
	}
	options = append(options, optManualEntry, "keep as unresolved", optSkip)

	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: "Pick a factory:",
		Options: options,
		Default: options[0],
	}, &choice); err != nil {
		return fixSkipped, err
	}

	switch choice {
	case optSkip:
		return fixSkipped, nil
	case "keep as unresolved":
		col.Unresolved = true
		return fixSkipped, nil
	case optManualEntry:
		var name string
		if err := survey.AskOne(&survey.Input{Message: "Factory name:"}, &name,
			survey.WithValidator(survey.Required)); err != nil {
			return fixSkipped, err
		}
		col.Factory = strings.TrimSpace(name)
		col.Unresolved = false
		return fixApplied, nil
	}
	col.Factory = labelToName[choice]
	col.Unresolved = false
	return fixApplied, nil
}

// flowReplaceFactory handles KindUnknownFactory: the current factory name is
// not registered. User might still want to keep it (it's a plugin), or swap.
func flowReplaceFactory(cfg *config.Config, reg *registry.Registry, issue validate.Issue) (fixResult, error) {
	col, err := lookupColumn(cfg, issue.Fix.Table, issue.Fix.Column)
	if err != nil {
		return fixSkipped, err
	}
	current, _ := issue.Fix.Ctx["current"].(string)

	apiCol := buildAPIColumn(cfg, issue.Fix.Table, issue.Fix.Column)
	ranked := rankFactories(reg, apiCol)

	options := make([]string, 0, len(ranked)+3)
	for _, rf := range ranked {
		options = append(options, rf.name)
	}
	// Alphabetical list of remaining factories (those with 0 score) so the user
	// can still find them via search.
	extra := []string{}
	seen := map[string]bool{}
	for _, o := range options {
		seen[o] = true
	}
	for _, f := range reg.All() {
		if !seen[f.Name()] {
			extra = append(extra, f.Name())
		}
	}
	sort.Strings(extra)
	options = append(options, extra...)

	keepLabel := fmt.Sprintf("keep %q (provided by a plugin)", current)
	options = append(options, keepLabel, optSkip)

	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: fmt.Sprintf("Replace factory %q with:", current),
		Options: options,
	}, &choice); err != nil {
		return fixSkipped, err
	}

	if choice == optSkip || choice == keepLabel {
		return fixSkipped, nil
	}
	col.Factory = choice
	col.Unresolved = false
	return fixApplied, nil
}

// flowValueType handles KindValueTypeMismatch: literal value type clashes with
// declared data_type. Offer: replace value / drop value / skip.
func flowValueType(cfg *config.Config, issue validate.Issue) (fixResult, error) {
	col, err := lookupColumn(cfg, issue.Fix.Table, issue.Fix.Column)
	if err != nil {
		return fixSkipped, err
	}

	const (
		optReplace = "enter a new value"
		optDrop    = "drop `value:` (use factory instead)"
	)
	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: fmt.Sprintf("Current value %v does not match data_type %s. How to fix?", col.Value, col.DataType),
		Options: []string{optReplace, optDrop, optSkip},
	}, &choice); err != nil {
		return fixSkipped, err
	}

	switch choice {
	case optSkip:
		return fixSkipped, nil
	case optDrop:
		col.Value = nil
		return fixApplied, nil
	case optReplace:
		var raw string
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("New value (data_type %s):", col.DataType),
		}, &raw, survey.WithValidator(survey.Required)); err != nil {
			return fixSkipped, err
		}
		col.Value = coerceScalar(raw, col.DataType)
		return fixApplied, nil
	}
	return fixSkipped, nil
}

// flowFKTarget handles KindFKRefMissingTarget and KindFKRefTargetNotFound.
// Lists every PK-looking column as a candidate target.
func flowFKTarget(cfg *config.Config, issue validate.Issue) (fixResult, error) {
	col, err := lookupColumn(cfg, issue.Fix.Table, issue.Fix.Column)
	if err != nil {
		return fixSkipped, err
	}
	targets := scanPKTargets(cfg)
	if len(targets) == 0 {
		colorDim.Println("  no PK columns found in config — skipping")
		return fixSkipped, nil
	}

	options := append([]string{}, targets...)
	options = append(options, optSkip)

	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: "Pick an FK target:",
		Options: options,
	}, &choice); err != nil {
		return fixSkipped, err
	}
	if choice == optSkip {
		return fixSkipped, nil
	}
	if col.Params == nil {
		col.Params = map[string]any{}
	}
	col.Params["target"] = choice
	col.Factory = seedapi.FactoryFKRef
	col.Unresolved = false
	return fixApplied, nil
}

// flowRowCountPer handles KindRowCountPerMissing: a row_count_per key points to
// a parent table that does not exist (typo or removed table).
func flowRowCountPer(cfg *config.Config, issue validate.Issue) (fixResult, error) {
	t, err := lookupTable(cfg, issue.Fix.Table)
	if err != nil {
		return fixSkipped, err
	}
	badKey, _ := issue.Fix.Ctx["parent"].(string)

	existing := make([]string, 0, len(cfg.Tables))
	for k, tbl := range cfg.Tables {
		if !tbl.Removed && k != issue.Fix.Table {
			existing = append(existing, k)
		}
	}
	sort.Strings(existing)

	const optRemove = "remove this entry"
	options := append([]string{optRemove}, existing...)
	options = append(options, optSkip)

	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: fmt.Sprintf("row_count_per[%q] has no matching parent. Replace with:", badKey),
		Options: options,
	}, &choice); err != nil {
		return fixSkipped, err
	}
	if choice == optSkip {
		return fixSkipped, nil
	}
	if choice == optRemove {
		delete(t.RowCountPer, badKey)
		return fixApplied, nil
	}
	// Replace: carry over the [lo, hi] pair under the new key.
	pair := t.RowCountPer[badKey]
	delete(t.RowCountPer, badKey)
	t.RowCountPer[choice] = pair
	return fixApplied, nil
}

// flowFKEmptyPool handles KindFKRefEmptyPool: NOT NULL fkref pointing at a
// table with row_count 0. Three fixes: raise parent row_count, nullable, value.
func flowFKEmptyPool(cfg *config.Config, issue validate.Issue) (fixResult, error) {
	col, err := lookupColumn(cfg, issue.Fix.Table, issue.Fix.Column)
	if err != nil {
		return fixSkipped, err
	}
	parentKey, _ := issue.Fix.Ctx["parentKey"].(string)
	parent, ok := cfg.Tables[parentKey]
	if !ok {
		return fixSkipped, fmt.Errorf("parent %s vanished between validate and fix", parentKey)
	}

	const (
		optRaise    = "set parent row_count"
		optNullable = "mark this column nullable"
		optValue    = "set a literal value"
	)
	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: fmt.Sprintf("Parent %s has row_count 0. How to fix?", parentKey),
		Options: []string{optRaise, optNullable, optValue, optSkip},
	}, &choice); err != nil {
		return fixSkipped, err
	}
	switch choice {
	case optSkip:
		return fixSkipped, nil
	case optNullable:
		col.Nullable = true
		return fixApplied, nil
	case optRaise:
		var raw string
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("New row_count for %s:", parentKey),
			Default: "100",
		}, &raw, survey.WithValidator(intValidator)); err != nil {
			return fixSkipped, err
		}
		n, _ := strconv.Atoi(raw)
		parent.RowCount = &n
		return fixApplied, nil
	case optValue:
		var raw string
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Literal value (data_type %s):", col.DataType),
		}, &raw, survey.WithValidator(survey.Required)); err != nil {
			return fixSkipped, err
		}
		col.Value = coerceScalar(raw, col.DataType)
		return fixApplied, nil
	}
	return fixSkipped, nil
}

// flowFKInCycle handles KindFKRefInCycle: NOT NULL fkref inside a deferrable
// cycle. First row emitted for the cycle has NULL target; either allow NULL or
// pin a literal.
func flowFKInCycle(cfg *config.Config, issue validate.Issue) (fixResult, error) {
	col, err := lookupColumn(cfg, issue.Fix.Table, issue.Fix.Column)
	if err != nil {
		return fixSkipped, err
	}

	const (
		optNullable = "mark column nullable (allow NULL on first emit)"
		optValue    = "set a literal value (bypass FK pool)"
	)
	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: "How to break the first-emit NULL?",
		Options: []string{optNullable, optValue, optSkip},
	}, &choice); err != nil {
		return fixSkipped, err
	}
	switch choice {
	case optSkip:
		return fixSkipped, nil
	case optNullable:
		col.Nullable = true
		return fixApplied, nil
	case optValue:
		var raw string
		if err := survey.AskOne(&survey.Input{
			Message: fmt.Sprintf("Literal value (data_type %s):", col.DataType),
		}, &raw, survey.WithValidator(survey.Required)); err != nil {
			return fixSkipped, err
		}
		col.Value = coerceScalar(raw, col.DataType)
		return fixApplied, nil
	}
	return fixSkipped, nil
}

// flowUniqueFactory handles KindUniqueUnsafeFactory: offers a swap to a safe
// factory (implements seedapi.UniqueGenerator) or accepts the risk.
func flowUniqueFactory(cfg *config.Config, reg *registry.Registry, issue validate.Issue) (fixResult, error) {
	col, err := lookupColumn(cfg, issue.Fix.Table, issue.Fix.Column)
	if err != nil {
		return fixSkipped, err
	}
	var safe []string
	for _, f := range reg.All() {
		if validate.UniqueSafeFactory(f.Name(), reg) {
			safe = append(safe, f.Name())
		}
	}
	sort.Strings(safe)
	const optAccept = "accept risk (keep as-is)"
	options := append([]string{}, safe...)
	options = append(options, optAccept, optSkip)

	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: "Switch to a unique-safe factory?",
		Options: options,
	}, &choice); err != nil {
		return fixSkipped, err
	}
	if choice == optSkip || choice == optAccept {
		return fixSkipped, nil
	}
	col.Factory = choice
	col.Unresolved = false
	return fixApplied, nil
}

// --- helpers ---

type scoredFactory struct {
	name  string
	score seedapi.MatchScore
}

// rankFactories scores every registered factory against a column and returns
// them sorted best-first. Factories with NoMatch are dropped.
func rankFactories(reg *registry.Registry, col seedapi.Column) []scoredFactory {
	ctx := seedapi.MatchContext{Column: col}
	scored := make([]scoredFactory, 0, 32)
	for _, f := range reg.All() {
		var s seedapi.MatchScore
		if m, ok := f.(seedapi.Matcher); ok {
			s = m.Match(ctx)
		} else {
			s = autoScore(f, col)
		}
		if s > seedapi.NoMatch {
			scored = append(scored, scoredFactory{f.Name(), s})
		}
	}
	sort.SliceStable(scored, func(i, j int) bool {
		if scored[i].score != scored[j].score {
			return scored[i].score > scored[j].score
		}
		return scored[i].name < scored[j].name
	})
	return scored
}

// autoScore mirrors registry.autoMatch for factories that don't implement
// seedapi.Matcher. Uses factories.NormName so the scores shown here match
// exactly what init/sync inference would produce.
func autoScore(f seedapi.Factory, col seedapi.Column) seedapi.MatchScore {
	colNorm := factories.NormName(col.Name)
	if colNorm == factories.NormName(f.Name()) {
		return seedapi.StrongMatch
	}
	for _, tag := range f.Tags() {
		if strings.Contains(colNorm, factories.NormName(tag)) {
			return seedapi.NameMatch
		}
	}
	return seedapi.NoMatch
}

func buildAPIColumn(cfg *config.Config, tableKey, colName string) seedapi.Column {
	t := cfg.Tables[tableKey]
	c := t.Columns[colName]
	schema, table := splitTableKey(tableKey)
	fkTarget := ""
	if c != nil && c.Factory == seedapi.FactoryFKRef {
		fkTarget, _ = c.Params["target"].(string)
	}
	out := seedapi.Column{
		Schema: schema,
		Table:  table,
		Name:   colName,
	}
	if c != nil {
		out.DataType = c.DataType
		out.Nullable = c.Nullable
		out.FKTarget = fkTarget
	}
	return out
}

func splitTableKey(key string) (string, string) {
	if idx := strings.Index(key, "."); idx >= 0 {
		return key[:idx], key[idx+1:]
	}
	return "public", key
}

func lookupColumn(cfg *config.Config, tableKey, colName string) (*config.ColumnSpec, error) {
	t, ok := cfg.Tables[tableKey]
	if !ok {
		return nil, fmt.Errorf("table %s not found", tableKey)
	}
	c, ok := t.Columns[colName]
	if !ok {
		return nil, fmt.Errorf("column %s.%s not found", tableKey, colName)
	}
	return c, nil
}

func lookupTable(cfg *config.Config, tableKey string) (*config.Table, error) {
	t, ok := cfg.Tables[tableKey]
	if !ok {
		return nil, fmt.Errorf("table %s not found", tableKey)
	}
	return t, nil
}

// scanPKTargets enumerates every column that looks like a primary key, formatted
// as "schema.table.column". Used by flowFKTarget to offer valid FK targets.
func scanPKTargets(cfg *config.Config) []string {
	var out []string
	for key, t := range cfg.Tables {
		if t.Removed {
			continue
		}
		pks := map[string]bool{}
		for _, c := range t.PrimaryKey {
			pks[c] = true
		}
		for cname, col := range t.Columns {
			if col.Removed {
				continue
			}
			if pks[cname] || col.Factory == "pk_serial" {
				out = append(out, key+"."+cname)
			}
		}
	}
	sort.Strings(out)
	return out
}

func intValidator(val any) error {
	s, ok := val.(string)
	if !ok {
		return fmt.Errorf("string required")
	}
	if s == "" {
		return fmt.Errorf("value required")
	}
	if _, err := strconv.Atoi(s); err != nil {
		return fmt.Errorf("must be an integer")
	}
	return nil
}

// coerceScalar parses a raw string into the appropriate Go scalar based on the
// declared data_type. YAML round-trip then emits it with the right quoting.
func coerceScalar(raw, dataType string) any {
	dt := strings.ToLower(strings.TrimSpace(dataType))
	intTypes := map[string]bool{
		"integer": true, "bigint": true, "smallint": true,
		"int": true, "int4": true, "int8": true, "int2": true,
	}
	floatTypes := map[string]bool{
		"numeric": true, "decimal": true, "real": true,
		"double precision": true, "float": true, "float4": true, "float8": true,
	}
	boolTypes := map[string]bool{"boolean": true, "bool": true}
	switch {
	case intTypes[dt]:
		if n, err := strconv.ParseInt(strings.TrimSpace(raw), 10, 64); err == nil {
			return n
		}
	case floatTypes[dt]:
		if f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil {
			return f
		}
	case boolTypes[dt]:
		if b, err := strconv.ParseBool(strings.TrimSpace(raw)); err == nil {
			return b
		}
	}
	return raw
}
