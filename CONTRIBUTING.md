# Contributing to

Thanks for your interest in improving `tfquery`. All contributions are welcome:

- Bug reports & fixes
- New features & enhancements
- Documentation & examples
- Refactors & performance improvements
- Feedback, ideas, and testing

---
## Quick Start

1. Fork the repo and clone your fork
2. Create a branch: `git switch -c feature/short-descriptor`
3. Make changes (see Style & Conventions below)
4. Run build & basic smoke checks: `go build ./...`
5. Open a Pull Request (PR) with a clear title & description
6. Respond to review feedback

---
## Development Environment

Requirements:
- Go (see the version badge in `README.md` – use that or newer minor)
- GNU Make (optional if you add helper targets later)
- Access to Terraform Cloud / Enterprise (for remote backend integration tests, optional)

Recommended tooling (optional but helpful):
- `golangci-lint` for static analysis
- `gofumpt` or `go fmt` (the repo follows standard formatting)
- `git-cliff` or conventional commit helpers for generating changelogs (future)

---
## Project Structure Overview

```
internal/
  attrs/        Attribute parsing, transforms & list management
  backend/      Backend resolution & implementations (cloud, remote, s3, local)
  command/      CLI command builders & shared helpers
  config/       Configuration loading and accessors
  differ/       State diff helpers
  driller/      JSON drilling / extraction support
  meta/         Runtime metadata container
  output/       Filtering, sorting, rendering (table, json, yaml)
  state/        State loading & (optional) decryption
  util/         Generic utilities (root dir parsing, etc.)
  version/      Version metadata
cmd/ (if added later)  Thin entrypoint(s) (currently top-level main.go)
```

---
## Code Style & Conventions

### General
- Follow standard Go idioms; prefer clarity over cleverness.
- Keep functions focused; extract helpers once duplication appears 3+ times.
- Avoid premature abstraction – let patterns stabilize first.

### Formatting & Lint
- Run `go fmt ./...` (CI will fail if formatting diverges).
- New comments: single space after periods; wrap at ≤ 80 cols when practical.
- Use American English spelling (e.g., “behavior”, “initialization”).

### Comments & Documentation
- All exported identifiers MUST have GoDoc.
- Package-level docs live in `doc.go` (already present for internal packages).
- Use `TODO`, `FIXME`, `NOTE`, `THINK` prefixes deliberately:
  - `TODO`: Concrete planned work.
  - `FIXME`: Known bug needing correction.
  - `THINK`: Open design question / speculative consideration.
- Remove stale THINK/TODO blocks when resolved.

### Errors
- Wrap errors with context using `fmt.Errorf("context: %w", err)`.
- Sentinel errors (`var ErrX = errors.New(...)`) only when callers need `errors.Is` / `errors.As`.
- Do not log AND return an error unless it adds distinct value; prefer one or the other.

### Logging
- Use `apex/log` consistently (already in project).
- Debug logs should aid diagnosis without leaking secrets.
- Avoid debug spam inside hot loops unless guarded by coarse checks.

### Flags & CLI Patterns
- New commands should reuse helpers in `internal/command/common.go`.
- Always support `--tldr` (short usage examples) when adding a new query-like command.
- Prefer composability over bespoke one-off flags when possible.

### Output & Filtering
- Keep filtering semantics in `internal/output`; avoid leaking parsing logic into commands.
- If adding new filter operands, update:
  - Parser regex
  - Docs (`docs/filters.md`)
  - Tests (add new cases)

### State & Backends
- Avoid adding backend-specific conditionals in generic layers; extend backend interfaces instead.
- S3 / remote / cloud / local should converge on consistent method semantics (already mostly aligned).

---
## Commit Messages

Format (inspired by Conventional Commits, relaxed):

```
<type>(optional-scope): short summary

Longer body explaining intent, rationale, side effects.

Refs: #123 (optional)
```

Types (suggested):
- bug, chore, docs, feat, refactor, revert, test.

Examples:
```
feat(backend): add s3 version filtering
fix(output): correct case-insensitive contains operand
```

---
## Branching

- `main` (or `master`): always buildable; releases cut from it.
- Feature branches: `feat/...`
- Fix branches: `bug/...`
- Docs-only: `docs/...`
- Avoid long-lived branches; rebase over merge when keeping a feature branch up-to-date.

---
## Testing

_Current test coverage is minimal (if present). You can help by adding tests._

Guidelines:
- Put table-driven tests alongside the code in `_test.go` files.
- Cover happy-path plus at least one edge case per function of interest.
- Avoid brittle tests tied to implementation details; test observable behavior.
- For backend-dependent logic, use small golden fixtures or mockable interfaces.

Suggested future test areas:
- Filter parsing & evaluation
- Attr transformation chaining
- Backend selection logic (`NewBackend`) for matrix of root dir states
- State version diff selection

---
## Performance Considerations

- Only optimize after profiling (`pprof`, `benchstat`).
- Be cautious with large allocations in tight loops (e.g., filtering + transforms).
- Use streaming / iterative decoding only if a real-world dataset size justifies it.

---
## Documentation Updates

When adding / changing functionality:
- Update `README.md` if user-facing behavior changes.
- Add / adjust docs under `docs/` (filters, flags, attrs, quickstart, etc.).
- Add a TLDR example if a command gains a notable new pattern.

---
## Release Process (Overview)

1. Ensure version bump in `internal/version` (if present) or tagging pipeline handles it.
2. Confirm changelog section (future automation TBD).
3. Tag: `git tag -a vX.Y.Z -m "Release vX.Y.Z" && git push origin vX.Y.Z`
4. CI / GoReleaser publishes artifacts & signatures.
5. Manually verify signatures using `KEYS` file.

---
## Security

- Report sensitive security issues privately (email in README) before filing a public issue.
- Do NOT include secrets in logs, errors, or test fixtures.
- Validate user input (especially filter specs or dynamic paths) before usage.

---
## Style Checklist (Pre-PR)

Run through quickly:
- [ ] go build ./... succeeds
- [ ] Added / updated GoDoc for new exported symbols
- [ ] No lingering double-space sentences in new comments
- [ ] Flags / command wiring reuse `common.go` helpers
- [ ] Added / updated docs & examples where applicable
- [ ] Any new filter operand documented & tested
- [ ] No debug leftovers or commented-out code blocks

---
## Good First Issues

Look for labels such as:
- `good first issue`
- `help wanted`
- `docs`

If something lacks context, ask before investing heavy effort.

---
## Communication

- Use GitHub Issues for bugs / features.
- Use PR discussions for implementation-level feedback.
- Keep discussions technical and respectful.

---
## Attribution & Trademarks

Terraform, Terraform Enterprise, and HCP Terraform are trademarks of HashiCorp, Inc. OpenTofu is a trademark of The Linux Foundation.

---
## Thank You
---
## Git hooks for doc generation (optional)

If you don’t want to use a Makefile, you can enable a versioned Git pre-commit hook included in this repo that automatically regenerates man and TLDR pages from the canonical Markdown docs when you commit.

What it does on each commit:
- Builds the tiny generator at `tools/docgen`.
- Generates `docs/man/share/man1/*.1` and `docs/tldr/*.md` from `docs/commands/*.md`.
- Stages changed generated files so they’re included in the commit.

Enable it once per clone:

```sh
git config core.hooksPath .githooks
```

Disable:

```sh
git config --unset core.hooksPath
```

Run generator manually if needed:

```sh
go run tools/docgen/main.go -root .
```


Your contributions help make infrastructure automation easier. Thank you for helping improve ``.
