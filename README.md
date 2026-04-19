# database-seed-cli

Go CLI for generating **relationally consistent** synthetic data for PostgreSQL databases.

Useful for: populating local development environments, preparing realistic datasets for **load testing**, and writing deterministic integration test fixtures.

> **Database support:** PostgreSQL only. MySQL, SQLite and others are planned for future releases.

Two explicit phases:

1. **Introspect → config.** `seed init --dsn <…> -o seed.yaml` reads the live schema and writes a YAML config where every column gets a `mechanism` (e.g. `pk_serial`, `email`, `fkref`). Columns that inference cannot classify are flagged `unresolved: true`.
2. **Config → SQL.** `seed generate -c seed.yaml -o seed.sql` walks the FK graph, generates rows in dependency order, and writes a single SQL script. You run it: `psql -f seed.sql`.

## Install

```bash
git clone https://github.com/inikalaev/database-seed-cli
cd database-seed-cli/cli
make build            # produces ./bin/seed-cli
```

Requires Go 1.25+. The `--generators` flag additionally requires a Go toolchain at runtime (the CLI recompiles itself with your plugins).

## Quick start

```bash
# Create a fresh config from a live DB
./bin/seed-cli init --dsn postgres://user:pass@localhost/app -o seed.yaml

# Edit seed.yaml — set row_count, tweak mechanisms/params, resolve `unresolved: true`

# Re-introspect after schema changes — user edits are preserved
./bin/seed-cli sync --dsn postgres://user:pass@localhost/app -c seed.yaml

# Lint the config
./bin/seed-cli validate -c seed.yaml

# Emit SQL and apply
./bin/seed-cli generate -c seed.yaml -o seed.sql
psql $DATABASE_URL -f seed.sql
```

### Multi-schema

```bash
seed-cli init --dsn … --schema public --schema billing -o seed.yaml
seed-cli init --dsn … --schema-all -o seed.yaml   # everything except pg_*/information_schema
```

Table keys in the YAML are fully-qualified `schema.table`; short form `table` is allowed only when the config covers a single schema.

## Commands

| Command               | Purpose                                                       |
|-----------------------|---------------------------------------------------------------|
| `seed-cli init`           | Introspect DB → write new YAML config.                        |
| `seed-cli sync`           | Re-introspect → merge into existing YAML (idempotent).         |
| `seed-cli introspect`     | Print raw schema JSON (debug / tooling).                       |
| `seed-cli validate`       | Report unresolved columns, cycles, missing FK targets.         |
| `seed-cli generate`       | Read config → emit SQL file.                                   |

## Design invariants

- **Idempotent merge.** `seed-cli sync` preserves every user edit; schema-derived fields refresh; removed tables/columns are flagged `removed: true`, not deleted.
- **Unresolved marking.** Inference never silently guesses — columns it cannot classify land in the config with `unresolved: true`.
- **Shared relation graph.** FK topology and insert order live in `internal/relations`; one source of truth for CLI and consumers.
- **Go plugins via a folder.** Drop `.go` files into a directory, pass `--generators ./dir`. The CLI recompiles itself with your mechanisms and caches the binary under `$XDG_CACHE_HOME/seed-cli/<hash>`. MVP needs `SEED_CLI_SRC=<path to cli/>`.

---

## Config reference

```yaml
version: 1
database:
  dialect: postgres           # only postgres in MVP
  schemas: [public, billing]  # schemas covered by introspection
defaults:
  locale: ru_RU               # pool selection hint for name/address mechanisms
  seed: 42                    # deterministic seed for generators (0 = nondeterministic)
tables:
  public.users:
    row_count: 1000
    tags: [core]
    columns:
      id:         { mechanism: pk_serial, data_type: integer }
      email:      { mechanism: email, params: { domain: acme.io }, data_type: text }
      first_name: { mechanism: first_name, data_type: text }
      metadata:   { mechanism: json_any, unresolved: true, data_type: jsonb }

  public.orders:
    # Per-parent expansion: for every row in users, insert 1..20 orders.
    row_count_per: { users: [1, 20] }
    columns:
      id:      { mechanism: pk_serial, data_type: integer }
      user_id: { mechanism: fkref, params: { target: public.users.id }, data_type: integer }
      total:   { mechanism: decimal, params: { min: 0, max: 10000 }, data_type: numeric }
```

### Field semantics

| Field                          | Who sets it     | Preserved on `sync`? |
|--------------------------------|-----------------|---------------------|
| `version`                      | CLI             | n/a                 |
| `database.dialect` / `schemas` | CLI             | CLI rewrites        |
| `defaults.*`                   | user            | yes                 |
| `tables.*.row_count`           | user (or 100)   | yes                 |
| `tables.*.row_count_per`       | user            | yes                 |
| `tables.*.tags`                | user            | yes                 |
| `tables.*.removed`             | CLI             | flagged, not deleted|
| `columns.*.mechanism`          | inferred → user | yes if user edited  |
| `columns.*.params`             | inferred → user | yes if user edited  |
| `columns.*.unresolved`         | CLI             | re-evaluated        |
| `columns.*.data_type`          | CLI             | CLI rewrites        |
| `columns.*.nullable`           | CLI             | CLI rewrites        |

### Built-in mechanisms

| Mechanism     | Typical columns                                  | Notes |
|---------------|--------------------------------------------------|-------|
| `pk_serial`   | integer PK (`id`)                                | Sequential starting from `params.start` (default 1) |
| `uuid`        | `uuid`                                           | Version 4 |
| `fkref`       | any FK column                                    | `params.target: schema.table.column` |
| `first_name`, `last_name`, `full_name` | name fields                             | Pool is locale-aware (extend via custom mechanism) |
| `email`       | `email`                                          | `params.domain: example.com` |
| `phone`, `url`, `company`, `city`, `country` | obvious contact/address fields    | |
| `enum_value`  | PG `USER-DEFINED` enum columns                    | Chooses uniformly from the enum labels |
| `integer`, `decimal` | numeric types                              | `params.min`, `params.max` |
| `bool`        | boolean                                          | |
| `timestamp`, `date` | temporal types                             | |
| `string`      | text fallback                                    | Pattern: `<column>_<row>` |
| `json_any`    | json / jsonb                                     | Emits `{"row": N}` — override for real schemas |

### `row_count_per`

Mapping of `parent_table` → `[lo, hi]`. The planner multiplies the parent's row count by the midpoint of the range:

```yaml
tables:
  public.orders:
    row_count_per: { users: [1, 20] }
```

With `public.users.row_count = 1000`, the planner produces `1000 * 10 = 10_000` orders. If both `row_count` and `row_count_per` are set, `row_count_per` wins.

### Literal value with `value`

Any column (or JSON field inside `values`) can be given a fixed literal instead of a mechanism. The same value is emitted for every row.

```yaml
columns:
  status:   { value: "active",  data_type: text }
  is_admin: { value: false,     data_type: boolean }
  version:  { value: 1,         data_type: integer }
  metadata:
    mechanism: json_any
    data_type: jsonb
    values:
      type:  { value: "user" }
      score: { mechanism: integer, params: { min: 1, max: 100 } }
```

Priority order: `value` → `values` (JSON shape) → `mechanism`.

### JSON shape with `values`

For `json` / `jsonb` columns, set `values` to define the object shape inline. Each key maps to a nested `ColumnSpec` with its own `mechanism` and optional `params`. Nesting is arbitrary.

```yaml
columns:
  metadata:
    mechanism: json_any
    data_type: jsonb
    values:
      name:  { mechanism: first_name }
      score: { mechanism: integer, params: { min: 1, max: 100 } }
      addr:
        mechanism: json_any
        values:
          city:    { mechanism: city }
          country: { mechanism: country }
```

When `values` is present the emitter builds the JSON object from those specs and ignores the column's own `mechanism`. The resulting SQL literal looks like `'{"addr":{"city":"Moscow","country":"RU"},"name":"Ivan","score":42}'`.

### FK cycles

`seed-cli validate` reports FK cycles. The emitter wraps the script in `SET CONSTRAINTS ALL DEFERRED`. Use `fkref` on both sides — the emitter samples PK values in plan order.

---

## Extending: custom mechanisms

Convention: **one generator per file**. Builtins live under `internal/mechanisms/<name>.go` with shared predicates in `helpers.go` and registration order in `mechanisms.go` (`All()`). User plugins follow the same rule under their `--generators ./dir`.

```bash
export SEED_CLI_SRC=/path/to/seed-cli/cli
seed-cli generate -c seed.yaml -o seed.sql --generators ./seed-generators
```

On first use the CLI compiles an augmented binary (your generators + stock mechanisms) and caches it under `~/.cache/seed-cli/<hash>`. Subsequent runs re-exec the cached binary.

See [`examples/custom-generators/sku.go`](examples/custom-generators/sku.go) for a complete reference plugin.

### Template

```go
package seedgens

import (
    "fmt"
    "regexp"

    "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type SKU struct{}

func (SKU) Name() string   { return "sku" }
func (SKU) Tags() []string { return []string{"product", "inventory"} }

func (SKU) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
    if ok, _ := regexp.MatchString(`(?i)^sku$|article|product_code`, ctx.Column.Name); ok {
        return seedapi.StrongMatch
    }
    return seedapi.NoMatch
}

func (SKU) Generate(ctx seedapi.GenContext) any {
    return fmt.Sprintf("SKU-%06d", ctx.Row+1)
}

func init() { seedapi.Register(SKU{}) }
```

### The `seedapi.Mechanism` contract

- **`Name()`** — unique across all registered mechanisms. Referenced in YAML: `mechanism: sku`.
- **`Tags()`** — used for filtering and reporting. Not read by the emitter today; keep them meaningful.
- **`Match(ctx)`** — returns a score 0–100. Inference picks the highest-scoring mechanism per column. Be conservative; a too-aggressive matcher steals columns from more specific ones. Scale:
  - `0` — no match.
  - `~10` — weak fallback (`string`).
  - `~40` — type-based match (`integer`).
  - `~70` — probable name match.
  - `~90` — strong match (specific name/type/unique tokens).
  - `100` — FK references (never override unless the user says so).
- **`Generate(ctx)`** — value to put in the row. Return `nil` for `NULL`. Use `ctx.Rng` for determinism; never touch `rand`-package globals.

### FK pool access

`ctx.FKPool.Pick(schema, table, column, ctx.Rng)` samples previously-generated PK values for a referenced table. Use this for composite or conditional FK generators.

### Testing plugins

Normal Go unit tests next to the mechanism file (same `seedgens` package):

```go
func TestSKUMatches(t *testing.T) {
    sc := SKU{}.Match(seedapi.MatchContext{Column: seedapi.Column{Name: "sku", DataType: "text"}})
    if sc != seedapi.StrongMatch {
        t.Fatalf("expected StrongMatch, got %d", sc)
    }
}
```

Run `go test ./...` inside the generators directory.

---

## Development

```bash
make build      # ./bin/seed
go test ./...   # unit tests
```

## License

MIT.
