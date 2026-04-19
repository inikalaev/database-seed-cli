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
	Schema      string
	Name        string
	Columns     []Column
	PrimaryKey  []string
	UniqueKeys  [][]string
	ForeignKeys []ForeignKey
}

func (t Table) QualifiedName() string {
	if t.Schema == "" || t.Schema == "public" {
		return t.Name
	}
	return t.Schema + "." + t.Name
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
	Name           string
	Columns        []string
	RefSchema      string
	RefTable       string
	RefColumns     []string
	OnDelete       string
	OnUpdate       string
	Deferrable     bool
	InitDeferred   bool
	MatchPartially bool
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
