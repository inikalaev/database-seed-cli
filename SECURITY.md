# Security

## Scope

`database-seed-cli` is a local developer tool. It connects to PostgreSQL via a DSN you supply, reads the schema, and emits a SQL file. It does not run as a server, does not handle HTTP requests, and does not store credentials.

## Reporting a vulnerability

If you find a security issue (e.g. SQL injection in emitted scripts, credential leakage, path traversal in `--factories`), please **do not open a public issue**. Email `ivan_n6_20@icloud.com` with:

- A description of the vulnerability.
- Steps to reproduce.
- Potential impact.

You'll receive a response within 72 hours. Once a fix is released, the vulnerability will be disclosed publicly with credit to the reporter.
