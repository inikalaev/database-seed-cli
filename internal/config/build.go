package config

import (
	"github.com/inikalaev/database-seed-cli/internal/registry"
	"github.com/inikalaev/database-seed-cli/internal/schema"
)

const defaultRowCount = 100

// FromModel builds a fresh Config from an introspected schema model using the
// inference registry. Every column gets an Origin="inferred" spec; Unresolved
// is set when the registry could not find a strong match.
func FromModel(model *schema.Model, reg *registry.Registry, defaults DefaultsSection) *Config {
	cfg := &Config{
		Version:  CurrentVersion,
		Database: DatabaseSection{Dialect: model.Dialect, Schemas: model.Schemas},
		Defaults: defaults,
		Tables:   map[string]*Table{},
	}
	enumValues := map[string][]string{}
	for _, e := range model.Enums {
		enumValues[e.Schema+"."+e.Name] = e.Values
	}
	for _, tbl := range model.Tables {
		key := QualifiedKey(tbl.Schema, tbl.Name)
		rc := defaultRowCount
		// Trigger-populated tables fill themselves as a side effect of their
		// source tables' INSERTs, so emitting rows independently would
		// collide on PK. Start at 0 — user can override.
		if tbl.TriggerPopulated {
			rc = 0
		}
		// EXCLUDE constraints are range/GIST semantic checks (overlap
		// prevention on tsrange, numrange, …). Independent random fill for
		// the constrained columns violates them deterministically, and the
		// expressions `tsrange(a,b)` in the index itself throws when a > b.
		// Default to 0 — user can write a custom factory pair and override.
		if len(tbl.ExcludeConstraints) > 0 {
			rc = 0
		}
		uks := make([][]string, len(tbl.UniqueKeys))
		for i, uk := range tbl.UniqueKeys {
			uks[i] = append([]string(nil), uk...)
		}
		puks := make([]PartialUniqueKey, len(tbl.PartialUniqueKeys))
		for i, p := range tbl.PartialUniqueKeys {
			puks[i] = PartialUniqueKey{
				Columns:   append([]string(nil), p.Columns...),
				Predicate: p.Predicate,
			}
		}
		checks := make([]CheckConstraint, len(tbl.CheckConstraints))
		for i, c := range tbl.CheckConstraints {
			checks[i] = CheckConstraint{
				Name:       c.Name,
				Expression: c.Expression,
				Columns:    append([]string(nil), c.Columns...),
			}
		}
		excludes := make([]ExcludeConstraint, len(tbl.ExcludeConstraints))
		for i, e := range tbl.ExcludeConstraints {
			excludes[i] = ExcludeConstraint{
				Name:       e.Name,
				Definition: e.Definition,
				Columns:    append([]string(nil), e.Columns...),
			}
		}
		polys := make([]PolymorphicKey, len(tbl.Polymorphs))
		for i, p := range tbl.Polymorphs {
			polys[i] = PolymorphicKey{TypeColumn: p.TypeColumn, IdColumn: p.IdColumn}
		}
		ct := &Table{
			Schema:            tbl.Schema,
			Name:              tbl.Name,
			RowCount:          &rc,
			Columns:           map[string]*ColumnSpec{},
			ColumnOrder:       columnOrder(tbl),
			PrimaryKey:        append([]string(nil), tbl.PrimaryKey...),
			UniqueKeys:        uks,
			PartialUniqueKeys: puks,
			Checks:            checks,
			Excludes:          excludes,
			TriggerPopulated:  tbl.TriggerPopulated,
			Polymorphs:        polys,
		}
		fkByCol, fkDeferrable := indexFK(tbl)
		compositeFK := compositeFKColumns(tbl)
		for _, col := range tbl.Columns {
			if col.IsGenerated {
				continue
			}
			apiCol := registry.ToAPIColumn(tbl, col, enumValues[col.EnumName], fkByCol[col.Name])
			res := reg.Infer(apiCol, defaults.Locale)
			spec := &ColumnSpec{
				Nullable: col.Nullable,
				DataType: col.DataType,
			}
			if res.Factory != nil {
				spec.Factory = res.Factory.Name()
			} else {
				spec.Factory = "string"
			}
			spec.Unresolved = res.Unresolved
			if apiCol.FKTarget != "" {
				spec.Params = map[string]any{"target": apiCol.FKTarget}
				if fkDeferrable[col.Name] {
					spec.Params["deferrable"] = true
				}
			}
			// Columns participating in composite FKs can't be auto-routed to a
			// single target. Flag them so the user sees they need manual wiring.
			if compositeFK[col.Name] {
				spec.Unresolved = true
			}
			// Enum labels live in the schema, not in YAML. Stash them in params
			// so enum_value.Generate can sample them at emit time without a
			// second round-trip through introspection.
			if len(apiCol.EnumValues) > 0 {
				if spec.Params == nil {
					spec.Params = map[string]any{}
				}
				vals := make([]any, len(apiCol.EnumValues))
				for i, v := range apiCol.EnumValues {
					vals[i] = v
				}
				spec.Params["values"] = vals
			}
			// Array element type is schema-derived; stash it so the `array`
			// factory can pick the right element generator at emit time
			// (UDTName isn't carried in ColumnSpec).
			if col.DataType == "ARRAY" && col.UDTName != "" {
				if spec.Params == nil {
					spec.Params = map[string]any{}
				}
				spec.Params["element"] = col.UDTName
			}
			// CharMaxLen is schema-derived: for `varchar(N)/char(N)` columns
			// we stash it so sqlemit can truncate generator output to fit,
			// preventing "value too long for type character varying(N)" errors.
			if col.CharMaxLen != nil && *col.CharMaxLen > 0 {
				if spec.Params == nil {
					spec.Params = map[string]any{}
				}
				spec.Params["max_len"] = *col.CharMaxLen
			}
			ct.Columns[col.Name] = spec
		}
		// Apply parseable CHECK constraints as column-level params (min/max/
		// max_len/values). Unrecognized checks stay in Table.Checks for validate
		// to surface.
		recognized := applyCheckConstraints(ct)
		// Any multi-column CHECK we can't parse expresses a semantic
		// invariant we won't satisfy by independent column sampling
		// (`ends_at > starts_at`, `finish > start`, …). Zero the table by
		// default so the apply doesn't fail; user can write a custom factory
		// pair and override.
		for _, chk := range ct.Checks {
			if recognized[chk.Name] {
				continue
			}
			if len(chk.Columns) >= 2 {
				rc = 0
				break
			}
		}
		// Mark polymorphic pair columns as unresolved so the user sees they
		// need candidate tables declared. sqlemit will special-case the pair
		// at emit time; until candidates are filled in, both columns emit
		// NULL (or fall through to the nullable check).
		for _, pk := range ct.Polymorphs {
			if col, ok := ct.Columns[pk.TypeColumn]; ok {
				col.Unresolved = true
			}
			if col, ok := ct.Columns[pk.IdColumn]; ok {
				col.Unresolved = true
			}
		}
		cfg.Tables[key] = ct
	}
	return cfg
}

func columnOrder(tbl schema.Table) []string {
	ordered := make([]string, 0, len(tbl.Columns))
	for _, c := range tbl.Columns {
		ordered = append(ordered, c.Name)
	}
	return ordered
}

func indexFK(tbl schema.Table) (map[string]string, map[string]bool) {
	out := map[string]string{}
	deferrable := map[string]bool{}
	for _, fk := range tbl.ForeignKeys {
		if len(fk.Columns) != 1 || len(fk.RefColumns) != 1 {
			continue // multi-column FK — user must declare target manually
		}
		col := fk.Columns[0]
		out[col] = fk.RefSchema + "." + fk.RefTable + "." + fk.RefColumns[0]
		if fk.Deferrable {
			deferrable[col] = true
		}
	}
	return out, deferrable
}

func compositeFKColumns(tbl schema.Table) map[string]bool {
	out := map[string]bool{}
	for _, fk := range tbl.ForeignKeys {
		if len(fk.Columns) > 1 {
			for _, c := range fk.Columns {
				out[c] = true
			}
		}
	}
	return out
}
