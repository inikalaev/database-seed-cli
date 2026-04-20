// Package schema describes the database shape as parsed by introspection.
//
// The model is deliberately dialect-agnostic: PG-specific concerns (enum OIDs,
// array dimensionality, domain types) are flattened into neutral fields before
// they cross this package boundary.
package schema

import "sort"

type Model struct {
	Dialect string
	Schemas []string
	Tables  []Table
	Enums   []Enum
}

type Table struct {
	Schema             string
	Name               string
	Columns            []Column
	PrimaryKey         []string
	UniqueKeys         [][]string
	PartialUniqueKeys  []PartialUniqueKey
	ForeignKeys        []ForeignKey
	CheckConstraints   []CheckConstraint
	ExcludeConstraints []ExcludeConstraint
	// TriggerPopulated is set when an INSERT into this table appears inside
	// the body of a trigger function attached to some other table. Such
	// tables (search indexes, audit logs, maintained counters) fill up as a
	// side effect of seeding their sources, so the generator should default
	// row_count to 0 and let triggers do the work. Users can still override.
	TriggerPopulated bool
	// Polymorphs lists polymorphic pointer pairs detected on this table —
	// the Rails/ActiveRecord `X_type` (class name) + `X_id` (integer)
	// convention. Candidates are left empty at introspect time; the user
	// fills them in the config, and emit samples one per row to fill both
	// columns atomically.
	Polymorphs []PolymorphicKey
}

// PolymorphicKey names a pair of columns representing a single logical
// pointer to one of several parent tables. TypeColumn holds a string
// identifier (conventionally the Rails class name, e.g. "User") and
// IdColumn holds the PK of that parent.
type PolymorphicKey struct {
	TypeColumn string
	IdColumn   string
}

// PartialUniqueKey is a UNIQUE index with a WHERE clause (e.g. soft-delete
// pattern `WHERE deleted_at IS NULL`). Not enforced by the generator — surfaced
// as info so the user can decide whether to tune row_count or live with the risk.
type PartialUniqueKey struct {
	Columns   []string
	Predicate string
}

// CheckConstraint captures a table-level CHECK. Expression is the raw SQL
// (e.g. "price > 0"). Columns lists the columns referenced by the expression,
// derived from pg_constraint.conkey.
type CheckConstraint struct {
	Name       string
	Expression string
	Columns    []string
}

// ExcludeConstraint captures an EXCLUDE constraint (e.g. range overlap
// prevention via GIST). Definition is the raw `pg_get_constraintdef` output
// (e.g. "EXCLUDE USING gist (room_id WITH =, during WITH &&)"). Columns lists
// the columns from pg_constraint.conkey.
type ExcludeConstraint struct {
	Name       string
	Definition string
	Columns    []string
}

type Column struct {
	Name         string
	DataType     string
	UDTName      string
	Nullable     bool
	Default      *string
	IsGenerated  bool
	IsIdentity   bool
	CharMaxLen   *int
	NumPrecision *int
	NumScale     *int
	ArrayDims    int
	EnumName     string
	Position     int
}

type ForeignKey struct {
	Name         string
	Columns      []string
	RefSchema    string
	RefTable     string
	RefColumns   []string
	OnDelete     string
	OnUpdate     string
	Deferrable   bool
	InitDeferred bool
}

type Enum struct {
	Schema string
	Name   string
	Values []string
}

func (m *Model) SortStable() {
	sort.SliceStable(m.Tables, func(i, j int) bool {
		a, b := m.Tables[i], m.Tables[j]
		if a.Schema != b.Schema {
			return a.Schema < b.Schema
		}
		return a.Name < b.Name
	})
	for i := range m.Tables {
		t := &m.Tables[i]
		sort.SliceStable(t.Columns, func(a, b int) bool { return t.Columns[a].Position < t.Columns[b].Position })
		sort.SliceStable(t.ForeignKeys, func(a, b int) bool { return t.ForeignKeys[a].Name < t.ForeignKeys[b].Name })
	}
	sort.SliceStable(m.Enums, func(i, j int) bool {
		a, b := m.Enums[i], m.Enums[j]
		if a.Schema != b.Schema {
			return a.Schema < b.Schema
		}
		return a.Name < b.Name
	})
}

func (m *Model) FindTable(schemaName, tableName string) *Table {
	for i := range m.Tables {
		t := &m.Tables[i]
		if t.Schema == schemaName && t.Name == tableName {
			return t
		}
	}
	return nil
}
