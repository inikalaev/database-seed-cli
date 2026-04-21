# Contributing

Issues, bug reports, and pull requests are welcome.

## Getting started

```bash
git clone https://github.com/inikalaev/database-seed-cli.git
cd database-seed-cli
go build ./...
go test ./...
```

## Adding a factory

One file per factory in `internal/factories/<name>.go`:

1. Implement `seedapi.Factory` (required: `Name()`, `Tags()`, `Generate()`).
2. Optionally implement `seedapi.Matcher` to override auto-matching.
3. Register in `internal/factories/factories.go` → `All()` list.
4. Add a test case in `internal/factories/match_test.go`.

See any existing factory (e.g. `email.go`) as a reference.

## Running the linter

```bash
golangci-lint run
```

Config is in `.golangci.yml`. CI runs the same linter on every PR.

## Pull request checklist

- [ ] `go build ./...` passes
- [ ] `go test ./...` passes (including `-race`)
- [ ] New factories have a `match_test.go` entry
- [ ] Public API changes are reflected in the README

## Code style

- Standard `gofmt` / `goimports`.
- No comments explaining what the code does — only why (non-obvious invariants, workarounds).
- No error handling for scenarios that can't happen.
