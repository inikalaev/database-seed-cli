package introspect

import (
	"context"
	"fmt"
	"strings"

	"github.com/ivannikolaev/seed-cli/cli/internal/schema"
	"github.com/jackc/pgx/v5"
)

type Postgres struct{}

func (p *Postgres) Introspect(ctx context.Context, opts Options) (*schema.Model, error) {
	if opts.DSN == "" {
		return nil, fmt.Errorf("empty DSN")
	}
	conn, err := pgx.Connect(ctx, opts.DSN)
	if err != nil {
		return nil, fmt.Errorf("connect: %w", err)
	}
	defer conn.Close(ctx)

	schemas, err := p.resolveSchemas(ctx, conn, opts)
	if err != nil {
		return nil, err
	}
	if len(schemas) == 0 {
		return nil, fmt.Errorf("no matching user schemas found")
	}

	m := &schema.Model{Dialect: "postgres", Schemas: schemas}

	if m.Enums, err = p.loadEnums(ctx, conn, schemas); err != nil {
		return nil, fmt.Errorf("enums: %w", err)
	}
	if m.Tables, err = p.loadTables(ctx, conn, schemas); err != nil {
		return nil, fmt.Errorf("tables: %w", err)
	}
	if err := p.loadColumns(ctx, conn, schemas, m); err != nil {
		return nil, fmt.Errorf("columns: %w", err)
	}
	if err := p.loadConstraints(ctx, conn, schemas, m); err != nil {
		return nil, fmt.Errorf("constraints: %w", err)
	}

	m.SortStable()
	return m, nil
}

func (p *Postgres) resolveSchemas(ctx context.Context, conn *pgx.Conn, opts Options) ([]string, error) {
	if opts.SchemaAll {
		rows, err := conn.Query(ctx, `
			SELECT schema_name
			FROM information_schema.schemata
			WHERE schema_name NOT LIKE 'pg_%'
			  AND schema_name <> 'information_schema'
			ORDER BY schema_name`)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		var out []string
		for rows.Next() {
			var s string
			if err := rows.Scan(&s); err != nil {
				return nil, err
			}
			out = append(out, s)
		}
		return out, rows.Err()
	}
	if len(opts.Schemas) == 0 {
		return []string{"public"}, nil
	}
	return opts.Schemas, nil
}

func (p *Postgres) loadEnums(ctx context.Context, conn *pgx.Conn, schemas []string) ([]schema.Enum, error) {
	rows, err := conn.Query(ctx, `
		SELECT n.nspname, t.typname, e.enumlabel
		FROM pg_type t
		JOIN pg_enum e ON e.enumtypid = t.oid
		JOIN pg_namespace n ON n.oid = t.typnamespace
		WHERE n.nspname = ANY($1)
		ORDER BY n.nspname, t.typname, e.enumsortorder`, schemas)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	type key struct{ s, n string }
	idx := map[key]int{}
	var out []schema.Enum
	for rows.Next() {
		var sName, eName, label string
		if err := rows.Scan(&sName, &eName, &label); err != nil {
			return nil, err
		}
		k := key{sName, eName}
		i, ok := idx[k]
		if !ok {
			idx[k] = len(out)
			out = append(out, schema.Enum{Schema: sName, Name: eName})
			i = len(out) - 1
		}
		out[i].Values = append(out[i].Values, label)
	}
	return out, rows.Err()
}

func (p *Postgres) loadTables(ctx context.Context, conn *pgx.Conn, schemas []string) ([]schema.Table, error) {
	rows, err := conn.Query(ctx, `
		SELECT table_schema, table_name
		FROM information_schema.tables
		WHERE table_schema = ANY($1)
		  AND table_type = 'BASE TABLE'
		ORDER BY table_schema, table_name`, schemas)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []schema.Table
	for rows.Next() {
		var t schema.Table
		if err := rows.Scan(&t.Schema, &t.Name); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, rows.Err()
}

func (p *Postgres) loadColumns(ctx context.Context, conn *pgx.Conn, schemas []string, m *schema.Model) error {
	rows, err := conn.Query(ctx, `
		SELECT
			c.table_schema, c.table_name, c.column_name, c.ordinal_position,
			c.data_type, c.udt_name, c.is_nullable = 'YES' AS nullable,
			c.column_default, c.is_generated <> 'NEVER' AS is_generated,
			c.is_identity = 'YES' AS is_identity,
			c.character_maximum_length, c.numeric_precision, c.numeric_scale
		FROM information_schema.columns c
		WHERE c.table_schema = ANY($1)
		ORDER BY c.table_schema, c.table_name, c.ordinal_position`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			sName, tName, cName, dataType, udt string
			pos                                int
			nullable, isGen, isIdent           bool
			def                                *string
			charMax, numPrec, numScale         *int
		)
		if err := rows.Scan(&sName, &tName, &cName, &pos, &dataType, &udt, &nullable, &def, &isGen, &isIdent, &charMax, &numPrec, &numScale); err != nil {
			return err
		}
		t := m.FindTable(sName, tName)
		if t == nil {
			continue
		}
		col := schema.Column{
			Name:         cName,
			Position:     pos,
			DataType:     dataType,
			UDTName:      udt,
			Nullable:     nullable,
			Default:      def,
			IsGenerated:  isGen,
			IsIdentity:   isIdent,
			CharMaxLen:   charMax,
			NumPrecision: numPrec,
			NumScale:     numScale,
		}
		// Arrays: data_type = "ARRAY", udt_name starts with "_"
		if strings.EqualFold(dataType, "ARRAY") {
			col.ArrayDims = 1
		}
		// Enum linkage: data_type = "USER-DEFINED", udt_name = enum type name
		if strings.EqualFold(dataType, "USER-DEFINED") {
			for _, e := range m.Enums {
				if e.Name == udt {
					col.EnumName = e.Name
					break
				}
			}
		}
		t.Columns = append(t.Columns, col)
	}
	return rows.Err()
}

func (p *Postgres) loadConstraints(ctx context.Context, conn *pgx.Conn, schemas []string, m *schema.Model) error {
	if err := p.loadPrimaryAndUnique(ctx, conn, schemas, m); err != nil {
		return err
	}
	return p.loadForeignKeys(ctx, conn, schemas, m)
}

func (p *Postgres) loadPrimaryAndUnique(ctx context.Context, conn *pgx.Conn, schemas []string, m *schema.Model) error {
	rows, err := conn.Query(ctx, `
		SELECT tc.table_schema, tc.table_name, tc.constraint_type, kcu.column_name, kcu.ordinal_position, tc.constraint_name
		FROM information_schema.table_constraints tc
		JOIN information_schema.key_column_usage kcu
		  ON kcu.constraint_schema = tc.constraint_schema
		 AND kcu.constraint_name = tc.constraint_name
		 AND kcu.table_schema = tc.table_schema
		 AND kcu.table_name = tc.table_name
		WHERE tc.table_schema = ANY($1)
		  AND tc.constraint_type IN ('PRIMARY KEY', 'UNIQUE')
		ORDER BY tc.table_schema, tc.table_name, tc.constraint_name, kcu.ordinal_position`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()

	type key struct{ s, t, c string }
	groups := map[key][]string{}
	kinds := map[key]string{}
	for rows.Next() {
		var sName, tName, kind, col, consName string
		var pos int
		if err := rows.Scan(&sName, &tName, &kind, &col, &pos, &consName); err != nil {
			return err
		}
		k := key{sName, tName, consName}
		groups[k] = append(groups[k], col)
		kinds[k] = kind
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for k, cols := range groups {
		t := m.FindTable(k.s, k.t)
		if t == nil {
			continue
		}
		switch kinds[k] {
		case "PRIMARY KEY":
			t.PrimaryKey = cols
		case "UNIQUE":
			t.UniqueKeys = append(t.UniqueKeys, cols)
		}
	}
	return nil
}

func (p *Postgres) loadForeignKeys(ctx context.Context, conn *pgx.Conn, schemas []string, m *schema.Model) error {
	rows, err := conn.Query(ctx, `
		SELECT
			n.nspname AS table_schema,
			c.relname AS table_name,
			con.conname,
			con.condeferrable,
			con.condeferred,
			con.confupdtype,
			con.confdeltype,
			con.conkey,
			con.confkey,
			rn.nspname AS ref_schema,
			rc.relname AS ref_table,
			array(
				SELECT a.attname FROM pg_attribute a
				WHERE a.attrelid = c.oid AND a.attnum = ANY(con.conkey)
				ORDER BY array_position(con.conkey, a.attnum)
			) AS src_cols,
			array(
				SELECT a.attname FROM pg_attribute a
				WHERE a.attrelid = rc.oid AND a.attnum = ANY(con.confkey)
				ORDER BY array_position(con.confkey, a.attnum)
			) AS ref_cols
		FROM pg_constraint con
		JOIN pg_class c ON c.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		JOIN pg_class rc ON rc.oid = con.confrelid
		JOIN pg_namespace rn ON rn.oid = rc.relnamespace
		WHERE con.contype = 'f'
		  AND n.nspname = ANY($1)
		ORDER BY n.nspname, c.relname, con.conname`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var (
			sName, tName, consName         string
			deferrable, deferred           bool
			updType, delType               string
			conkey, confkey                []int16
			refSchema, refTable            string
			srcCols, refCols               []string
		)
		if err := rows.Scan(&sName, &tName, &consName, &deferrable, &deferred, &updType, &delType, &conkey, &confkey, &refSchema, &refTable, &srcCols, &refCols); err != nil {
			return err
		}
		t := m.FindTable(sName, tName)
		if t == nil {
			continue
		}
		t.ForeignKeys = append(t.ForeignKeys, schema.ForeignKey{
			Name:         consName,
			Columns:      srcCols,
			RefSchema:    refSchema,
			RefTable:     refTable,
			RefColumns:   refCols,
			OnDelete:     actionFromCode(delType),
			OnUpdate:     actionFromCode(updType),
			Deferrable:   deferrable,
			InitDeferred: deferred,
		})
	}
	return rows.Err()
}

func actionFromCode(c string) string {
	switch c {
	case "a":
		return "NO ACTION"
	case "r":
		return "RESTRICT"
	case "c":
		return "CASCADE"
	case "n":
		return "SET NULL"
	case "d":
		return "SET DEFAULT"
	default:
		return ""
	}
}
