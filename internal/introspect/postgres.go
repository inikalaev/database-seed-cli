package introspect

import (
	"context"
	"fmt"
	"regexp"
	"sort"
	"strings"

	"github.com/inikalaev/database-seed-cli/internal/schema"
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
			c.data_type, c.udt_name, c.udt_schema, c.is_nullable = 'YES' AS nullable,
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
			sName, tName, cName, dataType, udt, udtSchema string
			pos                                            int
			nullable, isGen, isIdent                      bool
			def                                           *string
			charMax, numPrec, numScale                    *int
		)
		if err := rows.Scan(&sName, &tName, &cName, &pos, &dataType, &udt, &udtSchema, &nullable, &def, &isGen, &isIdent, &charMax, &numPrec, &numScale); err != nil {
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
		// Enum linkage: data_type = "USER-DEFINED". Use udt_schema to avoid
		// name collision between enums in different schemas (e.g. public.status
		// vs billing.status). Store schema-qualified name so build.go can use
		// a schema-qualified map key without ambiguity.
		if strings.EqualFold(dataType, "USER-DEFINED") {
			for _, e := range m.Enums {
				if e.Name == udt && e.Schema == udtSchema {
					col.EnumName = e.Schema + "." + e.Name
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
	if err := p.loadUniqueIndexes(ctx, conn, schemas, m); err != nil {
		return err
	}
	if err := p.loadForeignKeys(ctx, conn, schemas, m); err != nil {
		return err
	}
	if err := p.loadCheckConstraints(ctx, conn, schemas, m); err != nil {
		return err
	}
	if err := p.loadExcludeConstraints(ctx, conn, schemas, m); err != nil {
		return err
	}
	if err := p.loadTriggerPopulated(ctx, conn, schemas, m); err != nil {
		return err
	}
	p.detectPolymorphs(m)
	return nil
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
	// Sort constraint keys for deterministic output — map iteration is random.
	sortedKeys := make([]key, 0, len(groups))
	for k := range groups {
		sortedKeys = append(sortedKeys, k)
	}
	sort.Slice(sortedKeys, func(i, j int) bool {
		a, b := sortedKeys[i], sortedKeys[j]
		if a.s != b.s {
			return a.s < b.s
		}
		if a.t != b.t {
			return a.t < b.t
		}
		return a.c < b.c
	})
	for _, k := range sortedKeys {
		cols := groups[k]
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

// loadUniqueIndexes picks up UNIQUE enforced via `CREATE UNIQUE INDEX` (common
// in Rails/Django apps) that don't show up in information_schema.table_constraints.
// Expression components (indkey == 0 at that position) are resolved via
// pg_get_indexdef parsing — functional indexes like `lower(full_name)` are
// approximated by tracking `full_name` as the uniqueness column, which is
// correct whenever the generator produces distinct raw values (our default).
// Partial indexes are captured separately as PartialUniqueKeys so validate
// can surface them as info (soft-delete patterns like `WHERE deleted_at IS
// NULL` are extremely common).
func (p *Postgres) loadUniqueIndexes(ctx context.Context, conn *pgx.Conn, schemas []string, m *schema.Model) error {
	// Shortlist unique non-PK indexes with their raw definition. Expression
	// handling happens per-index in Go rather than SQL because PG has no
	// clean SQL-level extractor for expression trees.
	rows, err := conn.Query(ctx, `
		SELECT
			n.nspname,
			c.relname,
			ic.relname AS index_name,
			pg_get_indexdef(ix.indexrelid) AS indexdef,
			pg_get_expr(ix.indpred, ix.indrelid) AS predicate,
			ix.indkey::text AS indkey,
			array(
				SELECT a.attname FROM pg_attribute a
				WHERE a.attrelid = c.oid
			) AS all_cols,
			array(
				SELECT a.attname FROM pg_attribute a
				WHERE a.attrelid = c.oid AND a.attnum = ANY(ix.indkey)
				ORDER BY array_position(ix.indkey::int[], a.attnum::int)
			) AS direct_cols
		FROM pg_index ix
		JOIN pg_class c  ON c.oid  = ix.indrelid
		JOIN pg_class ic ON ic.oid = ix.indexrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE ix.indisunique
		  AND NOT ix.indisprimary
		  AND NOT EXISTS (SELECT 1 FROM pg_constraint pc WHERE pc.conindid = ix.indexrelid)
		  AND n.nspname = ANY($1)
		ORDER BY n.nspname, c.relname, ic.relname`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var sName, tName, indexName, indexDef, indkey string
		var predicate *string
		var allCols, directCols []string
		if err := rows.Scan(&sName, &tName, &indexName, &indexDef, &predicate, &indkey, &allCols, &directCols); err != nil {
			return err
		}
		t := m.FindTable(sName, tName)
		if t == nil {
			continue
		}
		cols := resolveIndexCols(indexDef, indkey, allCols, directCols)
		if len(cols) == 0 {
			continue
		}
		if predicate != nil && *predicate != "" {
			t.PartialUniqueKeys = append(t.PartialUniqueKeys, schema.PartialUniqueKey{
				Columns:   cols,
				Predicate: *predicate,
			})
			continue
		}
		if !hasUniqueKey(t.UniqueKeys, cols) {
			t.UniqueKeys = append(t.UniqueKeys, cols)
		}
	}
	return rows.Err()
}

// resolveIndexCols builds the column list for a UNIQUE index. Direct columns
// (indkey != 0) come straight from pg_attribute. Expression components (indkey
// == 0) are approximated by scanning pg_get_indexdef for references to known
// table columns. The result drives uniqueness tracking during generation —
// for `lower(full_name)` we register `full_name`, which is correct whenever
// the factory emits distinct raw values and safely over-constrains otherwise.
func resolveIndexCols(indexDef, indkey string, allCols, directCols []string) []string {
	hasExpr := strings.Contains(indkey, "0")
	if !hasExpr {
		return append([]string(nil), directCols...)
	}
	// Parenthesised substring is the index expression list. Anything outside
	// it (USING clauses, WHERE predicate) would otherwise leak bare idents.
	open := strings.Index(indexDef, "(")
	close := strings.LastIndex(indexDef, ")")
	if open < 0 || close < open {
		return append([]string(nil), directCols...)
	}
	inner := indexDef[open+1 : close]
	found := map[string]bool{}
	seenOrder := make([]string, 0, len(directCols)+2)
	add := func(name string) {
		if found[name] {
			return
		}
		found[name] = true
		seenOrder = append(seenOrder, name)
	}
	for _, c := range directCols {
		add(c)
	}
	wordRe := regexp.MustCompile(`[A-Za-z_][A-Za-z0-9_]*`)
	knownCol := map[string]bool{}
	for _, c := range allCols {
		knownCol[c] = true
	}
	for _, m := range wordRe.FindAllString(inner, -1) {
		if knownCol[m] {
			add(m)
		}
	}
	return seenOrder
}

// loadCheckConstraints reads CHECK expressions from pg_constraint. Expressions
// with convalidated=false or conislocal=false are skipped to avoid duplicates
// on inheritance hierarchies. Columns lists attnums from conkey (may be empty
// for table-level checks that reference no column directly).
func (p *Postgres) loadCheckConstraints(ctx context.Context, conn *pgx.Conn, schemas []string, m *schema.Model) error {
	rows, err := conn.Query(ctx, `
		SELECT
			n.nspname,
			c.relname,
			con.conname,
			pg_get_expr(con.conbin, con.conrelid) AS expression,
			COALESCE(array(
				SELECT a.attname FROM pg_attribute a
				WHERE a.attrelid = c.oid AND a.attnum = ANY(con.conkey)
				ORDER BY array_position(con.conkey, a.attnum)
			), '{}') AS cols
		FROM pg_constraint con
		JOIN pg_class c ON c.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE con.contype = 'c'
		  AND con.convalidated
		  AND con.conislocal
		  AND n.nspname = ANY($1)
		ORDER BY n.nspname, c.relname, con.conname`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sName, tName, name, expr string
		var cols []string
		if err := rows.Scan(&sName, &tName, &name, &expr, &cols); err != nil {
			return err
		}
		t := m.FindTable(sName, tName)
		if t == nil {
			continue
		}
		t.CheckConstraints = append(t.CheckConstraints, schema.CheckConstraint{
			Name:       name,
			Expression: expr,
			Columns:    cols,
		})
	}
	return rows.Err()
}

// loadExcludeConstraints reads EXCLUDE constraints (contype='x'). Definition is
// pg_get_constraintdef output — semantics are too varied for generic handling,
// but surfacing the raw definition lets validate warn the user.
func (p *Postgres) loadExcludeConstraints(ctx context.Context, conn *pgx.Conn, schemas []string, m *schema.Model) error {
	rows, err := conn.Query(ctx, `
		SELECT
			n.nspname,
			c.relname,
			con.conname,
			pg_get_constraintdef(con.oid, true) AS definition,
			COALESCE(array(
				SELECT a.attname FROM pg_attribute a
				WHERE a.attrelid = c.oid AND a.attnum = ANY(con.conkey)
				ORDER BY array_position(con.conkey, a.attnum)
			), '{}') AS cols
		FROM pg_constraint con
		JOIN pg_class c ON c.oid = con.conrelid
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE con.contype = 'x'
		  AND con.conislocal
		  AND n.nspname = ANY($1)
		ORDER BY n.nspname, c.relname, con.conname`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var sName, tName, name, def string
		var cols []string
		if err := rows.Scan(&sName, &tName, &name, &def, &cols); err != nil {
			return err
		}
		t := m.FindTable(sName, tName)
		if t == nil {
			continue
		}
		t.ExcludeConstraints = append(t.ExcludeConstraints, schema.ExcludeConstraint{
			Name:       name,
			Definition: def,
			Columns:    cols,
		})
	}
	return rows.Err()
}

func hasUniqueKey(existing [][]string, cols []string) bool {
	for _, uk := range existing {
		if len(uk) != len(cols) {
			continue
		}
		match := true
		for i := range uk {
			if uk[i] != cols[i] {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func (p *Postgres) loadForeignKeys(ctx context.Context, conn *pgx.Conn, schemas []string, m *schema.Model) error {
	rows, err := conn.Query(ctx, `
		SELECT
			n.nspname AS table_schema,
			c.relname AS table_name,
			con.conname,
			con.condeferrable,
			con.condeferred,
			con.confupdtype::text,
			con.confdeltype::text,
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
			sName, tName, consName string
			deferrable, deferred   bool
			updType, delType       string
			refSchema, refTable    string
			srcCols, refCols       []string
		)
		if err := rows.Scan(&sName, &tName, &consName, &deferrable, &deferred, &updType, &delType, &refSchema, &refTable, &srcCols, &refCols); err != nil {
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

// loadTriggerPopulated inspects each trigger function attached to a table in
// the introspected schemas and flags tables referenced by INSERT INTO inside
// the function body. This catches Rails-style counter/search-index patterns
// where a trigger on `comments` maintains rows in `searchers`, so the target
// table would collide on PK if the generator tries to seed it independently.
//
// The regex is deliberately simple — dynamic SQL (EXECUTE format()) won't
// match. That's acceptable: false negatives only degrade to the pre-existing
// behaviour where the user has to discover the issue via apply failure and
// zero the row count manually.
func (p *Postgres) loadTriggerPopulated(ctx context.Context, conn *pgx.Conn, schemas []string, m *schema.Model) error {
	rows, err := conn.Query(ctx, `
		SELECT DISTINCT
			tn.nspname AS trigger_schema,
			pn.nspname AS func_schema,
			p.prosrc
		FROM pg_trigger t
		JOIN pg_proc p ON p.oid = t.tgfoid
		JOIN pg_namespace pn ON pn.oid = p.pronamespace
		JOIN pg_class tc ON tc.oid = t.tgrelid
		JOIN pg_namespace tn ON tn.oid = tc.relnamespace
		WHERE NOT t.tgisinternal
		  AND tn.nspname = ANY($1)`, schemas)
	if err != nil {
		return err
	}
	defer rows.Close()

	// Accumulate target-table names referenced by any trigger body, then flag
	// matching tables in the model. Deduplicated via a set to keep the later
	// O(#tables) loop cheap.
	targets := map[string]bool{}
	insertRe := regexp.MustCompile(`(?i)INSERT\s+INTO\s+(?:"?([A-Za-z_][A-Za-z0-9_]*)"?\s*\.\s*)?"?([A-Za-z_][A-Za-z0-9_]*)"?`)
	for rows.Next() {
		var triggerSchema, funcSchema, src string
		if err := rows.Scan(&triggerSchema, &funcSchema, &src); err != nil {
			return err
		}
		for _, match := range insertRe.FindAllStringSubmatch(src, -1) {
			s, t := match[1], match[2]
			if s == "" {
				// Unqualified — best guess is the schema the trigger lives in.
				s = triggerSchema
			}
			targets[s+"."+t] = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	for i := range m.Tables {
		t := &m.Tables[i]
		if targets[t.Schema+"."+t.Name] {
			t.TriggerPopulated = true
		}
	}
	return nil
}

// detectPolymorphs finds Rails-style polymorphic pointer pairs on each table:
// a `<name>_type` varchar/text column alongside a `<name>_id` integer/bigint
// column. Only surfaces the pairs — candidate parent tables are left to the
// user to declare in the config (auto-guessing from table names would be a
// noise generator since the type string is application-defined).
func (p *Postgres) detectPolymorphs(m *schema.Model) {
	for i := range m.Tables {
		t := &m.Tables[i]
		byName := map[string]*schema.Column{}
		for j := range t.Columns {
			c := &t.Columns[j]
			byName[c.Name] = c
		}
		for name, col := range byName {
			if !strings.HasSuffix(name, "_type") {
				continue
			}
			if !isTextLike(col.DataType) {
				continue
			}
			base := strings.TrimSuffix(name, "_type")
			idCol, ok := byName[base+"_id"]
			if !ok {
				continue
			}
			if !isIntegerLike(idCol.DataType) {
				continue
			}
			t.Polymorphs = append(t.Polymorphs, schema.PolymorphicKey{
				TypeColumn: name,
				IdColumn:   base + "_id",
			})
		}
		sort.SliceStable(t.Polymorphs, func(a, b int) bool { return t.Polymorphs[a].TypeColumn < t.Polymorphs[b].TypeColumn })
	}
}

func isTextLike(dataType string) bool {
	switch strings.ToLower(dataType) {
	case "text", "character varying", "varchar", "citext", "character":
		return true
	}
	return false
}

func isIntegerLike(dataType string) bool {
	switch strings.ToLower(dataType) {
	case "integer", "bigint", "smallint", "int", "int2", "int4", "int8":
		return true
	}
	return false
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
