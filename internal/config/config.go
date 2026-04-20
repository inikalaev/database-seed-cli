// Package config defines the YAML schema exposed to the user and all I/O around it.
//
// Design notes:
//   - Table keys are fully-qualified "schema.table"; the short form "table" is only
//     accepted on load when exactly one schema is present, and is always written
//     fully qualified on save to keep merges stable.
//   - Merge policy is Factory-driven: a non-empty Factory in the existing config
//     is treated as user-authored and wins over freshly-inferred values. Leaving
//     Factory empty re-enables inference on the next sync.
//   - Removed tables/columns are flagged `removed: true` rather than deleted, so
//     the user can decide whether the absence was intentional.
package config

type Config struct {
	Version  int               `yaml:"version"`
	Database DatabaseSection   `yaml:"database"`
	Defaults DefaultsSection   `yaml:"defaults,omitempty"`
	Tables   map[string]*Table `yaml:"tables"`
}

type DatabaseSection struct {
	Dialect string   `yaml:"dialect"`
	Schemas []string `yaml:"schemas,omitempty"`
}

type DefaultsSection struct {
	Locale string `yaml:"locale,omitempty"`
	Seed   int64  `yaml:"seed,omitempty"`
}

type Table struct {
	Schema string `yaml:"-"`
	Name   string `yaml:"-"`
	// RowCount is a pointer so "row_count: 0" (do not generate) is distinct from
	// "unset" (fall back to default). Merge preserves whichever side has a value.
	RowCount    *int              `yaml:"row_count,omitempty"`
	RowCountPer map[string][2]int `yaml:"row_count_per,omitempty"`
	Tags        []string               `yaml:"tags,omitempty"`
	Removed     bool                   `yaml:"removed,omitempty"`
	Columns     map[string]*ColumnSpec `yaml:"columns"`
	// ColumnOrder preserves the physical column order seen at introspection time
	// and drives the INSERT column list. Not serialized — set by FromModel /
	// Merge and empty after a YAML round-trip; sqlemit.activeColumns falls back
	// to alphabetic order when it is missing.
	ColumnOrder []string `yaml:"-"`
	// PrimaryKey is the set of PK columns; used to populate the FK pool. It
	// must survive a round-trip through YAML, otherwise composite/non-"id" PKs
	// would be forgotten between `sync` and `generate`.
	PrimaryKey []string `yaml:"primary_key,omitempty"`
	// UniqueKeys stores each UNIQUE constraint as an ordered column list.
	// Schema-derived; always refreshed on sync. Used by validate to warn about
	// factories that may not guarantee uniqueness.
	UniqueKeys [][]string `yaml:"unique_keys,omitempty"`
	// PartialUniqueKeys captures UNIQUE indexes with a WHERE clause (soft-delete
	// patterns etc.). Schema-derived, not enforced by generation — surfaced by
	// validate as info so the user knows the risk.
	PartialUniqueKeys []PartialUniqueKey `yaml:"partial_unique_keys,omitempty"`
	// Checks captures CHECK constraints. Schema-derived; always refreshed on
	// sync. Simple expressions (col op literal, BETWEEN, IN, length bounds) are
	// additionally parsed at build time and applied to column params.
	Checks []CheckConstraint `yaml:"checks,omitempty"`
	// Excludes captures EXCLUDE constraints — semantics are too varied for
	// generic handling, but the raw definition is surfaced by validate.
	Excludes []ExcludeConstraint `yaml:"excludes,omitempty"`
	// TriggerPopulated marks the table as side-populated by triggers on
	// other tables. Schema-derived, always refreshed on sync. Build sets
	// row_count to 0 by default for these tables; the user can override.
	TriggerPopulated bool `yaml:"trigger_populated,omitempty"`
	// Polymorphs lists detected `<name>_type`/`<name>_id` pairs. Candidates
	// are user-authored — introspect only surfaces the pair; the user fills
	// in the list of parent tables that can legitimately fill both columns.
	Polymorphs []PolymorphicKey `yaml:"polymorphs,omitempty"`
}

type PolymorphicKey struct {
	TypeColumn string             `yaml:"type_column"`
	IdColumn   string             `yaml:"id_column"`
	Candidates []PolymorphCandidate `yaml:"candidates,omitempty"`
}

type PolymorphCandidate struct {
	// Table is the fully-qualified parent table name ("schema.table"). The
	// parent must itself be present in the config so its PK pool is built
	// before this table emits.
	Table string `yaml:"table"`
	// TypeName is the string written into the `_type` column when this
	// candidate is picked (e.g. Rails class name "User"). Defaults to the
	// CamelCase singular of the table basename when omitted.
	TypeName string `yaml:"type_name,omitempty"`
	// PkColumn is the parent's PK column; defaults to "id" when unset.
	PkColumn string `yaml:"pk_column,omitempty"`
}

type PartialUniqueKey struct {
	Columns   []string `yaml:"columns"`
	Predicate string   `yaml:"predicate"`
}

type CheckConstraint struct {
	Name       string   `yaml:"name"`
	Expression string   `yaml:"expression"`
	Columns    []string `yaml:"columns,omitempty"`
}

type ExcludeConstraint struct {
	Name       string   `yaml:"name"`
	Definition string   `yaml:"definition"`
	Columns    []string `yaml:"columns,omitempty"`
}

type ColumnSpec struct {
	Factory    string                 `yaml:"factory,omitempty"`
	// Value, when set, is emitted verbatim for every row — factory is ignored.
	// Accepts any scalar YAML type: string, int, float, bool.
	Value      any                    `yaml:"value,omitempty"`
	Params     map[string]any         `yaml:"params,omitempty"`
	// Values defines the shape of a JSON/JSONB column as a map of field name →
	// nested ColumnSpec. When set, the emitter builds a JSON object from this
	// shape instead of calling the mechanism's Generate. Nested Values are
	// supported for arbitrary depth.
	Values     map[string]*ColumnSpec `yaml:"values,omitempty"`
	Unresolved bool                   `yaml:"unresolved,omitempty"`
	Removed    bool                   `yaml:"removed,omitempty"`
	Nullable   bool                   `yaml:"nullable,omitempty"`
	DataType   string                 `yaml:"data_type,omitempty"`
}

const CurrentVersion = 1
