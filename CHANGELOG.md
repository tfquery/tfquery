# Changelog

## v2.0.0 - 2026-06-17

### Added

- Name changed to `tfq` to avoid confusion with hashicorp/tfctl-cli.

### Fixed

- Absolute IAC root dir paths failed on Windows.

### Chores

- Lint cleanup.

## v1.5.1 - 2026-06-09

### Fixed

- Eliminated double read of state files when compiling SV candidates list.

### Chores

- Added backend_local tests.
- Corrected sq docs.
- Lint cleanup.

## v1.5.0 - 2026-06-09

### Added

- Added aggregated states for --sq. Multiple IAC roots can now be specified and the results of each will be aggregated into one result set.  For example - `tfctl sq ./iacdir1 ./iacdir2`.

### Chores

- Lint cleanup.

## v1.4.0 - 2026-06-01

### Added

- `--filter` now includes existence `?` operator.
- `--jq` added to support [jq filter language](https://jqlang.org/manual/#basic-filters).

### Chores

- Copyright text in release template now de-emphasized.
- Lint cleanup.

## v1.3.0 - 2026-05-29

### Added

- Reimplemented user-defined and default attr sets.
- Reimplemented user-defined filter sets.

### Fixed

- Fixed bug where attribute transformations were ignored on non-time fields.

### Chores

- Retired tools/check.sh and release.sh in favor of make targets.
- Lint cleanup.

## v1.2.3 - 2026-05-25

### Added

- S3 backend now has increased compatibility with more S3 implementations (MiniStack, Minio, etc).
- `0` and `false` are no longer treated as missing values when presenting output.
- Aliases added for query subcommands. For example, `org` is now an alias for `oq`. See `tfctl --help`.
- Docsgen now includes alias content in output documents.

### Chores

- Makefile release target now refreshes casts, if needed.
- Release workflow refactored to include more pre-flight tests and include CHANGELOG content in GH release.
- Minor docs tweaks; docs template source file renamed to avoid confusion.

## v1.2.2 - 2026-05-17

### Added

- Release blocked if CHANGELOG.md not updated.
- Refresh docs and man pages and added docs generation to release workflow.

## v1.2.1 - 2026-05-16

### Added

- Simplified `--json-into` and `--yaml-into` logic.

- Consistent pre-commit message.

## v1.2.0 - 2026-05-16

### Added

- `--json-into <file>` writes the filtered, transformed result as JSON to the
  specified file as a secondary output, independent of `--output`. Inspired by
  the [`-json-into` flag][opentofu-1.12.0] introduced in OpenTofu v1.12.0.

- `--yaml-into <file>` writes the filtered, transformed result as YAML to the
  specified file as a secondary output, independent of `--output`.

- When `--output raw` is in effect, the raw API response is written.

- Both flags support special files such as `/dev/stdout` and named pipes. Named pipes are not supported on Windows.

### Fixed

- Time transformations now reliably use the system local timezone.

---

[opentofu-1.12.0]: https://opentofu.org/blog/opentofu-1-12-0/#simultaneous-human-readable-and-machine-readable-output

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.1.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).