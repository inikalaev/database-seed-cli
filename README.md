# database-seed-cli

Go CLI for generating **relationally consistent** synthetic data for PostgreSQL databases.

Useful for: populating local development environments, preparing realistic datasets for **load testing**, and writing deterministic integration test fixtures.

> **Database support:** PostgreSQL only. MySQL, SQLite and others are planned for future releases.

Two explicit phases:

1. **Introspect → config.** `seed init --dsn <…> -o seed.yaml` reads the live schema and writes a YAML config where every column gets a `factory` (e.g. `pk_serial`, `email`, `fkref`). Columns that inference cannot classify are flagged `unresolved: true`.
2. **Config → SQL.** `seed generate -c seed.yaml -o seed.sql` walks the FK graph, generates rows in dependency order, and writes a single SQL script. You run it: `psql -f seed.sql`.

## Install

```bash
git clone https://github.com/inikalaev/database-seed-cli
cd database-seed-cli
make build            # produces ./bin/seed-cli
```

Requires Go 1.25+. The `--generators` flag additionally requires a Go toolchain at runtime (the CLI recompiles itself with your plugins).

## Quick start

```bash
# Create a fresh config from a live DB
./bin/seed-cli init --dsn postgres://user:pass@localhost/app -o seed.yaml

# Edit seed.yaml — set row_count, tweak factories/params, resolve `unresolved: true`

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

### Filtering tables

```bash
# include only specific tables
seed-cli init --dsn … --only users,orders -o seed.yaml
seed-cli init --dsn … --only public.users --only public.orders -o seed.yaml

# exclude tables you don't need
seed-cli init --dsn … --exclude ar_internal_metadata,schema_migrations -o seed.yaml
```

Both flags accept short form (`table`) or fully-qualified (`schema.table`), and can be comma-separated or repeated. `--only` and `--exclude` work on `init`, `sync`, and `introspect`.

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
- **Go plugins via a folder.** Drop `.go` files into a directory, pass `--generators ./dir`. The CLI recompiles itself with your factories and caches the binary under `$XDG_CACHE_HOME/seed-cli/<hash>`. MVP needs `SEED_CLI_SRC=<path to cli/>`.

---

## Config reference

```yaml
version: 1
database:
  dialect: postgres           # only postgres in MVP
  schemas: [public, billing]  # schemas covered by introspection
defaults:
  locale: ru_RU               # pool selection hint for name/address factories
  seed: 42                    # deterministic seed for generators (0 = nondeterministic)
tables:
  public.users:
    row_count: 1000
    tags: [core]
    columns:
      id:         { factory: pk_serial, data_type: integer }
      email:      { factory: email, params: { domain: acme.io }, data_type: text }
      first_name: { factory: first_name, data_type: text }
      metadata:   { factory: json_any, unresolved: true, data_type: jsonb }

  public.orders:
    # Per-parent expansion: for every row in users, insert 1..20 orders.
    row_count_per: { users: [1, 20] }
    columns:
      id:      { factory: pk_serial, data_type: integer }
      user_id: { factory: fkref, params: { target: public.users.id }, data_type: integer }
      total:   { factory: decimal, params: { min: 0, max: 10000 }, data_type: numeric }
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
| `columns.*.factory`          | inferred → user | yes if user edited  |
| `columns.*.params`             | inferred → user | yes if user edited  |
| `columns.*.unresolved`         | CLI             | re-evaluated        |
| `columns.*.data_type`          | CLI             | CLI rewrites        |
| `columns.*.nullable`           | CLI             | CLI rewrites        |

### Built-in factories

**Identity / keys**

| Factory       | Typical columns              | Notes |
|---------------|------------------------------|-------|
| `pk_serial`   | integer PK (`id`)            | Sequential from `params.start` (default 1) |
| `uuid`        | `uuid`                       | Version 4 |
| `fkref`       | any FK column                | `params.target: schema.table.column` |
| `enum_value`  | PG `USER-DEFINED` enum       | Chooses uniformly from enum labels |

**People / contact**

| Factory       | Typical columns              | Notes |
|---------------|------------------------------|-------|
| `first_name`, `last_name`, `full_name`, `patronymic` | name fields | Pool is locale-aware |
| `email`       | `email`                      | `params.domain: example.com` |
| `phone`       | `phone`, `mobile`            | |
| `username`    | `username`, `login`, `handle`| |
| `gender`      | `gender`, `sex`              | |

**Web / network**

| Factory       | Typical columns              | Notes |
|---------------|------------------------------|-------|
| `url`, `image_url` | `url`, `avatar`, `photo` | |
| `hostname`    | `host`, `domain`             | |
| `ip_address`  | `ip`, `ip_address`           | |
| `port`        | `port`                       | 1–65535 |
| `slug`        | `slug`, `permalink`          | URL-safe lowercase |
| `token`       | `token`, `secret`, `api_key` | Random hex |

**Location / locale**

| Factory       | Typical columns              | Notes |
|---------------|------------------------------|-------|
| `company`     | `company`, `organization`    | |
| `city`        | `city`                       | |
| `country`     | `country`, `country_code`    | ISO 3166-1 alpha-2 |
| `currency`    | `currency`, `currency_code`  | ISO 4217 |
| `language_code` | `language`, `locale`       | BCP 47 |
| `latitude`, `longitude` | `lat`, `lon`, `latitude`, `longitude` | |

**Content / media**

| Factory       | Typical columns              | Notes |
|---------------|------------------------------|-------|
| `title`       | `title`, `subject`, `heading`| Sentence-case phrase |
| `color`       | `color`, `bg_color`          | Hex `#rrggbb` |
| `filename`    | `filename`, `attachment`     | `file_<row>.ext` |
| `mime_type`   | `mime_type`, `content_type`  | |

**Numeric / temporal**

| Factory       | Typical columns              | Notes |
|---------------|------------------------------|-------|
| `integer`     | generic integer fallback     | `params.min`, `params.max`; generic `*_id` without FK → unresolved |
| `decimal`     | generic numeric/float        | `params.min`, `params.max` |
| `amount`      | `amount`, `price`, `cost`, `total` | Numeric; `params.min`, `params.max` |
| `percentage`  | `percent`, `score`, `rate`   | 0–100 |
| `counter`     | `count`, `total_count`       | Non-negative integer |
| `year`        | `year`, `birth_year`         | Realistic year range |
| `position`    | `position`, `rank`, `order`  | Positive integer |
| `level`       | `level`, `depth`, `tier`     | |
| `priority`    | `priority`                   | |
| `version_int` | `version`, `schema_version`  | |
| `version_str` | `semver`, `app_version`      | `x.y.z` |
| `file_size`   | `size`, `file_size`          | Bytes |
| `status_code` | `http_status`, `status_code` | HTTP status code |
| `duration`    | `duration`, `elapsed`        | Seconds |
| `checksum`    | `checksum`, `crc`, `hash`    | Hex string |
| `bool`        | boolean                      | generic; plugins with NameMatch win |
| `timestamp`, `date` | temporal columns       | named patterns (`_at`, `_on`, `_date`, `deadline`) → resolved; bare column → unresolved |
| `time_of_day` | `time`, `start_time`         | `HH:MM:SS` |
| `pg_interval` | `interval` PG type           | |
| `tstzrange`   | `tstzrange` PG type          | |

**Structured / binary**

| Factory       | Typical columns              | Notes |
|---------------|------------------------------|-------|
| `string`      | text fallback                | Pattern: `<column>_<row>`; unresolved |
| `json_any`    | json / jsonb                 | Emits `{"row": N}` — override for real schemas; unresolved |
| `localized_json` | jsonb with locale-keyed object | |
| `hstore`      | `hstore`                     | Empty map default |
| `bytea`       | `bytea`                      | Random bytes |
| `array`       | any array type               | `params.length` |
| `point`       | `point` PG type              | |

### `row_count_per`

Mapping of `parent_table` → `[lo, hi]`. The planner multiplies the parent's row count by the midpoint of the range:

```yaml
tables:
  public.orders:
    row_count_per: { users: [1, 20] }
```

With `public.users.row_count = 1000`, the planner produces `1000 * 10 = 10_000` orders. If both `row_count` and `row_count_per` are set, `row_count_per` wins.

### Literal value with `value`

Any column (or JSON field inside `values`) can be given a fixed literal instead of a factory. The same value is emitted for every row.

```yaml
columns:
  status:   { value: "active",  data_type: text }
  is_admin: { value: false,     data_type: boolean }
  version:  { value: 1,         data_type: integer }
  metadata:
    factory: json_any
    data_type: jsonb
    values:
      type:  { value: "user" }
      score: { factory: integer, params: { min: 1, max: 100 } }
```

Priority order: `value` → `values` (JSON shape) → `factory`.

### JSON shape with `values`

For `json` / `jsonb` columns, set `values` to define the object shape inline. Each key maps to a nested `ColumnSpec` with its own `factory` and optional `params`. Nesting is arbitrary.

```yaml
columns:
  metadata:
    factory: json_any
    data_type: jsonb
    values:
      name:  { factory: first_name }
      score: { factory: integer, params: { min: 1, max: 100 } }
      addr:
        factory: json_any
        values:
          city:    { factory: city }
          country: { factory: country }
```

When `values` is present the emitter builds the JSON object from those specs and ignores the column's own `factory`. The resulting SQL literal looks like `'{"addr":{"city":"Moscow","country":"RU"},"name":"Ivan","score":42}'`.

### FK cycles

`seed-cli validate` reports FK cycles. The emitter wraps the script in `SET CONSTRAINTS ALL DEFERRED`. Use `fkref` on both sides — the emitter samples PK values in plan order.

---

## Extending: custom factories

Convention: **one generator per file**. Builtins live under `internal/factories/<name>.go` with shared predicates in `helpers.go` and registration order in `factories.go` (`All()`). User plugins follow the same rule under their `--generators ./dir`.

```bash
export SEED_CLI_SRC=/path/to/seed-cli/cli
seed-cli generate -c seed.yaml -o seed.sql --generators ./seed-generators
```

On first use the CLI compiles an augmented binary (your generators + stock factories) and caches it under `~/.cache/seed-cli/<hash>`. Subsequent runs re-exec the cached binary.

See [`examples/custom-generators/sku.go`](examples/custom-generators/sku.go) for a complete reference plugin.

### Template

The minimal factory — just `Name()`, `Tags()`, `Generate()`. The registry auto-matches by `Name()` (StrongMatch) and each tag (NameMatch, substring, case-insensitive, underscores stripped).

```go
package seedgens

import (
    "fmt"

    "github.com/ivannikolaev/seed-cli/cli/pkg/seedapi"
)

type SKU struct{}

func (SKU) Name() string   { return "sku" }
func (SKU) Tags() []string { return []string{"article", "product_code"} }

func (SKU) Generate(ctx seedapi.GenContext) any {
    return fmt.Sprintf("SKU-%06d", ctx.Row+1)
}

func init() { seedapi.Register(SKU{}) }
```

For custom matching logic (type checks, regex, compound conditions) implement `seedapi.Matcher`:

```go
func (SKU) Match(ctx seedapi.MatchContext) seedapi.MatchScore {
    if ok, _ := regexp.MatchString(`(?i)^sku$|article|product_code`, ctx.Column.Name); ok {
        return seedapi.StrongMatch
    }
    return seedapi.NoMatch
}
```

### The `seedapi.Factory` contract

- **`Name()`** — unique key across all registered factories. Referenced in YAML: `factory: sku`. Also the primary auto-match pattern (exact, StrongMatch).
- **`Tags()`** — column-name patterns for auto-matching. Each tag is matched as a case-insensitive substring (underscores/hyphens stripped). A hit scores NameMatch (~70).
- **`Generate(ctx)`** — value to put in the row. Return `nil` for `NULL`. Use `ctx.Rng` for determinism; never touch `rand`-package globals.
- **`Match(ctx)`** *(optional, implement `seedapi.Matcher`)* — override auto-matching with custom scoring. Scale:
  - `0` — no match.
  - `~10` — weak fallback (`string`).
  - `~40` — type-only match (bare `timestamp`, orphan `*_id`, enum-like `status`/`type`).
  - `~60` — WeakNameMatch: generic type with a known default (bool, date, decimal, hstore, timestamp by name pattern). Resolved by default; any plugin returning NameMatch wins.
  - `~70` — probable name match (named factories, Tags() hit).
  - `~90` — strong match (specific name/type/unique tokens).
  - `100` — FK references (never override unless the user says so).

### FK pool access

`ctx.FKPool.Pick(schema, table, column, ctx.Rng)` samples previously-generated PK values for a referenced table. Use this for composite or conditional FK generators.

### Testing plugins

Normal Go unit tests next to the factory file (same `seedgens` package):

```go
func TestSKUGenerates(t *testing.T) {
    rng := rand.New(rand.NewPCG(1, 1))
    v := SKU{}.Generate(seedapi.GenContext{Row: 0, Rng: rng})
    if v != "SKU-000001" {
        t.Fatalf("unexpected %v", v)
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

## Contributing

Issues, PRs, and feedback are very welcome. The codebase was largely written with AI assistance (Claude Code), so there are likely rough edges — bug reports and code reviews are especially appreciated.

If you're adding a factory: one file per factory in `internal/factories/`, implement `seedapi.Factory` (optionally `seedapi.Matcher`), register in `All()`, add a test case in `match_test.go`.

## License

MIT.
