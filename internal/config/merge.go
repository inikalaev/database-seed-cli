package config

// Merge applies freshly-introspected data (`incoming`) onto an existing config
// (`existing`) while preserving every user-authored field.
//
// Merge rules:
//   - Tables present in both: merge column by column. New columns inherit the
//     inferred spec. Columns missing from the DB are marked Removed=true but
//     kept in the config.
//   - Tables only in `incoming`: added as-is.
//   - Tables only in `existing`: marked Removed=true, user edits preserved.
//   - Column spec: if existing has a non-inferred Mechanism (Origin != "inferred"),
//     every user-authored field wins. Schema-derived fields (DataType, Nullable)
//     always take the incoming value.
//   - Unresolved flag tracks the latest inference: if the user later sets a
//     Mechanism manually, Unresolved is cleared even if inference still can't
//     classify the column.
func Merge(existing, incoming *Config) *Config {
	if existing == nil {
		return incoming
	}
	out := &Config{
		Version:  incoming.Version,
		Database: incoming.Database,
		Defaults: pickDefaults(existing.Defaults, incoming.Defaults),
		Tables:   map[string]*Table{},
	}
	for key, inc := range incoming.Tables {
		exist := existing.Tables[key]
		out.Tables[key] = mergeTable(exist, inc)
	}
	for key, exist := range existing.Tables {
		if _, stillPresent := incoming.Tables[key]; stillPresent {
			continue
		}
		// Table removed in the DB — keep it with removed=true.
		copy := *exist
		copy.Removed = true
		out.Tables[key] = &copy
	}
	return out
}

func pickDefaults(existing, incoming DefaultsSection) DefaultsSection {
	// User-authored defaults win. Introspection doesn't have an opinion about them.
	out := existing
	if out.Locale == "" {
		out.Locale = incoming.Locale
	}
	if out.Seed == 0 {
		out.Seed = incoming.Seed
	}
	return out
}

func mergeTable(existing, incoming *Table) *Table {
	if existing == nil {
		return incoming
	}
	out := &Table{
		Schema:      incoming.Schema,
		Name:        incoming.Name,
		RowCount:    pickInt(existing.RowCount, incoming.RowCount),
		RowCountPer: pickRowCountPer(existing.RowCountPer, incoming.RowCountPer),
		Tags:        pickStrings(existing.Tags, incoming.Tags),
		Removed:     false,
		Columns:     map[string]*ColumnSpec{},
		PKOrder:     incoming.PKOrder,
	}
	for name, incCol := range incoming.Columns {
		existCol := existing.Columns[name]
		out.Columns[name] = mergeColumn(existCol, incCol)
	}
	for name, existCol := range existing.Columns {
		if _, stillPresent := incoming.Columns[name]; stillPresent {
			continue
		}
		copy := *existCol
		copy.Removed = true
		out.Columns[name] = &copy
	}
	return out
}

func mergeColumn(existing, incoming *ColumnSpec) *ColumnSpec {
	if existing == nil {
		return incoming
	}
	out := *incoming
	// Schema-derived fields always take the incoming value.
	out.Nullable = incoming.Nullable
	out.DataType = incoming.DataType
	out.Removed = false

	// Existing mechanism always wins on re-sync (user may have edited it).
	// If the user wants fresh inference, they delete the mechanism value.
	if existing.Mechanism != "" {
		out.Mechanism = existing.Mechanism
		out.Unresolved = existing.Unresolved
		out.Origin = existing.Origin
	}
	// Literal value wins — user set it explicitly.
	if existing.Value != nil {
		out.Value = existing.Value
	}
	// Existing params win wholesale. We don't deep-merge — the YAML shape is too varied.
	if len(existing.Params) > 0 {
		out.Params = existing.Params
	}
	// values is user-authored JSON shape — preserve it on sync.
	if len(existing.Values) > 0 {
		out.Values = existing.Values
	}
	return &out
}

func pickInt(a, b int) int {
	if a != 0 {
		return a
	}
	return b
}

func pickStrings(a, b []string) []string {
	if len(a) > 0 {
		return a
	}
	return b
}

func pickRowCountPer(a, b map[string][2]int) map[string][2]int {
	if len(a) > 0 {
		return a
	}
	return b
}
