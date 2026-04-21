package validate

import (
	"fmt"
	"strings"

	"github.com/inikalaev/database-seed-cli/internal/config"
	"github.com/inikalaev/database-seed-cli/internal/registry"
	"github.com/inikalaev/database-seed-cli/internal/relations"
	"github.com/inikalaev/database-seed-cli/pkg/seedapi"
)

// Check runs every lint the `validate` command cares about and returns an
// ordered list of findings. Issues are emitted in discovery order — the caller
// (validate/fix) decides on presentation order.
//
// An error is returned only when the relation graph itself cannot be built
// (malformed FK metadata); every other problem is expressed as an Issue.
func Check(cfg *config.Config, reg *registry.Registry) ([]Issue, error) {
	var out []Issue
	out = append(out, checkColumns(cfg, reg)...)
	out = append(out, checkRowCountPer(cfg)...)

	g, err := relations.Build(cfg)
	if err != nil {
		return out, err
	}
	plan := g.PlanFor(cfg)

	out = append(out, checkFKEmptyPool(cfg, plan)...)
	out = append(out, checkFKInCycle(cfg, plan)...)
	out = append(out, checkUniqueKeys(cfg, reg)...)
	out = append(out, checkCompositeFKs(cfg)...)
	out = append(out, checkCycles(plan)...)
	out = append(out, checkConstraints(cfg)...)
	out = append(out, checkExcludes(cfg)...)
	out = append(out, checkPartialUniques(cfg)...)
	return out, nil
}

func checkColumns(cfg *config.Config, reg *registry.Registry) []Issue {
	var out []Issue
	for key, t := range cfg.Tables {
		if t.Removed {
			continue
		}
		for cname, col := range t.Columns {
			if col.Removed {
				continue
			}
			out = append(out, checkColumn(cfg, key, cname, col, reg)...)
		}
	}
	return out
}

func checkColumn(cfg *config.Config, tableKey, cname string, col *config.ColumnSpec, reg *registry.Registry) []Issue {
	var out []Issue
	loc := tableKey + "." + cname

	if col.Unresolved {
		out = append(out, Issue{
			Level: LevelWarn, Kind: KindUnresolved, Location: loc,
			Message: "unresolved",
			Hint:    "pick a factory with `seed-cli fix` or set `factory:` manually",
			Fix:     &FixSpec{Kind: KindUnresolved, Table: tableKey, Column: cname},
		})
	}

	if col.Value == nil && len(col.Values) == 0 {
		out = append(out, checkColumnFactory(loc, tableKey, cname, col, reg)...)
	}

	if col.Value != nil && col.DataType != "" {
		if err := checkValueType(col.Value, col.DataType); err != nil {
			out = append(out, Issue{
				Level: LevelErr, Kind: KindValueTypeMismatch, Location: loc,
				Message: fmt.Sprintf("value type mismatch: %v", err),
				Hint:    "use a literal of the right type or drop `value:`",
				Fix:     &FixSpec{Kind: KindValueTypeMismatch, Table: tableKey, Column: cname},
			})
		}
	}

	if col.Factory == seedapi.FactoryFKRef {
		out = append(out, checkFKRefTarget(cfg, loc, tableKey, cname, col)...)
	}

	return out
}

func checkColumnFactory(loc, tableKey, cname string, col *config.ColumnSpec, reg *registry.Registry) []Issue {
	if col.Factory == "" {
		return []Issue{{
			Level: LevelErr, Kind: KindNoFactory, Location: loc,
			Message: "no factory",
			Hint:    "add `factory:` or set a literal `value:`",
			Fix:     &FixSpec{Kind: KindNoFactory, Table: tableKey, Column: cname},
		}}
	}
	if _, ok := reg.Get(col.Factory); !ok {
		return []Issue{{
			Level: LevelErr, Kind: KindUnknownFactory, Location: loc,
			Message: fmt.Sprintf("unknown factory %q", col.Factory),
			Hint:    "typo? pick a real factory, or keep if it's a user plugin",
			Fix: &FixSpec{
				Kind: KindUnknownFactory, Table: tableKey, Column: cname,
				Ctx: map[string]any{"current": col.Factory},
			},
		}}
	}
	return nil
}

func checkFKRefTarget(cfg *config.Config, loc, tableKey, cname string, col *config.ColumnSpec) []Issue {
	target, _ := col.Params["target"].(string)
	switch {
	case target == "":
		return []Issue{{
			Level: LevelErr, Kind: KindFKRefMissingTarget, Location: loc,
			Message: "fkref: missing target",
			Hint:    "add `params: {target: schema.table.column}`",
			Fix:     &FixSpec{Kind: KindFKRefMissingTarget, Table: tableKey, Column: cname},
		}}
	case !targetExists(cfg, target):
		return []Issue{{
			Level: LevelErr, Kind: KindFKRefTargetNotFound, Location: loc,
			Message: fmt.Sprintf("fkref: target %q not found", target),
			Hint:    "pick an existing PK column",
			Fix: &FixSpec{
				Kind: KindFKRefTargetNotFound, Table: tableKey, Column: cname,
				Ctx: map[string]any{"current": target},
			},
		}}
	}
	return nil
}

func checkRowCountPer(cfg *config.Config) []Issue {
	var out []Issue
	for key, t := range cfg.Tables {
		if t.Removed {
			continue
		}
		for parent := range t.RowCountPer {
			parentKey := parent
			if !strings.Contains(parentKey, ".") {
				parentKey = "public." + parent
			}
			pt, exists := cfg.Tables[parentKey]
			if !exists || pt.Removed {
				out = append(out, Issue{
					Level: LevelErr, Kind: KindRowCountPerMissing, Location: key,
					Message: fmt.Sprintf("row_count_per: parent %q missing", parentKey),
					Hint:    "fix the key or remove the entry",
					Fix: &FixSpec{
						Kind: KindRowCountPerMissing, Table: key,
						Ctx: map[string]any{"parent": parent, "parentKey": parentKey},
					},
				})
			}
		}
	}
	return out
}

func checkFKEmptyPool(cfg *config.Config, plan *relations.Plan) []Issue {
	var out []Issue
	for key, t := range cfg.Tables {
		if t.Removed {
			continue
		}
		for cname, col := range t.Columns {
			if col.Removed || col.Factory != seedapi.FactoryFKRef || col.Nullable {
				continue
			}
			target, _ := col.Params["target"].(string)
			if target == "" {
				continue
			}
			targetKey := fkTargetTableKey(target)
			if plan.RowCounts[targetKey] == 0 {
				out = append(out, Issue{
					Level: LevelErr, Kind: KindFKRefEmptyPool,
					Location: key + "." + cname,
					Message:  fmt.Sprintf("fkref NOT NULL → %s has row_count 0", targetKey),
					Hint:     "raise parent row_count, mark column nullable, or set `value:`",
					Fix: &FixSpec{
						Kind: KindFKRefEmptyPool, Table: key, Column: cname,
						Ctx: map[string]any{"parentKey": targetKey},
					},
				})
			}
		}
	}
	return out
}

func checkFKInCycle(cfg *config.Config, plan *relations.Plan) []Issue {
	if len(plan.Cycles) == 0 {
		return nil
	}
	cycleMembers := buildCycleMembers(plan.Cycles)

	var out []Issue
	for key, t := range cfg.Tables {
		if t.Removed {
			continue
		}
		siblings := cycleMembers[key]
		if len(siblings) == 0 {
			continue
		}
		for cname, col := range t.Columns {
			if col.Removed || col.Factory != seedapi.FactoryFKRef || col.Nullable {
				continue
			}
			target, _ := col.Params["target"].(string)
			if target == "" || !siblings[fkTargetTableKey(target)] {
				continue
			}
			out = append(out, Issue{
				Level: LevelErr, Kind: KindFKRefInCycle,
				Location: key + "." + cname,
				Message:  "NOT NULL fkref in FK cycle — NULL on first emit",
				Hint:     "mark nullable or set `value:` to break the first-emit NULL",
				Fix:      &FixSpec{Kind: KindFKRefInCycle, Table: key, Column: cname},
			})
		}
	}
	return out
}

func buildCycleMembers(cycles [][]relations.TableRef) map[string]map[string]bool {
	result := map[string]map[string]bool{}
	for _, cycle := range cycles {
		members := map[string]bool{}
		for _, ref := range cycle {
			members[ref.Key()] = true
		}
		for _, ref := range cycle {
			result[ref.Key()] = members
		}
	}
	return result
}

func checkUniqueKeys(cfg *config.Config, reg *registry.Registry) []Issue {
	var out []Issue
	for key, t := range cfg.Tables {
		if t.Removed {
			continue
		}
		for _, uk := range t.UniqueKeys {
			if len(uk) != 1 {
				out = append(out, Issue{
					Level: LevelInfo, Kind: KindCompositeUnique, Location: key,
					Message: fmt.Sprintf("composite UNIQUE %v — verify manually", uk),
					Hint:    "ensure factory combinations produce distinct tuples",
				})
				continue
			}
			col := t.Columns[uk[0]]
			if col == nil || col.Removed || col.Value != nil {
				continue
			}
			if !uniqueSafeFactory(col.Factory, reg) {
				out = append(out, Issue{
					Level: LevelWarn, Kind: KindUniqueUnsafeFactory,
					Location: key + "." + uk[0],
					Message:  fmt.Sprintf("UNIQUE + unsafe factory %q", col.Factory),
					Hint:     "switch to uuid/pk_serial/token, or accept collision risk",
					Fix: &FixSpec{
						Kind: KindUniqueUnsafeFactory, Table: key, Column: uk[0],
						Ctx: map[string]any{"current": col.Factory},
					},
				})
			}
		}
	}
	return out
}

type parentTable struct{ schema, table string }
type fkMapping struct{ parentCol, childCol string }

func checkCompositeFKs(cfg *config.Config) []Issue {
	var out []Issue
	for key, t := range cfg.Tables {
		if t.Removed {
			continue
		}
		out = append(out, checkTableCompositeFKs(key, t)...)
	}
	return out
}

func checkTableCompositeFKs(tableKey string, t *config.Table) []Issue {
	fkMappings := map[parentTable][]fkMapping{}
	for cname, col := range t.Columns {
		if col.Removed || col.Factory != seedapi.FactoryFKRef {
			continue
		}
		target, _ := col.Params["target"].(string)
		if target == "" {
			continue
		}
		parts := strings.Split(target, ".")
		var ps, pt, pc string
		switch len(parts) {
		case 3:
			ps, pt, pc = parts[0], parts[1], parts[2]
		case 2:
			ps, pt, pc = "public", parts[0], parts[1]
		default:
			continue
		}
		ref := parentTable{ps, pt}
		fkMappings[ref] = append(fkMappings[ref], fkMapping{pc, cname})
	}

	var out []Issue
	for ref, mappings := range fkMappings {
		if len(mappings) <= 1 {
			continue
		}
		parentCols := map[string]bool{}
		for _, m := range mappings {
			parentCols[m.parentCol] = true
		}
		if len(parentCols) <= 1 {
			continue
		}
		childCols := make([]string, len(mappings))
		for i, m := range mappings {
			childCols[i] = m.childCol
		}
		out = append(out, Issue{
			Level: LevelWarn, Kind: KindCompositeFK, Location: tableKey,
			Message: fmt.Sprintf("composite FK %v → %s.%s (independent sampling)", childCols, ref.schema, ref.table),
			Hint:    "write a correlated custom generator if tuple consistency matters",
		})
	}
	return out
}

func checkCycles(plan *relations.Plan) []Issue {
	if len(plan.Cycles) == 0 {
		return nil
	}
	var out []Issue
	for _, c := range plan.Cycles {
		if bad := plan.NonDeferrableEdgesIn(c); len(bad) > 0 {
			names := make([]string, len(bad))
			for i, e := range bad {
				names[i] = e.From.Key() + "." + e.Column
			}
			out = append(out, Issue{
				Level: LevelErr, Kind: KindNonDeferrableCycle, Location: "fk cycle",
				Message: fmt.Sprintf("non-deferrable edges %v — apply will fail", names),
				Hint:    "ALTER TABLE ... INITIALLY DEFERRED, or make an edge nullable in the DB",
			})
		} else {
			out = append(out, Issue{
				Level: LevelInfo, Kind: KindDeferrableCycle, Location: "fk cycle",
				Message: fmt.Sprintf("%v (DEFERRABLE)", refsKeys(c)),
				Hint:    "ok — generator emits SET CONSTRAINTS ALL DEFERRED",
			})
		}
	}
	return out
}

func checkConstraints(cfg *config.Config) []Issue {
	var out []Issue
	for key, t := range cfg.Tables {
		if t.Removed {
			continue
		}
		for _, chk := range t.Checks {
			if isCheckApplied(t, chk) {
				continue
			}
			out = append(out, Issue{
				Level: LevelInfo, Kind: KindCheckNotApplied, Location: key,
				Message: fmt.Sprintf("CHECK %q not applied: %s", chk.Name, chk.Expression),
				Hint:    "tune factory params (min/max/values) so output satisfies the check",
			})
		}
	}
	return out
}

func checkExcludes(cfg *config.Config) []Issue {
	var out []Issue
	for key, t := range cfg.Tables {
		if t.Removed {
			continue
		}
		for _, ex := range t.Excludes {
			out = append(out, Issue{
				Level: LevelWarn, Kind: KindExclude, Location: key,
				Message: fmt.Sprintf("EXCLUDE %q: %s", ex.Name, ex.Definition),
				Hint:    "default generation will likely violate — set row_count: 0 or write a correlated generator",
			})
		}
	}
	return out
}

func checkPartialUniques(cfg *config.Config) []Issue {
	var out []Issue
	for key, t := range cfg.Tables {
		if t.Removed {
			continue
		}
		for _, p := range t.PartialUniqueKeys {
			out = append(out, Issue{
				Level: LevelInfo, Kind: KindPartialUnique, Location: key,
				Message: fmt.Sprintf("partial UNIQUE %v WHERE %s", p.Columns, p.Predicate),
				Hint:    "generator does not enforce the WHERE clause — verify manually",
			})
		}
	}
	return out
}

// Counts returns the number of issues per level for stats rendering.
func Counts(issues []Issue) (errs, warns, infos int) {
	for _, i := range issues {
		switch i.Level {
		case LevelErr:
			errs++
		case LevelWarn:
			warns++
		case LevelInfo:
			infos++
		}
	}
	return
}

// HasErrors returns true if any issue is at ERR level.
func HasErrors(issues []Issue) bool {
	for _, i := range issues {
		if i.Level == LevelErr {
			return true
		}
	}
	return false
}

// HasFixable returns true if any issue has an auto-fix flow.
func HasFixable(issues []Issue) bool {
	for _, i := range issues {
		if i.Fix != nil {
			return true
		}
	}
	return false
}

// targetExists checks that an "schema.table.column" (or "table.column")
// reference points at a real column in the config.
func targetExists(cfg *config.Config, target string) bool {
	parts := strings.Split(target, ".")
	var s, t, c string
	switch len(parts) {
	case 3:
		s, t, c = parts[0], parts[1], parts[2]
	case 2:
		s, t, c = "public", parts[0], parts[1]
	default:
		return false
	}
	tbl, ok := cfg.Tables[s+"."+t]
	if !ok || tbl.Removed {
		return false
	}
	col, ok := tbl.Columns[c]
	if !ok || col.Removed {
		return false
	}
	return true
}

func fkTargetTableKey(target string) string {
	parts := strings.Split(target, ".")
	switch len(parts) {
	case 3:
		return parts[0] + "." + parts[1]
	case 2:
		return "public." + parts[0]
	default:
		return target
	}
}

// UniqueSafeFactory reports whether a factory guarantees per-row uniqueness
// via the UniqueGenerator interface. Exported for use by the fix flow.
func UniqueSafeFactory(name string, reg *registry.Registry) bool {
	return uniqueSafeFactory(name, reg)
}

func uniqueSafeFactory(name string, reg *registry.Registry) bool {
	if f, ok := reg.Get(name); ok {
		if ug, ok := f.(seedapi.UniqueGenerator); ok {
			return ug.UniquePerRow()
		}
	}
	return false
}

func checkValueType(v any, dataType string) error {
	intTypes := map[string]bool{
		"integer": true, "bigint": true, "smallint": true,
		"int": true, "int4": true, "int8": true, "int2": true,
	}
	boolTypes := map[string]bool{"boolean": true, "bool": true}
	dt := strings.ToLower(dataType)
	switch v.(type) {
	case string:
		if intTypes[dt] || boolTypes[dt] {
			return fmt.Errorf("string value %q assigned to %s column", v, dataType)
		}
	case bool:
		if intTypes[dt] {
			return fmt.Errorf("bool value assigned to %s column", dataType)
		}
	}
	return nil
}

func isCheckApplied(t *config.Table, chk config.CheckConstraint) bool {
	if len(chk.Columns) != 1 {
		return false
	}
	col := t.Columns[chk.Columns[0]]
	if col == nil || col.Removed {
		return false
	}
	for _, k := range []string{"values", "min", "max", "max_len"} {
		if _, ok := col.Params[k]; ok {
			return true
		}
	}
	return false
}

func refsKeys(refs []relations.TableRef) []string {
	out := make([]string, len(refs))
	for i, r := range refs {
		out[i] = r.Key()
	}
	return out
}
