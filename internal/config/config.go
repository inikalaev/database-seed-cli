// Package config defines the YAML schema exposed to the user and all I/O around it.
//
// Design notes:
//   - Table keys are fully-qualified "schema.table"; the short form "table" is only
//     accepted on load when exactly one schema is present, and is always written
//     fully qualified on save to keep merges stable.
//   - Every column entry is a ColumnSpec; the Origin field ("inferred" | "user")
//     drives the merge policy: inferred fields may be overwritten by a later
//     introspection, user-authored fields are preserved.
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
	Schema      string                 `yaml:"-"`
	Name        string                 `yaml:"-"`
	RowCount    int                    `yaml:"row_count,omitempty"`
	RowCountPer map[string][2]int      `yaml:"row_count_per,omitempty"`
	Tags        []string               `yaml:"tags,omitempty"`
	Removed     bool                   `yaml:"removed,omitempty"`
	Columns     map[string]*ColumnSpec `yaml:"columns"`
	// PKOrder preserves the physical column order seen at introspection time;
	// used to emit SQL with a predictable column list.
	PKOrder []string `yaml:"-"`
}

type ColumnSpec struct {
	Mechanism string                 `yaml:"mechanism,omitempty"`
	// Value, when set, is emitted verbatim for every row — mechanism is ignored.
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
	// Origin is "inferred" or "user". Inferred fields are schema-derived and may
	// be rewritten on `sync`; user-authored fields are preserved.
	Origin string `yaml:"-"`
}

const CurrentVersion = 1
