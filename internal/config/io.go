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
	if c.Version == 0 {
		fmt.Fprintf(os.Stderr, "seed: warning: config has no version field; assuming version %d\n", CurrentVersion)
		c.Version = CurrentVersion
	} else if c.Version > CurrentVersion {
		return nil, fmt.Errorf("config version %d is newer than this CLI supports (%d); upgrade seed CLI", c.Version, CurrentVersion)
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
	if t.RowCount != nil {
		appendScalar(n, "row_count", *t.RowCount)
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
	if len(t.PrimaryKey) > 0 {
		appendSeq(n, "primary_key", t.PrimaryKey)
	}
	if len(t.UniqueKeys) > 0 {
		uksNode := &yaml.Node{Kind: yaml.SequenceNode}
		for _, uk := range t.UniqueKeys {
			inner := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
			for _, col := range uk {
				inner.Content = append(inner.Content, scalarNode(col))
			}
			uksNode.Content = append(uksNode.Content, inner)
		}
		n.Content = append(n.Content, keyNode("unique_keys"), uksNode)
	}
	if len(t.PartialUniqueKeys) > 0 {
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, p := range t.PartialUniqueKeys {
			entry := &yaml.Node{Kind: yaml.MappingNode, Style: yaml.FlowStyle}
			inner := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
			for _, c := range p.Columns {
				inner.Content = append(inner.Content, scalarNode(c))
			}
			entry.Content = append(entry.Content, keyNode("columns"), inner)
			entry.Content = append(entry.Content, keyNode("predicate"), scalarNode(p.Predicate))
			seq.Content = append(seq.Content, entry)
		}
		n.Content = append(n.Content, keyNode("partial_unique_keys"), seq)
	}
	if len(t.Checks) > 0 {
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, chk := range t.Checks {
			entry := &yaml.Node{Kind: yaml.MappingNode, Style: yaml.FlowStyle}
			entry.Content = append(entry.Content, keyNode("name"), scalarNode(chk.Name))
			entry.Content = append(entry.Content, keyNode("expression"), scalarNode(chk.Expression))
			if len(chk.Columns) > 0 {
				inner := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
				for _, c := range chk.Columns {
					inner.Content = append(inner.Content, scalarNode(c))
				}
				entry.Content = append(entry.Content, keyNode("columns"), inner)
			}
			seq.Content = append(seq.Content, entry)
		}
		n.Content = append(n.Content, keyNode("checks"), seq)
	}
	if len(t.Excludes) > 0 {
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, ex := range t.Excludes {
			entry := &yaml.Node{Kind: yaml.MappingNode, Style: yaml.FlowStyle}
			entry.Content = append(entry.Content, keyNode("name"), scalarNode(ex.Name))
			entry.Content = append(entry.Content, keyNode("definition"), scalarNode(ex.Definition))
			if len(ex.Columns) > 0 {
				inner := &yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle}
				for _, c := range ex.Columns {
					inner.Content = append(inner.Content, scalarNode(c))
				}
				entry.Content = append(entry.Content, keyNode("columns"), inner)
			}
			seq.Content = append(seq.Content, entry)
		}
		n.Content = append(n.Content, keyNode("excludes"), seq)
	}
	if t.TriggerPopulated {
		appendScalar(n, "trigger_populated", true)
	}
	if len(t.Polymorphs) > 0 {
		seq := &yaml.Node{Kind: yaml.SequenceNode}
		for _, pk := range t.Polymorphs {
			entry := &yaml.Node{Kind: yaml.MappingNode}
			entry.Content = append(entry.Content, keyNode("type_column"), scalarNode(pk.TypeColumn))
			entry.Content = append(entry.Content, keyNode("id_column"), scalarNode(pk.IdColumn))
			if len(pk.Candidates) > 0 {
				cseq := &yaml.Node{Kind: yaml.SequenceNode}
				for _, c := range pk.Candidates {
					cm := &yaml.Node{Kind: yaml.MappingNode, Style: yaml.FlowStyle}
					cm.Content = append(cm.Content, keyNode("table"), scalarNode(c.Table))
					if c.TypeName != "" {
						cm.Content = append(cm.Content, keyNode("type_name"), scalarNode(c.TypeName))
					}
					if c.PkColumn != "" {
						cm.Content = append(cm.Content, keyNode("pk_column"), scalarNode(c.PkColumn))
					}
					cseq.Content = append(cseq.Content, cm)
				}
				entry.Content = append(entry.Content, keyNode("candidates"), cseq)
			} else {
				// Empty flow sequence makes the hole visible in diffs.
				entry.Content = append(entry.Content, keyNode("candidates"),
					&yaml.Node{Kind: yaml.SequenceNode, Style: yaml.FlowStyle})
			}
			seq.Content = append(seq.Content, entry)
		}
		n.Content = append(n.Content, keyNode("polymorphs"), seq)
	}

	// Columns in ColumnOrder first, then alphabetic for the rest (stable output).
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
	for _, name := range t.ColumnOrder {
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
	style := yaml.FlowStyle
	if len(c.Values) > 0 {
		style = 0 // block style when nested JSON shape is present — flow would be unreadable.
	}
	n := &yaml.Node{Kind: yaml.MappingNode, Style: style}
	if c.Factory != "" {
		appendScalar(n, "factory", c.Factory)
	}
	if c.Value != nil {
		// Route Value through yaml.Encode so quoting is type-preserving:
		// `value: "42"` stays a string across round-trip instead of coercing
		// to int on reload.
		vn := &yaml.Node{}
		if err := vn.Encode(c.Value); err == nil {
			n.Content = append(n.Content, keyNode("value"), vn)
		} else {
			appendScalar(n, "value", c.Value)
		}
	}
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
	if len(c.Values) > 0 {
		vs := &yaml.Node{Kind: yaml.MappingNode}
		keys := make([]string, 0, len(c.Values))
		for k := range c.Values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			vs.Content = append(vs.Content, keyNode(k), columnNode(c.Values[k]))
		}
		n.Content = append(n.Content, keyNode("values"), vs)
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
	// For scalars fmt.Sprint is enough and keeps keys/simple values tidy.
	// For slices/maps/etc. rely on yaml.Node.Encode so the output parses back.
	switch v.(type) {
	case string, bool, int, int64, int32, float64, float32:
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprint(v)}
	}
	n := &yaml.Node{}
	if err := n.Encode(v); err != nil {
		fmt.Fprintf(os.Stderr, "seed: warning: YAML encode failed for %T: %v\n", v, err)
		return &yaml.Node{Kind: yaml.ScalarNode, Value: fmt.Sprint(v)}
	}
	return n
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
