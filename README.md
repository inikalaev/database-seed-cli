# database-seed-cli

[![CI](https://github.com/inikalaev/database-seed-cli/actions/workflows/ci.yml/badge.svg)](https://github.com/inikalaev/database-seed-cli/actions/workflows/ci.yml)
[![Release](https://github.com/inikalaev/database-seed-cli/actions/workflows/release.yml/badge.svg)](https://github.com/inikalaev/database-seed-cli/actions/workflows/release.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/inikalaev/database-seed-cli)](https://goreportcard.com/report/github.com/inikalaev/database-seed-cli)
[![Go Reference](https://pkg.go.dev/badge/github.com/inikalaev/database-seed-cli.svg)](https://pkg.go.dev/github.com/inikalaev/database-seed-cli)
[![License: MIT](https://img.shields.io/badge/License-MIT-blue.svg)](LICENSE)

Generate **relationally consistent** synthetic data for PostgreSQL — reads your live schema and emits a ready-to-run SQL seed script. FK constraints, cycles, and row counts are all handled automatically.

![demo](demo/demo.gif)

```
$ seed-cli init --dsn postgres://localhost/myapp -o seed.yaml
$ seed-cli generate -c seed.yaml -o seed.sql
$ psql $DATABASE_URL -f seed.sql
Seeded 1 000 users · 10 234 orders · 42 817 events in 0.3s
```

Two phases, zero manual FK wiring:

1. **Introspect → config.** `seed-cli init` reads the live schema and writes a YAML config where every column gets a `factory` (`pk_serial`, `email`, `fkref`, …). Columns inference can't classify are flagged `unresolved: true` so you know exactly what to review.
2. **Config → SQL.** `seed-cli generate` walks the FK graph, inserts in dependency order, and writes a single SQL script.

> **Database support:** PostgreSQL only. MySQL, SQLite, and others are planned.

## Install

```bash
# Go toolchain (recommended — single binary, stays current)
go install github.com/inikalaev/database-seed-cli/cmd/seed-cli@latest

# Build from source
git clone https://github.com/inikalaev/database-seed-cli.git
cd database-seed-cli
go install ./cmd/seed-cli
```

Requires Go 1.25+. The `--factories` flag additionally requires a Go toolchain at runtime (the CLI recompiles itself with your plugins).

## Quick start

```bash
# Create a fresh config from a live DB
seed-cli init --dsn postgres://user:pass@localhost/app -o seed.yaml

# Edit seed.yaml — set row_count, tweak factories/params, resolve `unresolved: true`

# Re-introspect after schema changes — user edits are preserved
seed-cli sync --dsn postgres://user:pass@localhost/app -c seed.yaml

# Lint the config
seed-cli validate -c seed.yaml

# Emit SQL and apply
seed-cli generate -c seed.yaml -o seed.sql
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
| `seed-cli fix`            | Interactively walk through `validate` findings and apply fixes. |
| `seed-cli generate`       | Read config → emit SQL file.                                   |

## Diagnostics: reading `validate` output

`seed-cli validate` prints issues at three severity levels:

- `ERR` — `generate` will likely fail or produce SQL that errors on apply. Fix before generating.
- `WARN` — likely failure on apply or correctness issue (duplicates, NULL in NOT NULL). Resolve before running against real data.
- `INFO` — constraint the generator cannot automate (composite UNIQUE/FK, CHECK, EXCLUDE, partial UNIQUE). A reminder of manual responsibility.

Output ends with a summary `N error(s) · M warning(s) · K info` and a hint to run `seed-cli fix` for auto-fixable issues.

### All issue kinds

`→` marks issues that `seed-cli fix` can resolve interactively. The rest require manual config edits.

| Kind | Lv | Meaning | How to fix | `fix` |
|------|----|---------|------------|-------|
| `unresolved` | WARN | Inference produced a low-confidence match (score < WeakNameMatch). | Set `factory:` explicitly, or leave as-is if the fallback is acceptable. | ✓ |
| `no-factory` | ERR | Column has no `factory:`, `value:`, or `values:` set. | Add `factory: <name>` or `value: <literal>`. | ✓ |
| `unknown-factory` | ERR | `factory:` names an unregistered factory (typo or missing plugin). | Fix the name or wire the plugin via `--factories`. | ✓ |
| `missing-factory-param` | WARN | The chosen factory declares a required parameter via `seedapi.Configurable` but it isn't set (e.g. `EnumValueStr` with no `params.values`). | Run `seed-cli fix` to enter the value — the cascade prompts for every required param in one session. | ✓ |
| `json-field-unresolved` | WARN | A field inside a `values:` JSON shape has `unresolved: true` — no confident factory found. | Run `seed-cli fix` to pick a factory for that field. | ✓ |
| `value-type-mismatch` | ERR | Literal in `value:` is incompatible with `data_type`. | Replace with a compatible literal or remove `value:`. | ✓ |
| `fkref-missing-target` | ERR | `factory: fkref` but `params.target` is empty. | Add `params: { target: schema.table.column }`. | ✓ |
| `fkref-target-not-found` | ERR | `target` points to a column/table not in the config. | Fix the path to a real PK column. | ✓ |
| `row-count-per-missing` | ERR | A key in `row_count_per` references a table not in the config. | Remove the key or rename it to an existing table. | ✓ |
| `fkref-empty-pool` | ERR | NOT NULL fkref targets a table with `row_count: 0` — pool is empty. | Raise parent `row_count`, set `nullable: true`, or add `value:`. | ✓ |
| `fkref-in-cycle` | ERR | NOT NULL fkref is in an FK cycle; first emission will produce NULL in a NOT NULL column. | Set `nullable: true` or `value:`. | ✓ |
| `unique-unsafe-factory` | WARN | Column has UNIQUE but factory doesn't guarantee uniqueness (e.g. `string`). | Switch to `uuid`/`pk_serial`/`token`, or accept the risk. | ✓ |
| `composite-unique` | INFO | Composite UNIQUE across 2+ columns. Uniqueness of tuples can't be checked automatically. | Ensure the combination of factories produces unique tuples; sometimes needs a custom correlated generator. | – |
| `composite-fk` | WARN | Multiple fkref columns reference different columns of the same parent — likely a composite FK. | fkref samples columns independently; tuple consistency isn't guaranteed. Write a custom generator that reads all related fields from one parent row. | – |
| `deferrable-cycle` | INFO | FK cycle detected, but all edges are DEFERRABLE. The emitter wraps the script in `SET CONSTRAINTS ALL DEFERRED`. | No action required. | – |
| `non-deferrable-cycle` | ERR | FK cycle with at least one non-deferrable edge. `SET CONSTRAINTS` won't help — apply will fail. | Make the edge DEFERRABLE in the DB (`ALTER TABLE ... INITIALLY DEFERRED`) or allow NULL to break the cycle. | – |
| `check-not-applied` | INFO | Column has a CHECK the parser couldn't auto-translate to `params` (multi-column or complex expression). | Manually set `params: { min, max, values, max_len }` so the factory respects the CHECK. | – |
| `exclude` | WARN | EXCLUDE constraint (e.g. overlap-prevention on `tstzrange`). Generator cannot satisfy it. | Options: set `row_count: 0`, write a custom paired generator, or accept that apply may fail. | – |
| `partial-unique` | INFO | UNIQUE with a `WHERE` clause (e.g. soft-delete `WHERE deleted_at IS NULL`). Filter isn't applied during generation. | Ensure generated data won't violate the filtered UNIQUE — typically just need distinct keys in matching rows. | – |

### Interactive fixing: `seed-cli fix`

```bash
seed-cli fix -c seed.yaml
```

Walks through all auto-fixable issues (✓ above), prompting for a resolution on each. After each accepted fix the config is **written to disk immediately** — Ctrl+C is safe at any point; completed fixes persist, and the next run picks up where you left off.

Flags:
- `-c, --config` — path to YAML (default `seed.yaml`).
- `--dry-run` — walk through prompts without writing changes. Useful to preview what `fix` would do.

Note: saving re-orders columns alphabetically and drops comments, same as `sync`.

Example session:
```
Found 3 fixable issue(s). Ctrl+C at any time — your edits are saved after each fix.

[1/3] WARN  public.orders.status  factory EnumValueStr requires values
? Allowed values (comma-separated):
  > enter value
    change factory instead
? Allowed values (comma-separated): pending,active,cancelled
  ✓ applied

[2/3] WARN  public.products.sku  unresolved
? Pick a factory: skip for now
  skipped

[3/3] WARN  public.users.metadata  unresolved
? Example JSON for public.users.metadata: {"plan":"pro","score":42,"first_name":"Alice"}

  ✓ first_name        first_name
  ? plan              string  (unresolved)
  ✓ score             decimal

? Factory for json field "plan": EnumValueStr  (score 70)
? Allowed values (comma-separated):
  > enter value
    change factory instead
? Allowed values (comma-separated): free,pro,enterprise
  ✓ applied

2 fixed · 1 skipped
```

Whenever `fix` writes a `factory:`, it immediately asks the factory whether it needs further setup. Each required parameter is prompted in the same session, and the user can pick `change factory instead` at any step to swap the factory without leaving the cascade.

After a session, run `seed-cli validate` to see only the remaining non-auto issues and anything you explicitly skipped.

#### `EnumValueStr` — text columns with enum semantics

Columns named `status`, `type`, `*_status`, or `*_type` (TEXT) are matched by `EnumValueStr`. The allowed values aren't derivable from the schema, so the factory declares `values` as a required param via `seedapi.Configurable`:

```yaml
status:
  factory: EnumValueStr
  params:
    values: [pending, active, cancelled]
  data_type: text
```

`fix` collects this list inline via the cascade — either as part of picking the factory, or when re-entering `fix` after `validate` has flagged `missing-factory-param`.

#### JSONB shape inference

For `jsonb` / `json` columns, `fix` asks for an example object (single line). It then infers a factory for each field by name and value type, prints the result, and **immediately prompts inline** for any fields it couldn't classify — no second pass required. The cascade runs on every picked factory, so a field that becomes `EnumValueStr` then asks for its `values` right there.

```
? Example JSON for public.users.metadata: {"plan":"pro","score":42,"first_name":"Alice"}

  ✓ first_name        first_name
  ? plan              string  (unresolved)
  ✓ score             decimal

? Factory for json field "plan": EnumValueStr  (score 70)
? Allowed values (comma-separated): free,pro,enterprise
```

The resulting config:

```yaml
metadata:
  data_type: jsonb
  values:
    first_name: { factory: first_name, data_type: text }
    plan:        { factory: EnumValueStr, params: { values: [free, pro, enterprise] }, data_type: text }
    score:       { factory: decimal, data_type: numeric }
```

Fields explicitly skipped during the inline prompt keep `unresolved: true` and appear in the next `validate` run as `json-field-unresolved`. Nesting is arbitrary: a field can have its own `values:` map for nested objects.

#### Writing a `Configurable` factory

If your factory needs parameters that only the user can supply, implement the optional `seedapi.Configurable` interface:

```go
type Configurable interface {
    RequiredSetup(params map[string]any) []SetupStep
}
```

`RequiredSetup` is called once in `validate` and repeatedly in `fix` (after every value the user enters). Return an empty slice when the factory is fully configured for the given params; return one or more `SetupStep`s describing what the CLI should prompt for.

##### `SetupStep` fields

| Field      | Meaning |
|------------|---------|
| `ParamKey` | Key under `col.Params` where the accepted value is written. The factory reads the same key in `Generate`. |
| `Kind`     | Prompt type: `SetupString`, `SetupInt`, `SetupFloat`, `SetupBool`, `SetupList`. Determines both UI and the Go type stored in `col.Params`. |
| `Element`  | Only for `SetupList`. Describes the element kind (one level deep). |
| `Prompt`   | User-facing message. Short, actionable, e.g. `"Allowed values (comma-separated):"`. |
| `Help`     | Optional help text shown when the user presses `?`. |
| `Required` | `true` → no skip option; user must enter a value or change factory. `false` → `skip (use default)` is offered, and the factory must cope with the param being absent. |

##### Stored value types per `SetupKind`

| Kind          | `col.Params[ParamKey]` type     |
|---------------|---------------------------------|
| `SetupString` | `string` (TrimSpace'd)           |
| `SetupInt`    | `int`                           |
| `SetupFloat`  | `float64`                       |
| `SetupBool`   | `bool`                          |
| `SetupList`   | `[]any` where each element has the Go type produced by `Element.Kind` |

##### Examples

**One required param.** Mirrors the built-in `EnumValueStr`:

```go
func (myEnum) RequiredSetup(p map[string]any) []seedapi.SetupStep {
    if v, ok := p["values"].([]any); ok && len(v) > 0 {
        return nil
    }
    return []seedapi.SetupStep{{
        ParamKey: "values",
        Kind:     seedapi.SetupList,
        Element:  &seedapi.SetupStep{Kind: seedapi.SetupString},
        Prompt:   "Allowed values (comma-separated):",
        Help:     "e.g. draft,published,archived",
        Required: true,
    }}
}
```

**Optional param with a default.** Blank input stays absent; the factory picks the default at generation time:

```go
func (myBytea) RequiredSetup(p map[string]any) []seedapi.SetupStep {
    if _, ok := p["size"].(int); ok {
        return nil
    }
    return []seedapi.SetupStep{{
        ParamKey: "size",
        Kind:     seedapi.SetupInt,
        Prompt:   "Byte size (blank = default 16):",
        Required: false,
    }}
}
```

**Two required params, asked sequentially.** The cascade handles both in order and re-evaluates between them:

```go
func (myRange) RequiredSetup(p map[string]any) []seedapi.SetupStep {
    var steps []seedapi.SetupStep
    if _, ok := p["min"].(float64); !ok {
        steps = append(steps, seedapi.SetupStep{
            ParamKey: "min", Kind: seedapi.SetupFloat,
            Prompt: "Min:", Required: true,
        })
    }
    if _, ok := p["max"].(float64); !ok {
        steps = append(steps, seedapi.SetupStep{
            ParamKey: "max", Kind: seedapi.SetupFloat,
            Prompt: "Max:", Required: true,
        })
    }
    return steps
}
```

**Branching on params.** First prompt gates the second; if the user fills `template`, the `values` path is never shown:

```go
func (myJson) RequiredSetup(p map[string]any) []seedapi.SetupStep {
    if _, ok := p["template"].(string); ok {
        return nil
    }
    if _, ok := p["values"].([]any); ok {
        return nil
    }
    return []seedapi.SetupStep{{
        ParamKey: "template",
        Kind:     seedapi.SetupString,
        Prompt:   "JSON template (blank to fall back to values list):",
        Required: false,
    }}
}
```

**Complex types the CLI can't prompt directly.** Maps, nested arrays, or arbitrary JSON → take a raw string and parse it yourself:

```go
func (weightedEnum) RequiredSetup(p map[string]any) []seedapi.SetupStep {
    if v, ok := p["slots"].(map[string]any); ok && len(v) > 0 {
        return nil
    }
    return []seedapi.SetupStep{{
        ParamKey: "slots",
        Kind:     seedapi.SetupString, // parsed to map[string]int in Generate
        Prompt:   "Slots as JSON object:",
        Help:     `e.g. {"pending":1,"active":3,"cancelled":1}`,
        Required: true,
    }}
}
```

##### Contract

- `RequiredSetup` must be a pure function of `params`.
- After the CLI writes `params[ParamKey]` with an accepted value, your next `RequiredSetup(params)` call must not return the same `ParamKey` again, otherwise the cascade would loop forever.
- The CLI caps cascade iterations at 16 as a safety net — it's an emergency stop, not a contract. Buggy factories still get caught by validate downstream.

##### Interaction with `fix`

At every prompt the CLI offers three options (two when the step is required):

1. **`enter value`** — collect the value per `Kind` and write into `col.Params[ParamKey]`.
2. **`change factory instead`** — delegate back to the factory-picker. The cascade then restarts on the newly chosen factory. Previously stored params stay as-is; the new factory either uses them or ignores them.
3. **`skip (use default)`** — only shown when `Required: false`. Leaves the param absent.

### Not auto-fixable

`composite-unique`, `composite-fk`, `check-not-applied`, `exclude`, `partial-unique`, `deferrable-cycle`, and `non-deferrable-cycle` require manual config edits or schema changes. They stay in `validate` output as persistent reminders — `fix` never prompts for them.

Typical strategies:
- **Composite constraints** — write a custom generator that sees all dependent columns at once (see *Extending: custom factories*).
- **CHECK/EXCLUDE** — tighten generator `params` to the range the constraint allows.
- **Non-deferrable cycle** — fix the DB schema; there's no way to seed around it.

---

## Design invariants

- **Idempotent merge.** `seed-cli sync` preserves every user edit; schema-derived fields refresh; removed tables/columns are flagged `removed: true`, not deleted.
- **Unresolved marking.** Inference never silently guesses — columns it cannot classify land in the config with `unresolved: true`.
- **Shared relation graph.** FK topology and insert order live in `internal/relations`; one source of truth for CLI and consumers.
- **Go plugins via a folder.** Drop `.go` files into a directory, pass `--factories ./dir`. The CLI recompiles itself with your factories and caches the binary under `$XDG_CACHE_HOME/seed-cli/<hash>`. Requires `SEED_CLI_SRC` pointing at this repository root (the directory containing `go.mod`).

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
| `EnumValueStr` | `status`, `type`, `*_status`, `*_type` (TEXT) | Text enum — set `params.values` or use `fix` |

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

Convention: **one generator per file**. Builtins live under `internal/factories/<name>.go` with shared predicates in `helpers.go` and registration order in `factories.go` (`All()`). User plugins follow the same rule under their `--factories ./dir`.

```bash
export SEED_CLI_SRC=$(pwd)   # root of this repo (directory containing go.mod)
seed-cli generate -c seed.yaml -o seed.sql --factories ./seed-factories
```

On first use the CLI compiles an augmented binary (your factories + stock factories) and caches it under `~/.cache/seed-cli/<hash>`. Subsequent runs re-exec the cached binary.

See [`examples/custom-factories/sku.go`](examples/custom-factories/sku.go) for a complete reference plugin.

### Template

The minimal factory — just `Name()`, `Tags()`, `Generate()`. The registry auto-matches by `Name()` (StrongMatch) and each tag (NameMatch, substring, case-insensitive, underscores stripped).

```go
package seedgens

import (
    "fmt"

    "github.com/inikalaev/database-seed-cli/pkg/seedapi"
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

Run `go test ./...` inside the factories directory.

---

## Development

```bash
go install ./cmd/seed-cli
go test ./...
```

### Testing strategy

Tests are organized by layer. Each package owns the tests for its own responsibilities; integration between layers is covered end-to-end in the packages where integration actually happens.

| Package | What is tested | How |
|---------|---------------|-----|
| `internal/validate` | Every `Check()` issue kind (unresolved, no-factory, fkref-*, unique, CHECK, EXCLUDE, …), `Counts`/`HasErrors`/`HasFixable`, `Kind.String()`, `UniqueSafeFactory` | Unit — minimal `*config.Config` values built inline |
| `internal/factories` | `Generate()` returns a non-nil value of the right type for every built-in factory; `EnumValueStr`/`EnumValue.RequiredSetup` contract; boundary invariants (min/max inclusive, no overflow) | Unit + table-driven smoke |
| `internal/config` | `Marshal/Unmarshal` round-trip, `Save/Load` via temp file, version warning callback, `Merge` idempotency (user edits survive re-sync), CHECK-constraint param propagation, migration scaffold | Unit |
| `internal/sqlemit` | Full SQL output pinned to golden files in `testdata/` — regenerate with `go test -update` | Golden-file regression |
| `internal/relations` | FK graph construction, topological order, cycle detection (Tarjan SCC), composite-FK grouping | Unit |
| `internal/registry` | Factory lookup, deduplication, tag-based inference ranking | Unit |
| `internal/configbuild` | `FromModel` column classification, default row count, enum inference | Unit |
| `cmd/seed-cli/cli` | Pure helper functions only (`lookupJsonField`, `parseListInput`); interactive survey flows are not unit-tested | Unit |

**What is not tested:** `buildplugins` (requires a full Go toolchain and disk writes at runtime — covered by the manual demo) and `pkg/seedapi` (only interface declarations).

To view coverage per package:

```bash
go test -coverprofile=cover.out ./...
go tool cover -func=cover.out | grep -v "100.0%"
```

## Contributing

Issues, PRs, and feedback are very welcome. Parts of this codebase were written with AI assistance, so bug reports and code reviews are especially appreciated.

If you're adding a factory: one file per factory in `internal/factories/`, implement `seedapi.Factory` (optionally `seedapi.Matcher`), register in `All()`, add a test case in `match_test.go`.

## License

MIT.
