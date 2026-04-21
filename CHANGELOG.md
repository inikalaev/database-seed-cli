# Changelog

All notable changes to this project will be documented in this file.

The format follows [Keep a Changelog](https://keepachangelog.com/en/1.1.0/).
This project uses [Semantic Versioning](https://semver.org/).

## [Unreleased]

## [0.1.0] - 2025-04-21

### Added

- `seed-cli init` — introspect a live PostgreSQL schema and emit a YAML config with inferred factories.
- `seed-cli sync` — re-introspect after schema changes; user edits (factories, row counts, params) are preserved (idempotent merge).
- `seed-cli generate` — consume config, resolve FK order via Tarjan SCC, emit a SQL seed script.
- `seed-cli validate` — report unresolved columns, unknown factories, FK cycle issues, UNIQUE safety warnings, and constraint hints (17 issue kinds at ERR / WARN / INFO levels).
- `seed-cli fix` — interactive walk-through of auto-fixable issues with per-fix persistence (Ctrl+C safe).
- `seed-cli introspect` — print raw schema JSON (debug / tooling).
- 55+ built-in factories: people, web/network, location, numeric, temporal, structured types.
- FK cycle detection and breaking via deferred constraints.
- `row_count_per` for per-parent row expansion.
- Go plugin system via `--factories ./dir` (compiles an augmented binary, cached by content hash).
- `pkg/seedapi` public contract for user-written factories.
