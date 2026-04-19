package config

import (
	"fmt"
	"os"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Unmarshal(data)
}

func Unmarshal(data []byte) (*Config, error) {
	var c Config
	if err := yaml.Unmarshal(data, &c); err != nil {
		return nil, err
	}
	c.normalize()
	return &c, nil
}

func Save(path string, c *Config) error {
	data, err := Marshal(c)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func Marshal(c *Config) ([]byte, error) {
	// Emit via a node tree to guarantee key order.
	node := &yaml.Node{Kind: yaml.MappingNode}
	appendScalar(node, "version", c.Version)
	{
		db := &yaml.Node{Kind: yaml.MappingNode}
		appendScalar(db, "dialect", c.Database.Dialect)
		if len(c.Database.Schemas) > 0 {
			appendSeq(db, "schemas", c.Database.Schemas)
		}
		node.Content = append(node.Content, keyNode("database"), db)
	}
	if c.Defaults.Locale != "" || c.Defaults.Seed != 0 {
		d := &yaml.Node{Kind: yaml.MappingNode}
		if c.Defaults.Locale != "" {
			appendScalar(d, "locale", c.Defaults.Locale)
		}
		if c.Defaults.Seed != 0 {
			appendScalar(d, "seed", c.Defaults.Seed)
		}
		node.Content = append(node.Content, keyNode("defaults"), d)
	}

	// Tables — ordered by qualified name.
	tbls := &yaml.Node{Kind: yaml.MappingNode}
	keys := make([]string, 0, len(c.Tables))
	for k := range c.Tables {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		t := c.Tables[k]
		tbls.Content = append(tbls.Content, keyNode(k), tableNode(t))
	}
	node.Content = append(node.Content, keyNode("tables"), tbls)

	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{node}}
	data, err := yaml.Marshal(doc)
	if err != nil {
		return nil, err
	}
	return data, nil
}

func tableNode(t *Table) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	if t.Removed {
		appendScalar(n, "removed", true)
	}
	if t.RowCount > 0 {
		appendScalar(n, "row_count", t.RowCount)
	}
	if len(t.RowCountPer) > 0 {
		per := &yaml.Node{Kind: yaml.MappingNode}
		keys := make([]string, 0, len(t.RowCountPer))
		for k := range t.RowCountPer {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			pair := t.RowCountPer[k]
			seq := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
			seq.Content = append(seq.Content, scalarNode(pair[0]), scalarNode(pair[1]))
			per.Content = append(per.Content, keyNode(k), seq)
		}
		n.Content = append(n.Content, keyNode("row_count_per"), per)
	}
	if len(t.Tags) > 0 {
		appendSeq(n, "tags", t.Tags)
	}

	// Columns in PKOrder first, then alphabetic for the rest (stable output).
	cols := &yaml.Node{Kind: yaml.MappingNode}
	seen := map[string]bool{}
	emit := func(name string) {
		c, ok := t.Columns[name]
		if !ok {
			return
		}
		cols.Content = append(cols.Content, keyNode(name), columnNode(c))
		seen[name] = true
	}
	for _, name := range t.PKOrder {
		emit(name)
	}
	rest := make([]string, 0, len(t.Columns))
	for k := range t.Columns {
		if !seen[k] {
			rest = append(rest, k)
		}
	}
	sort.Strings(rest)
	for _, name := range rest {
		emit(name)
	}
	n.Content = append(n.Content, keyNode("columns"), cols)
	return n
}

func columnNode(c *ColumnSpec) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode, Style: yaml.FlowStyle}
	appendScalar(n, "mechanism", c.Mechanism)
	if len(c.Params) > 0 {
		p := &yaml.Node{Kind: yaml.MappingNode, Style: yaml.FlowStyle}
		keys := make([]string, 0, len(c.Params))
		for k := range c.Params {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			p.Content = append(p.Content, keyNode(k), scalarNode(c.Params[k]))
		}
		n.Content = append(n.Content, keyNode("params"), p)
	}
	if c.Unresolved {
		appendScalar(n, "unresolved", true)
	}
	if c.Removed {
		appendScalar(n, "removed", true)
	}
	if c.DataType != "" {
		appendScalar(n, "data_type", c.DataType)
	}
	if c.Nullable {
		appendScalar(n, "nullable", true)
	}
	return n
}

func appendScalar(parent *yaml.Node, key string, value any) {
	parent.Content = append(parent.Content, keyNode(key), scalarNode(value))
}

func appendSeq(parent *yaml.Node, key string, values []string) {
	seq := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
	for _, v := range values {
		seq.Content = append(seq.Content, scalarNode(v))
	}
	parent.Content = append(parent.Content, keyNode(key), seq)
}

func keyNode(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: s}
}

func scalarNode(v any) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprint(v)}
}

// normalize: ensure Table.Schema/Name are populated from the map key.
func (c *Config) normalize() {
	if c.Tables == nil {
		c.Tables = map[string]*Table{}
	}
	// If database.schemas is empty and every key is unqualified, default to "public".
	onlySchema := ""
	if len(c.Database.Schemas) == 1 {
		onlySchema = c.Database.Schemas[0]
	}
	for k, t := range c.Tables {
		t.Schema, t.Name = splitQualified(k, onlySchema)
		if t.Columns == nil {
			t.Columns = map[string]*ColumnSpec{}
		}
		for _, col := range t.Columns {
			// Anything loaded from disk is considered user-owned; sync must preserve it.
			col.Origin = "user"
		}
	}
}

func splitQualified(qualified, defaultSchema string) (string, string) {
	if idx := strings.Index(qualified, "."); idx >= 0 {
		return qualified[:idx], qualified[idx+1:]
	}
	if defaultSchema == "" {
		defaultSchema = "public"
	}
	return defaultSchema, qualified
}

// QualifiedKey returns the canonical "schema.table" key for the map.
func QualifiedKey(schemaName, tableName string) string {
	if schemaName == "" {
		schemaName = "public"
	}
	return schemaName + "." + tableName
}
