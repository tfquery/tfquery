# Contributing to tfquery

Thanks for your interest in improving `tfquery`.

This guide is PR-focused. It describes how to propose changes and how reviews
are handled. It intentionally does not document release or maintainer workflow.

## Governance and Merge Policy

- `main` is maintainer-governed.
- All contributions must come through a Pull Request.
- Direct pushes to `main` are not accepted.
- Maintainer approval is required before merge.
- Only maintainers merge PRs.
- Only maintainers decide release timing and release inclusion.

If there is uncertainty, maintainer guidance is final.

## PR Workflow

1. Fork the repository.
2. Create a branch for your change.
3. Make a focused change with tests and docs as needed.
4. Open a PR with a clear summary, rationale, and scope.
5. Respond to review feedback and update the PR.

Keep PRs small and reviewable when possible.

## PR Expectations

### Scope

- Solve one problem per PR when practical.
- Avoid unrelated refactors in the same PR.
- Keep behavior changes explicit in the PR description.

### Code Quality

- Follow idiomatic Go and existing project patterns.
- Keep functions cohesive and readable.
- Add or update tests when behavior changes.
- Add or update docs when user-facing behavior changes.
- Include GoDoc comments for exported identifiers.

### CLI and Query Behavior

- Reuse helpers in `internal/command/common.go` for command wiring.
- Keep filter behavior in `internal/output`.
- For new filter operands, update parser behavior, tests, and docs.

### Error and Logging Practices

- Wrap errors with contextual messages.
- Avoid duplicate error signaling (log and return) unless both add value.
- Keep logs useful and avoid secret exposure.

## Commit and Branch Conventions

- Prefer concise, descriptive commits.
- Suggested commit types: `bug`, `chore`, `docs`, `feat`, `refactor`, `revert`, `test`.
- Suggested branch prefixes: `feat/...`, `bug/...`, `docs/...`.

Examples:

- `feat(backend): add s3 version filtering`
- `fix(output): correct case-insensitive contains operand`

## Review Checklist (Before Requesting Review)

- [ ] Change is focused and explained in the PR body.
- [ ] Tests are added/updated where behavior changed.
- [ ] User-facing docs are updated where applicable.
- [ ] Exported symbols include GoDoc comments.
- [ ] No debug leftovers or dead code.

## Security Reporting

- Report sensitive security issues privately (see README contact details) before opening a public issue.
- Do not include secrets in logs, examples, fixtures, or screenshots.

## Communication

- Use GitHub Issues for bugs and feature discussions.
- Use PR discussions for implementation details.
- Keep feedback technical, direct, and respectful.

## Attribution and Trademarks

Terraform, Terraform Enterprise, and HCP Terraform are trademarks of
HashiCorp, Inc. OpenTofu is a trademark of The Linux Foundation.

Thanks for contributing.
