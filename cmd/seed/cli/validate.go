package cli

import (
	"fmt"
	"strings"

	"github.com/ivannikolaev/seed-cli/cli/internal/config"
	"github.com/ivannikolaev/seed-cli/cli/internal/registry"
	"github.com/ivannikolaev/seed-cli/cli/internal/relations"
	"github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
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
						fmt.Fprintf(cmd.ErrOrStderr(), "unresolved: %s.%s\n", key, cname)
						issues++
					}
					// Columns with a literal value or a JSON `values:` shape
					// bypass the factory dispatch entirely.
					needsFactory := col.Value == nil && len(col.Values) == 0
					if needsFactory {
						if col.Factory == "" {
							fmt.Fprintf(cmd.ErrOrStderr(), "missing factory: %s.%s\n", key, cname)
							issues++
						} else if _, ok := reg.Get(col.Factory); !ok {
							fmt.Fprintf(cmd.ErrOrStderr(), "unknown factory: %s.%s uses %q\n", key, cname, col.Factory)
							issues++
						}
					}
					if col.Value != nil && col.DataType != "" {
						if err := checkValueType(col.Value, col.DataType); err != nil {
							fmt.Fprintf(cmd.ErrOrStderr(), "value type mismatch: %s.%s: %v\n", key, cname, err)
							issues++
						}
					}
					if col.Factory == seedapi.FactoryFKRef {
						target, _ := col.Params["target"].(string)
						switch {
						case target == "":
							fmt.Fprintf(cmd.ErrOrStderr(), "fkref missing target: %s.%s\n", key, cname)
							issues++
						case !targetExists(cfg, target):
							fmt.Fprintf(cmd.ErrOrStderr(), "fkref target not found: %s.%s → %q\n", key, cname, target)
							issues++
						}
					}
				}
			}
			// row_count_per: check that referenced parent tables exist
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
						fmt.Fprintf(cmd.ErrOrStderr(), "row_count_per: %s references missing parent %s\n", key, parentKey)
						issues++
					}
				}
			}

			g, err := relations.Build(cfg)
			if err != nil {
				return err
			}
			plan := g.PlanFor(cfg)
			// NOT NULL fkref columns pointing at a table with row_count 0 silently
			// emit NULL at generate time, causing a NOT NULL constraint failure at
			// apply time.
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
						fmt.Fprintf(cmd.ErrOrStderr(), "fkref not-null empty pool: %s.%s → %s has row_count 0 — apply will fail\n", key, cname, targetKey)
						issues++
					}
				}
			}
			// NOT NULL fkref inside a deferrable cycle: the first-emitted table draws
			// from an empty pool → fkRef.Generate returns nil → NULL in a NOT NULL column.
			// SET CONSTRAINTS ALL DEFERRED defers FK constraint checking but NOT NULL is
			// always enforced immediately.
			if len(plan.Cycles) > 0 {
				cycleMembers := map[string]map[string]bool{}
				for _, cycle := range plan.Cycles {
					members := map[string]bool{}
					for _, ref := range cycle {
						members[ref.Key()] = true
					}
					for _, ref := range cycle {
						cycleMembers[ref.Key()] = members
					}
				}
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
						if target == "" {
							continue
						}
						if siblings[fkTargetTableKey(target)] {
							fmt.Fprintf(cmd.ErrOrStderr(), "cycle-null: %s.%s is NOT NULL fkref inside a deferrable cycle — first-emitted rows will have NULL, apply will fail\n", key, cname)
							issues++
						}
					}
				}
			}

			// C1: UNIQUE columns with factories that don't guarantee per-row uniqueness.
			for key, t := range cfg.Tables {
				if t.Removed {
					continue
				}
				for _, uk := range t.UniqueKeys {
					if len(uk) != 1 {
						// Composite UNIQUE: can't automatically verify — surface as info.
						fmt.Fprintf(cmd.ErrOrStderr(), "info: %s has composite UNIQUE constraint %v — verify that assigned factories produce distinct tuples\n", key, uk)
						continue
					}
					col := t.Columns[uk[0]]
					if col == nil || col.Removed || col.Value != nil {
						continue
					}
					if !uniqueSafeFactory(col.Factory, reg) {
						fmt.Fprintf(cmd.ErrOrStderr(), "unique-unsafe: %s.%s has UNIQUE constraint but factory %q may produce duplicate values\n", key, uk[0], col.Factory)
						issues++
					}
				}
			}

			// C2: multiple fkref columns in one table targeting the same parent table
			// suggests a composite FK — values are sampled independently, so tuples
			// won't correspond to the same parent row.
			for key, t := range cfg.Tables {
				if t.Removed {
					continue
				}
				// Group FK columns by parent table. Only warn when they target
				// different parent columns — that indicates a composite FK constraint
				// where correlated sampling is required. Two independent FK columns
				// targeting the same parent column (e.g. author_id and reviewer_id
				// both → users.id) are legitimate and should not produce a warning.
				type parentTable struct{ schema, table string }
				type fkMapping struct{ parentCol, childCol string }
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
				for ref, mappings := range fkMappings {
					if len(mappings) <= 1 {
						continue
					}
					parentCols := map[string]bool{}
					for _, m := range mappings {
						parentCols[m.parentCol] = true
					}
					if len(parentCols) > 1 {
						childCols := make([]string, len(mappings))
						for i, m := range mappings {
							childCols[i] = m.childCol
						}
						fmt.Fprintf(cmd.ErrOrStderr(), "composite-fk: %s has fkref columns %v targeting different columns of %s.%s — values sampled independently, tuple consistency not guaranteed\n", key, childCols, ref.schema, ref.table)
						issues++
					}
				}
			}

			if len(plan.Cycles) > 0 {
				for _, c := range plan.Cycles {
					bad := plan.NonDeferrableEdgesIn(c)
					if len(bad) > 0 {
						names := make([]string, len(bad))
						for i, e := range bad {
							names[i] = fmt.Sprintf("%s.%s", e.From.Key(), e.Column)
						}
						fmt.Fprintf(cmd.ErrOrStderr(), "fk cycle %v: non-deferrable edges %v — apply will fail\n", refsKeys(c), names)
						issues++
					} else {
						fmt.Fprintf(cmd.ErrOrStderr(), "fk cycle: %v (DEFERRABLE — ok)\n", refsKeys(c))
					}
				}
			}

			// C3: CHECK constraints that the parser could not interpret. They
			// stay in the schema but the generator has no idea what to satisfy.
			// Single-column parseable forms are already applied to params at
			// build time, so anything left here is either multi-column or an
			// expression shape we don't understand.
			for key, t := range cfg.Tables {
				if t.Removed {
					continue
				}
				for _, chk := range t.Checks {
					if isCheckApplied(t, chk) {
						continue
					}
					fmt.Fprintf(cmd.ErrOrStderr(), "info: %s CHECK %q not applied automatically: %s — verify factory output satisfies it\n", key, chk.Name, chk.Expression)
				}
			}

			// C4: EXCLUDE constraints — semantics vary per usage. Surface as
			// warning; the user should decide whether to tune row_count or
			// provide a correlated generator.
			for key, t := range cfg.Tables {
				if t.Removed {
					continue
				}
				for _, ex := range t.Excludes {
					fmt.Fprintf(cmd.ErrOrStderr(), "warn: %s has EXCLUDE %q: %s — default generation will likely violate it\n", key, ex.Name, ex.Definition)
				}
			}

			// C5: Partial UNIQUE indexes (e.g. soft-delete `WHERE deleted_at
			// IS NULL`) — not enforced by the generator. Surface as info.
			for key, t := range cfg.Tables {
				if t.Removed {
					continue
				}
				for _, p := range t.PartialUniqueKeys {
					fmt.Fprintf(cmd.ErrOrStderr(), "info: %s partial UNIQUE on %v WHERE %s — not enforced by generator\n", key, p.Columns, p.Predicate)
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

// targetExists checks that a "schema.table.column" (or "table.column")
// reference points at a real column in the config. A bad target generates
// valid-looking SQL that picks NULL from an empty pool and then fails on FK
// constraint at apply time.
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

// fkTargetTableKey extracts the "schema.table" key from a "schema.table.column"
// or "table.column" fkref target string.
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

// uniqueSafeFactory returns true for factories that guarantee per-row unique
// output within a single Emit call. Checks the seedapi.UniqueGenerator interface
// first so user plugins can opt in; falls back to a hardcoded set for builtins.
func uniqueSafeFactory(name string, reg *registry.Registry) bool {
	if f, ok := reg.Get(name); ok {
		if ug, ok := f.(seedapi.UniqueGenerator); ok {
			return ug.UniquePerRow()
		}
	}
	return false
}

// checkValueType returns an error if a literal value is obviously incompatible
// with the column's DataType (e.g. a string literal on an integer column).
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

// isCheckApplied returns true when the CHECK constraint is single-column and
// the parsed bound has already been merged into that column's params. We use
// presence of any of (values, min, max, max_len) as the signal — build.go's
// applyCheckConstraints only ever writes those keys for recognized patterns.
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
