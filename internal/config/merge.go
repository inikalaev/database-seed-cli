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
//   - Column spec: if existing declares a Factory (non-empty), every
//     user-authored field wins. Schema-derived fields (DataType, Nullable)
//     always take the incoming value.
//   - Unresolved flag tracks the latest inference: if the user later sets a
//     Factory manually, Unresolved is cleared even if inference still can't
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
		RowCount:    pickIntPtr(existing.RowCount, incoming.RowCount),
		RowCountPer: pickRowCountPer(existing.RowCountPer, incoming.RowCountPer),
		Tags:        pickStrings(existing.Tags, incoming.Tags),
		Removed:     false,
		Columns:     map[string]*ColumnSpec{},
		ColumnOrder:       incoming.ColumnOrder,
		PrimaryKey:        incoming.PrimaryKey,        // schema-derived, always refresh from incoming
		UniqueKeys:        incoming.UniqueKeys,        // schema-derived, always refresh from incoming
		PartialUniqueKeys: incoming.PartialUniqueKeys, // schema-derived, always refresh
		Checks:            incoming.Checks,            // schema-derived, always refresh
		Excludes:          incoming.Excludes,          // schema-derived, always refresh
		TriggerPopulated:  incoming.TriggerPopulated,  // schema-derived
		Polymorphs:        mergePolymorphs(existing.Polymorphs, incoming.Polymorphs),
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

	// Existing factory always wins on re-sync (user may have edited it).
	// If the user wants fresh inference, they delete the factory value.
	if existing.Factory != "" {
		out.Factory = existing.Factory
		out.Unresolved = existing.Unresolved
	}
	// Literal value wins — user set it explicitly.
	if existing.Value != nil {
		out.Value = existing.Value
	}
	// Params merge key-by-key: schema-derived keys refresh from incoming so
	// enum labels / FK targets / deferrable stay accurate; user-added keys
	// survive. Wholesale overwrite here was the cause of enum-label drift.
	out.Params = mergeParams(existing.Params, incoming.Params)
	// values is user-authored JSON shape — preserve it on sync.
	if len(existing.Values) > 0 {
		out.Values = existing.Values
	}
	return &out
}

func pickIntPtr(a, b *int) *int {
	if a != nil {
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

// inferredParamKeys lists params whose value is derived from the database schema
// during FromModel. These refresh on every sync so the config stays in sync with
// the DB truth; everything else is considered user-authored and preserved.
//
// "target" is intentionally included: it reflects the actual FK constraint in
// the DB. If a user wants to remap an FK to a different table, they should
// change the factory away from fkref rather than editing params.target, since
// the next sync would overwrite any manual target edit.
var inferredParamKeys = map[string]bool{
	"target":     true,
	"values":     true,
	"deferrable": true,
	"element":    true,
	"max_len":    true,
}

func mergeParams(existing, incoming map[string]any) map[string]any {
	if len(existing) == 0 && len(incoming) == 0 {
		return nil
	}
	out := map[string]any{}
	for k, v := range existing {
		if inferredParamKeys[k] {
			continue // drop stale schema-derived value — incoming will re-add.
		}
		out[k] = v
	}
	for k, v := range incoming {
		if inferredParamKeys[k] {
			out[k] = v
			continue
		}
		if _, ok := out[k]; !ok {
			out[k] = v // user hasn't overridden it — pick up new inferred defaults.
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func pickRowCountPer(a, b map[string][2]int) map[string][2]int {
	if len(a) > 0 {
		return a
	}
	return b
}

// mergePolymorphs keeps incoming (freshly-detected) pairs — they're
// schema-derived — but preserves any user-authored candidates from existing.
// A pair that disappears from introspection drops out; a new pair shows up
// with empty candidates so the user knows to fill them in.
func mergePolymorphs(existing, incoming []PolymorphicKey) []PolymorphicKey {
	if len(incoming) == 0 {
		return nil
	}
	byKey := map[string][]PolymorphCandidate{}
	for _, p := range existing {
		byKey[p.TypeColumn+"|"+p.IdColumn] = p.Candidates
	}
	out := make([]PolymorphicKey, len(incoming))
	for i, p := range incoming {
		out[i] = PolymorphicKey{
			TypeColumn: p.TypeColumn,
			IdColumn:   p.IdColumn,
			Candidates: byKey[p.TypeColumn+"|"+p.IdColumn],
		}
	}
	return out
}
