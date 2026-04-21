package introspect

import (
	"fmt"
	"os"
	"strings"

	pg_query "github.com/pganalyze/pg_query_go/v6"

	"github.com/inikalaev/database-seed-cli/internal/schema"
)

// DDL parses a SQL DDL file (e.g. from pg_dump --schema-only) into schema.Model.
// No live DB connection is required.
type DDL struct{}

func (d *DDL) Introspect(path, defaultSchema string) (*schema.Model, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	result, err := pg_query.Parse(string(src))
	if err != nil {
		return nil, fmt.Errorf("parse DDL: %w", err)
	}
	if defaultSchema == "" {
		defaultSchema = "public"
	}

	m := &schema.Model{Dialect: "postgres"}
	schemaSet := map[string]struct{}{}

	for _, raw := range result.Stmts {
		if raw.Stmt == nil {
			continue
		}
		if cs := raw.Stmt.GetCreateStmt(); cs != nil {
			d.handleCreate(cs, m, schemaSet, defaultSchema)
		} else if at := raw.Stmt.GetAlterTableStmt(); at != nil {
			d.handleAlter(at, m, defaultSchema)
		} else if ce := raw.Stmt.GetCreateEnumStmt(); ce != nil {
			d.handleEnum(ce, m, schemaSet, defaultSchema)
		}
	}

	for s := range schemaSet {
		m.Schemas = append(m.Schemas, s)
	}
	if len(m.Schemas) == 0 {
		m.Schemas = []string{defaultSchema}
	}
	m.SortStable()
	return m, nil
}

func (d *DDL) handleCreate(n *pg_query.CreateStmt, m *schema.Model, schemaSet map[string]struct{}, defaultSchema string) {
	if n.Relation == nil {
		return
	}
	sName := firstNonEmpty(n.Relation.Schemaname, defaultSchema)
	tName := n.Relation.Relname
	if tName == "" {
		return
	}
	schemaSet[sName] = struct{}{}

	t := schema.Table{Schema: sName, Name: tName}
	pos := 0
	for _, item := range n.TableElts {
		if cd := item.GetColumnDef(); cd != nil {
			pos++
			col := d.parseColumnDef(cd, pos, m, sName)
			// Inline column constraints that promote to table level.
			for _, c := range cd.Constraints {
				con := c.GetConstraint()
				if con == nil {
					continue
				}
				switch con.Contype {
				case pg_query.ConstrType_CONSTR_PRIMARY:
					t.PrimaryKey = append(t.PrimaryKey, col.Name)
				case pg_query.ConstrType_CONSTR_FOREIGN:
					fk := d.parseFKConstraint(con, sName)
					fk.Columns = []string{col.Name}
					t.ForeignKeys = append(t.ForeignKeys, fk)
				case pg_query.ConstrType_CONSTR_UNIQUE:
					t.UniqueKeys = append(t.UniqueKeys, []string{col.Name})
				}
			}
			t.Columns = append(t.Columns, col)
		} else if con := item.GetConstraint(); con != nil {
			d.applyTableConstraint(con, &t, sName)
		}
	}
	m.Tables = append(m.Tables, t)
}

func (d *DDL) parseColumnDef(el *pg_query.ColumnDef, pos int, m *schema.Model, sName string) schema.Column {
	col := schema.Column{
		Name:     el.Colname,
		Position: pos,
		Nullable: true,
	}

	var udtSchema string
	if el.TypeName != nil {
		var dims int
		col.DataType, col.UDTName, udtSchema, dims = resolveTypeName(el.TypeName)
		col.ArrayDims = dims
	}

	// is_not_null is set by the parser for NOT NULL shorthand in ColumnDef
	if el.IsNotNull {
		col.Nullable = false
	}
	if el.Identity != "" {
		col.IsIdentity = true
		col.Nullable = false
	}
	if el.Generated != "" {
		col.IsGenerated = true
	}

	for _, c := range el.Constraints {
		con := c.GetConstraint()
		if con == nil {
			continue
		}
		switch con.Contype {
		case pg_query.ConstrType_CONSTR_NOTNULL:
			col.Nullable = false
		case pg_query.ConstrType_CONSTR_NULL:
			col.Nullable = true
		case pg_query.ConstrType_CONSTR_DEFAULT:
			if con.RawExpr != nil {
				s := exprToString(con.RawExpr)
				col.Default = &s
			}
		case pg_query.ConstrType_CONSTR_IDENTITY:
			col.IsIdentity = true
			col.Nullable = false
		case pg_query.ConstrType_CONSTR_GENERATED:
			col.IsGenerated = true
		case pg_query.ConstrType_CONSTR_PRIMARY:
			col.Nullable = false
		}
	}

	// Expand SERIAL pseudo-types
	switch strings.ToLower(col.DataType) {
	case "serial", "serial4":
		col.DataType = "integer"
		col.UDTName = "int4"
		s := "nextval(...)"
		col.Default = &s
		col.Nullable = false
	case "bigserial", "serial8":
		col.DataType = "bigint"
		col.UDTName = "int8"
		s := "nextval(...)"
		col.Default = &s
		col.Nullable = false
	case "smallserial", "serial2":
		col.DataType = "smallint"
		col.UDTName = "int2"
		s := "nextval(...)"
		col.Default = &s
		col.Nullable = false
	}

	// Link enum. udtSchema comes from the written type (`schema.name`); if the
	// DDL used a bare name, fall back to the table's schema.
	if strings.EqualFold(col.DataType, "USER-DEFINED") {
		enumSchema := udtSchema
		if enumSchema == "" {
			enumSchema = sName
		}
		for _, e := range m.Enums {
			if e.Schema == enumSchema && e.Name == col.UDTName {
				col.EnumName = e.Schema + "." + e.Name
				break
			}
		}
	}

	return col
}

func (d *DDL) applyTableConstraint(con *pg_query.Constraint, t *schema.Table, srcSchema string) {
	switch con.Contype {
	case pg_query.ConstrType_CONSTR_PRIMARY:
		for _, k := range con.Keys {
			if s := nodeString(k); s != "" {
				t.PrimaryKey = append(t.PrimaryKey, s)
			}
		}
	case pg_query.ConstrType_CONSTR_UNIQUE:
		var cols []string
		for _, k := range con.Keys {
			if s := nodeString(k); s != "" {
				cols = append(cols, s)
			}
		}
		if len(cols) > 0 {
			t.UniqueKeys = append(t.UniqueKeys, cols)
		}
	case pg_query.ConstrType_CONSTR_FOREIGN:
		t.ForeignKeys = append(t.ForeignKeys, d.parseFKConstraint(con, srcSchema))
	}
}

func (d *DDL) handleAlter(n *pg_query.AlterTableStmt, m *schema.Model, defaultSchema string) {
	if n.Relation == nil {
		return
	}
	sName := firstNonEmpty(n.Relation.Schemaname, defaultSchema)
	t := m.FindTable(sName, n.Relation.Relname)
	if t == nil {
		return
	}
	for _, cmd := range n.Cmds {
		ac := cmd.GetAlterTableCmd()
		if ac == nil || ac.Subtype != pg_query.AlterTableType_AT_AddConstraint {
			continue
		}
		con := ac.Def.GetConstraint()
		if con == nil {
			continue
		}
		d.applyTableConstraint(con, t, sName)
	}
}

func (d *DDL) handleEnum(n *pg_query.CreateEnumStmt, m *schema.Model, schemaSet map[string]struct{}, defaultSchema string) {
	var sName, eName string
	switch len(n.TypeName) {
	case 1:
		sName = defaultSchema
		eName = nodeString(n.TypeName[0])
	case 2:
		sName = nodeString(n.TypeName[0])
		eName = nodeString(n.TypeName[1])
	default:
		return
	}
	schemaSet[sName] = struct{}{}
	e := schema.Enum{Schema: sName, Name: eName}
	for _, v := range n.Vals {
		e.Values = append(e.Values, nodeString(v))
	}
	m.Enums = append(m.Enums, e)
}

func (d *DDL) parseFKConstraint(con *pg_query.Constraint, srcSchema string) schema.ForeignKey {
	fk := schema.ForeignKey{
		Name:         con.Conname,
		Deferrable:   con.Deferrable,
		InitDeferred: con.Initdeferred,
	}
	for _, k := range con.FkAttrs {
		if s := nodeString(k); s != "" {
			fk.Columns = append(fk.Columns, s)
		}
	}
	if con.Pktable != nil {
		fk.RefSchema = firstNonEmpty(con.Pktable.Schemaname, srcSchema)
		fk.RefTable = con.Pktable.Relname
	}
	for _, k := range con.PkAttrs {
		if s := nodeString(k); s != "" {
			fk.RefColumns = append(fk.RefColumns, s)
		}
	}
	fk.OnDelete = fkActionFromChar(con.FkDelAction)
	fk.OnUpdate = fkActionFromChar(con.FkUpdAction)
	return fk
}

// resolveTypeName maps a pg_query TypeName to (dataType, udtName, udtSchema, arrayDims).
// Mirrors information_schema.columns conventions: udtName is the unqualified
// type name (prefixed with `_` for arrays, matching pg_catalog); udtSchema is
// the qualifier if one was written in the DDL (empty → caller should fall
// back to the table's schema). arrayDims is 0 for non-array types.
func resolveTypeName(tn *pg_query.TypeName) (dataType, udtName, udtSchema string, arrayDims int) {
	var parts []string
	for _, n := range tn.Names {
		if s := nodeString(n); s != "" && s != "pg_catalog" {
			parts = append(parts, s)
		}
	}
	lowered := make([]string, len(parts))
	for i, p := range parts {
		lowered[i] = strings.ToLower(p)
	}

	// For qualified types (`schema.type`), split the qualifier off. For bare
	// names, udtSchema stays empty and the caller picks a fallback.
	var last, qual string
	switch len(lowered) {
	case 0:
		return "", "", "", 0
	case 1:
		last = lowered[0]
	default:
		qual = lowered[len(lowered)-2]
		last = lowered[len(lowered)-1]
	}

	dims := len(tn.ArrayBounds)
	dt := pgCatalogToInfoSchema(last)
	if dims > 0 {
		return "ARRAY", "_" + last, qual, dims
	}
	return dt, last, qual, 0
}

func pgCatalogToInfoSchema(udt string) string {
	switch udt {
	case "int2", "smallint":
		return "smallint"
	case "int4", "int", "integer":
		return "integer"
	case "int8", "bigint":
		return "bigint"
	case "float4", "real":
		return "real"
	case "float8", "double precision":
		return "double precision"
	case "numeric", "decimal":
		return "numeric"
	case "bool", "boolean":
		return "boolean"
	case "text":
		return "text"
	case "varchar", "character varying":
		return "character varying"
	case "bpchar", "char", "character":
		return "character"
	case "bytea":
		return "bytea"
	case "date":
		return "date"
	case "time", "timetz":
		return "time without time zone"
	case "timestamp":
		return "timestamp without time zone"
	case "timestamptz":
		return "timestamp with time zone"
	case "interval":
		return "interval"
	case "uuid":
		return "uuid"
	case "json":
		return "json"
	case "jsonb":
		return "jsonb"
	case "xml":
		return "xml"
	case "serial", "serial4":
		return "serial"
	case "bigserial", "serial8":
		return "bigserial"
	case "smallserial", "serial2":
		return "smallserial"
	default:
		return "USER-DEFINED"
	}
}

func fkActionFromChar(s string) string {
	if len(s) == 0 {
		return ""
	}
	switch s[0] {
	case 'a':
		return "NO ACTION"
	case 'r':
		return "RESTRICT"
	case 'c':
		return "CASCADE"
	case 'n':
		return "SET NULL"
	case 'd':
		return "SET DEFAULT"
	default:
		return ""
	}
}

func nodeString(n *pg_query.Node) string {
	if n == nil {
		return ""
	}
	if s := n.GetString_(); s != nil {
		return s.Sval
	}
	if c := n.GetAConst(); c != nil {
		if sv := c.GetSval(); sv != nil {
			return sv.Sval
		}
	}
	return ""
}

func exprToString(n *pg_query.Node) string {
	if n == nil {
		return ""
	}
	if c := n.GetAConst(); c != nil {
		if sv := c.GetSval(); sv != nil {
			return sv.Sval
		}
		if iv := c.GetIval(); iv != nil {
			return fmt.Sprintf("%d", iv.Ival)
		}
		if fv := c.GetFval(); fv != nil {
			return fv.Fval
		}
	}
	if fc := n.GetFuncCall(); fc != nil {
		var parts []string
		for _, fn := range fc.Funcname {
			parts = append(parts, nodeString(fn))
		}
		return strings.Join(parts, ".") + "(...)"
	}
	if tc := n.GetTypeCast(); tc != nil {
		return exprToString(tc.Arg)
	}
	if cr := n.GetColumnRef(); cr != nil {
		var parts []string
		for _, f := range cr.Fields {
			parts = append(parts, nodeString(f))
		}
		return strings.Join(parts, ".")
	}
	return ""
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}
