package config

import (
	"github.com/ivannikolaev/seed-cli/cli/internal/registry"
	"github.com/ivannikolaev/seed-cli/cli/internal/schema"
)

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
		enumValues[e.Name] = e.Values
	}
	for _, tbl := range model.Tables {
		key := QualifiedKey(tbl.Schema, tbl.Name)
		ct := &Table{
			Schema:   tbl.Schema,
			Name:     tbl.Name,
			RowCount: 100,
			Columns:  map[string]*ColumnSpec{},
			PKOrder:  collectPKOrder(tbl),
		}
		fkByCol := indexFK(tbl)
		for _, col := range tbl.Columns {
			apiCol := registry.ToAPIColumn(tbl, col, enumValues[col.EnumName], fkByCol[col.Name])
			res := reg.Infer(apiCol, defaults.Locale)
			spec := &ColumnSpec{
				Nullable: col.Nullable,
				DataType: col.DataType,
				Origin:   "inferred",
			}
			if res.Mechanism != nil {
				spec.Mechanism = res.Mechanism.Name()
			} else {
				spec.Mechanism = "string"
			}
			spec.Unresolved = res.Unresolved
			if apiCol.FKTarget != "" {
				spec.Params = map[string]any{"target": apiCol.FKTarget}
			}
			ct.Columns[col.Name] = spec
		}
		cfg.Tables[key] = ct
	}
	return cfg
}

func collectPKOrder(tbl schema.Table) []string {
	ordered := make([]string, 0, len(tbl.Columns))
	for _, c := range tbl.Columns {
		ordered = append(ordered, c.Name)
	}
	return ordered
}

func indexFK(tbl schema.Table) map[string]string {
	out := map[string]string{}
	for _, fk := range tbl.ForeignKeys {
		if len(fk.Columns) != 1 || len(fk.RefColumns) != 1 {
			continue // multi-column FK — user must declare target manually
		}
		out[fk.Columns[0]] = fk.RefSchema + "." + fk.RefTable + "." + fk.RefColumns[0]
	}
	return out
}
