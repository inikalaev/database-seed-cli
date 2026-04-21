package cli

import (
	"encoding/json"
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
		if err := cascadeFactorySetup(cfg, reg, issue.Fix.Table, issue.Fix.Column, col); err != nil {
			return fixSkipped, err
		}
		return fixApplied, nil
	}
	col.Factory = labelToName[choice]
	col.Unresolved = false
	if err := cascadeFactorySetup(cfg, reg, issue.Fix.Table, issue.Fix.Column, col); err != nil {
		return fixSkipped, err
	}
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

// paramStepResult tells cascadeFactorySetup what happened inside a single
// prompt so the outer loop can move on, restart, or bail out.
type paramStepResult int

const (
	paramStepApplied paramStepResult = iota
	paramStepSkipped
	paramStepFactoryChanged
	paramStepAborted
)

const optChangeFactory = "change factory instead"

// cascadeFactorySetup repeatedly asks the current factory for its required
// setup steps and prompts for each. If the user switches factory mid-prompt
// the loop restarts with the new factory's requirements. A hard iteration
// cap guards against misbehaving factories that don't shrink their required
// step set after the CLI writes the param.
//
// Reentrancy: `change factory instead` inside flowSetFactoryParam calls
// flowChooseFactory, which itself invokes cascadeFactorySetup for the newly
// picked factory. When control returns here, the outer loop runs
// RequiredSetup one more time — a no-op if the nested cascade already filled
// everything. Anyone removing the inner call from flowChooseFactory should
// keep this outer pass as the safety net.
func cascadeFactorySetup(cfg *config.Config, reg *registry.Registry, tableKey, colName string, col *config.ColumnSpec) error {
	const maxIters = 16
	for i := 0; i < maxIters; i++ {
		f, ok := reg.Get(col.Factory)
		if !ok {
			return nil
		}
		conf, ok := f.(seedapi.Configurable)
		if !ok {
			return nil
		}
		steps := conf.RequiredSetup(col.Params)
		if len(steps) == 0 {
			return nil
		}
		restart := false
		abort := false
		for _, step := range steps {
			result, err := flowSetFactoryParam(cfg, reg, tableKey, colName, col, step)
			if err != nil {
				return err
			}
			switch result {
			case paramStepFactoryChanged:
				restart = true
			case paramStepAborted:
				abort = true
			}
			if restart || abort {
				break
			}
		}
		if abort {
			return nil
		}
		if !restart {
			return nil
		}
	}
	return nil
}

// flowSetFactoryParam prompts the user for one SetupStep. Offers three top
// choices: enter value, change factory, or skip (only when not required).
func flowSetFactoryParam(cfg *config.Config, reg *registry.Registry, tableKey, colName string, col *config.ColumnSpec, step seedapi.SetupStep) (paramStepResult, error) {
	const optEnter = "enter value"
	options := []string{optEnter, optChangeFactory}
	if !step.Required {
		options = append(options, optSkip)
	}
	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: step.Prompt,
		Options: options,
		Default: optEnter,
	}, &choice); err != nil {
		return paramStepAborted, err
	}
	switch choice {
	case optSkip:
		return paramStepSkipped, nil
	case optChangeFactory:
		issue := validate.Issue{Fix: &validate.FixSpec{Table: tableKey, Column: colName}}
		if _, err := flowChooseFactory(cfg, reg, issue); err != nil {
			return paramStepAborted, err
		}
		return paramStepFactoryChanged, nil
	}
	val, err := promptScalarOrList(step)
	if err != nil {
		return paramStepAborted, err
	}
	if col.Params == nil {
		col.Params = map[string]any{}
	}
	col.Params[step.ParamKey] = val
	return paramStepApplied, nil
}

// promptScalarOrList dispatches to the right survey prompt for the step kind
// and returns the parsed value ready to write into col.Params.
func promptScalarOrList(step seedapi.SetupStep) (any, error) {
	switch step.Kind {
	case seedapi.SetupString:
		var raw string
		if err := survey.AskOne(&survey.Input{Message: step.Prompt, Help: step.Help}, &raw,
			survey.WithValidator(survey.Required)); err != nil {
			return nil, err
		}
		return strings.TrimSpace(raw), nil
	case seedapi.SetupInt:
		var raw string
		if err := survey.AskOne(&survey.Input{Message: step.Prompt, Help: step.Help}, &raw,
			survey.WithValidator(intValidator)); err != nil {
			return nil, err
		}
		n, _ := strconv.Atoi(strings.TrimSpace(raw))
		return n, nil
	case seedapi.SetupFloat:
		var raw string
		if err := survey.AskOne(&survey.Input{Message: step.Prompt, Help: step.Help}, &raw,
			survey.WithValidator(floatValidator)); err != nil {
			return nil, err
		}
		f, _ := strconv.ParseFloat(strings.TrimSpace(raw), 64)
		return f, nil
	case seedapi.SetupBool:
		var choice string
		if err := survey.AskOne(&survey.Select{Message: step.Prompt, Options: []string{"true", "false"}}, &choice); err != nil {
			return nil, err
		}
		return choice == "true", nil
	case seedapi.SetupList:
		if step.Element == nil {
			return nil, fmt.Errorf("list step %q has nil Element", step.ParamKey)
		}
		var raw string
		if err := survey.AskOne(&survey.Input{Message: step.Prompt, Help: step.Help}, &raw,
			survey.WithValidator(survey.Required)); err != nil {
			return nil, err
		}
		return parseListInput(raw, step.Element.Kind)
	}
	return nil, fmt.Errorf("unsupported SetupKind %q", step.Kind)
}

// parseListInput splits a CSV string and parses each element by the declared
// element kind. Empty elements are dropped; an empty result is an error.
func parseListInput(raw string, elem seedapi.SetupKind) ([]any, error) {
	parts := strings.Split(raw, ",")
	out := make([]any, 0, len(parts))
	for i, p := range parts {
		s := strings.TrimSpace(p)
		if s == "" {
			continue
		}
		switch elem {
		case seedapi.SetupString:
			out = append(out, s)
		case seedapi.SetupInt:
			n, err := strconv.Atoi(s)
			if err != nil {
				return nil, fmt.Errorf("element %d: %w", i+1, err)
			}
			out = append(out, n)
		case seedapi.SetupFloat:
			f, err := strconv.ParseFloat(s, 64)
			if err != nil {
				return nil, fmt.Errorf("element %d: %w", i+1, err)
			}
			out = append(out, f)
		case seedapi.SetupBool:
			b, err := strconv.ParseBool(s)
			if err != nil {
				return nil, fmt.Errorf("element %d: %w", i+1, err)
			}
			out = append(out, b)
		default:
			return nil, fmt.Errorf("unsupported list element kind %q", elem)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one value required")
	}
	return out, nil
}

func floatValidator(val any) error {
	s, ok := val.(string)
	if !ok {
		return fmt.Errorf("string required")
	}
	if _, err := strconv.ParseFloat(strings.TrimSpace(s), 64); err != nil {
		return fmt.Errorf("must be a number")
	}
	return nil
}

// flowFactoryParam handles KindMissingFactoryParam issues discovered by
// validate. It routes through the same cascade used post-selection so any
// additional requirements are captured in one go.
func flowFactoryParam(cfg *config.Config, reg *registry.Registry, issue validate.Issue) (fixResult, error) {
	col, err := lookupColumn(cfg, issue.Fix.Table, issue.Fix.Column)
	if err != nil {
		return fixSkipped, err
	}
	// JSON subfield: cascade on the field spec, not the column spec.
	if issue.Fix.Field != "" {
		fieldSpec, ok := col.Values[issue.Fix.Field]
		if !ok || fieldSpec == nil {
			return fixSkipped, fmt.Errorf("json field %q not found", issue.Fix.Field)
		}
		factoryBefore := fieldSpec.Factory
		paramsBefore := len(fieldSpec.Params)
		if err := cascadeFactorySetup(cfg, reg, issue.Fix.Table, issue.Fix.Column, fieldSpec); err != nil {
			return fixSkipped, err
		}
		if fieldSpec.Factory != factoryBefore || len(fieldSpec.Params) > paramsBefore {
			return fixApplied, nil
		}
		return fixSkipped, nil
	}
	factoryBefore := col.Factory
	paramsBefore := len(col.Params)
	if err := cascadeFactorySetup(cfg, reg, issue.Fix.Table, issue.Fix.Column, col); err != nil {
		return fixSkipped, err
	}
	if col.Factory != factoryBefore || len(col.Params) > paramsBefore {
		return fixApplied, nil
	}
	return fixSkipped, nil
}

// flowSetJsonShape handles KindUnresolved for jsonb columns. Asks the user for
// an example JSON object, infers a factory for each field, prints the resulting
// schema, then immediately prompts for any fields it couldn't classify
// confidently — all in a single interaction before saving.
func flowSetJsonShape(cfg *config.Config, reg *registry.Registry, issue validate.Issue) (fixResult, error) {
	col, err := lookupColumn(cfg, issue.Fix.Table, issue.Fix.Column)
	if err != nil {
		return fixSkipped, err
	}

	var raw string
	if err := survey.AskOne(&survey.Input{
		Message: fmt.Sprintf("Example JSON for %s.%s:", issue.Fix.Table, issue.Fix.Column),
		Help:    `paste a representative object, e.g. {"name":"Alice","score":42}`,
	}, &raw, survey.WithValidator(survey.Required)); err != nil {
		return fixSkipped, err
	}

	var example map[string]any
	if err := json.Unmarshal([]byte(raw), &example); err != nil {
		fmt.Printf("invalid JSON: %v\n", err)
		return fixSkipped, nil
	}
	if len(example) == 0 {
		return fixSkipped, nil
	}

	// Infer a factory for every field.
	values := make(map[string]*config.ColumnSpec, len(example))
	for fieldName, val := range example {
		dt := jsonValueDataType(val)
		syntheticCol := seedapi.Column{Name: fieldName, DataType: dt}
		ranked := rankFactories(reg, syntheticCol)

		spec := &config.ColumnSpec{DataType: dt}
		if len(ranked) > 0 && ranked[0].score >= seedapi.WeakNameMatch {
			spec.Factory = ranked[0].name
		} else if len(ranked) > 0 {
			spec.Factory = ranked[0].name
			spec.Unresolved = true
		} else {
			spec.Unresolved = true
		}
		values[fieldName] = spec
	}

	// Print inferred schema sorted for readability.
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	fmt.Println()
	for _, k := range keys {
		spec := values[k]
		if spec.Unresolved {
			colorWarn.Printf("  ? %-18s  %s  (unresolved)\n", k, spec.Factory)
		} else {
			colorOK.Printf("  ✓ %-18s  ", k)
			fmt.Println(spec.Factory)
		}
	}
	fmt.Println()

	// Persist the inferred shape before touching per-field prompts. If the
	// user Ctrl+C's mid-cascade below, the outer fix loop breaks without
	// saving this iteration — so we'd normally lose everything. By committing
	// col.Values now and swallowing cascade interruptions, progress sticks:
	// next validate re-flags only the still-unresolved subfields.
	col.Values = values
	col.Factory = ""
	col.Unresolved = false

	// For each unresolved field, prompt inline — no need for a second fix pass.
subfieldLoop:
	for _, k := range keys {
		spec := values[k]
		if !spec.Unresolved {
			continue
		}
		syntheticCol := seedapi.Column{Name: k, DataType: spec.DataType}
		ranked := rankFactories(reg, syntheticCol)
		top := ranked
		if len(top) > 8 {
			top = top[:8]
		}

		options := make([]string, 0, len(top)+2)
		labelToName := map[string]string{}
		for _, rf := range top {
			label := fmt.Sprintf("%s  (score %d)", rf.name, rf.score)
			if rf.name == spec.Factory {
				label += "  [current]"
			}
			options = append(options, label)
			labelToName[label] = rf.name
		}
		options = append(options, optManualEntry, optSkip)

		var choice string
		if err := survey.AskOne(&survey.Select{
			Message: fmt.Sprintf("Factory for json field %q:", k),
			Options: options,
			Default: options[0],
		}, &choice); err != nil {
			// Stop touching remaining fields but keep the shape we built.
			break subfieldLoop
		}

		switch choice {
		case optSkip:
			// leave as unresolved — will appear in next validate
		case optManualEntry:
			var name string
			if err := survey.AskOne(&survey.Input{Message: "Factory name:"}, &name,
				survey.WithValidator(survey.Required)); err != nil {
				break subfieldLoop
			}
			spec.Factory = strings.TrimSpace(name)
			spec.Unresolved = false
			if err := cascadeFactorySetup(cfg, reg, issue.Fix.Table, issue.Fix.Column, spec); err != nil {
				break subfieldLoop
			}
		default:
			spec.Factory = labelToName[choice]
			spec.Unresolved = false
			if err := cascadeFactorySetup(cfg, reg, issue.Fix.Table, issue.Fix.Column, spec); err != nil {
				break subfieldLoop
			}
		}
	}

	return fixApplied, nil
}

// flowChooseJsonFieldFactory handles KindJsonFieldUnresolved: picks a factory
// for a single field inside a jsonb column's Values map.
func flowChooseJsonFieldFactory(cfg *config.Config, reg *registry.Registry, issue validate.Issue) (fixResult, error) {
	col, err := lookupColumn(cfg, issue.Fix.Table, issue.Fix.Column)
	if err != nil {
		return fixSkipped, err
	}
	fieldSpec, ok := col.Values[issue.Fix.Field]
	if !ok {
		return fixSkipped, fmt.Errorf("json field %q not found in %s.%s", issue.Fix.Field, issue.Fix.Table, issue.Fix.Column)
	}

	syntheticCol := seedapi.Column{Name: issue.Fix.Field, DataType: fieldSpec.DataType}
	ranked := rankFactories(reg, syntheticCol)
	top := ranked
	if len(top) > 8 {
		top = top[:8]
	}

	options := make([]string, 0, len(top)+2)
	labelToName := map[string]string{}
	for _, rf := range top {
		label := fmt.Sprintf("%s  (score %d)", rf.name, rf.score)
		if rf.name == fieldSpec.Factory {
			label += "  [current]"
		}
		options = append(options, label)
		labelToName[label] = rf.name
	}
	options = append(options, optManualEntry, optSkip)

	var choice string
	if err := survey.AskOne(&survey.Select{
		Message: fmt.Sprintf("Factory for json field %q:", issue.Fix.Field),
		Options: options,
		Default: options[0],
	}, &choice); err != nil {
		return fixSkipped, err
	}

	switch choice {
	case optSkip:
		return fixSkipped, nil
	case optManualEntry:
		var name string
		if err := survey.AskOne(&survey.Input{Message: "Factory name:"}, &name,
			survey.WithValidator(survey.Required)); err != nil {
			return fixSkipped, err
		}
		fieldSpec.Factory = strings.TrimSpace(name)
		fieldSpec.Unresolved = false
		if err := cascadeFactorySetup(cfg, reg, issue.Fix.Table, issue.Fix.Column, fieldSpec); err != nil {
			return fixSkipped, err
		}
		return fixApplied, nil
	}
	fieldSpec.Factory = labelToName[choice]
	fieldSpec.Unresolved = false
	if err := cascadeFactorySetup(cfg, reg, issue.Fix.Table, issue.Fix.Column, fieldSpec); err != nil {
		return fixSkipped, err
	}
	return fixApplied, nil
}

// jsonValueDataType maps a Go value decoded from JSON to a PostgreSQL-style
// data type string used for factory inference.
func jsonValueDataType(v any) string {
	switch v.(type) {
	case string:
		return "text"
	case float64:
		return "numeric"
	case bool:
		return "boolean"
	case map[string]any:
		return "jsonb"
	default:
		return "text"
	}
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
