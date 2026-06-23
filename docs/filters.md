# tfquery Filters

The `--filter` flag narrows query results using a small, expressive syntax. This page documents the syntax and gives examples.

The `--jq` flag provides an alternate filtering mode using jq query syntax.
It is experimental and cannot be combined with `--filter`.

## Overview

- The default delimiter between multiple filters is a comma (`,`). You can override the delimiter by setting the environment variable `TFQ_FILTER_DELIM`.
- Each filter has the form `key<operator>value` for binary filters, or
  `key?` / `key!?` for existence checks.
- Prefixing an operator with `!` negates that operator (for example
  `name!@prod` means "name does NOT contain 'prod'").
- The `key` is the key as it exists in the output. For example, if the filter is `--filter "name@prod"`, there must be an attribute in the output called `name`. If that attribute is not included as one of the defaults, it must be included via `--attrs`.
- Filters referencing attribute keys that start with `_` are treated as server-side API filters and are ignored by the local filter engine.

## Supported operators

- `=`  : exact match (string equality)
- `~`  : case-insensitive equality
- `^`  : starts-with
- `>`  : lexicographic greater-than
- `<`  : lexicographic less-than
- `@`  : contains / membership (works for substrings and collections)
- `/`  : regular expression match
- `?`  : field exists (unary)

Negation is expressed by placing `!` before the operator.
For existence checks, `key!?` means field is missing.

## Special filters

- `hungarian` : Detects resources following Hungarian notation naming conventions (applies to `sq` command only)
  - Example: `tfquery sq --filter hungarian=true` — shows only resources with names following Hungarian notation
  - Supported values: `true`, `false`, or bare `hungarian` (equivalent to `hungarian=true`)
  - The filter analyzes resource type and name to detect common prefix patterns (e.g., `s3Bucket`, `ec2Instance`, `iamRole`)
  - Case-insensitive and supports underscores and dashes in names

## Delimiter and quoting

If a target contains the delimiter character, quote the whole filter or choose a different delimiter with `TFQ_FILTER_DELIM`.

## Behavior notes and edge cases

- If a filter references an attribute name that doesn't exist in the discovered attribute list, the filter check will fail, a warning will be displayed and the filter ignored.
- For `@` (contains), the implementation supports strings, slices, and maps:
  - For strings it does a substring test.
  - For slices it tests for membership.
  - For maps it checks for a key.
- For `/` (regex), the pattern uses Go's `regexp` package syntax. Invalid regular expressions will log an error and exclude the item from results.
- For existence checks (`?` and `!?`), a field is considered **missing** if it meets any of these conditions:
  - The field key does not exist in the resource.
  - The field value is `null`.
  - The field value is an empty string (`""`).
- Filters are evaluated before attribute transformations are applied (so transformations in `--attrs` won't affect filter matching).
- When using `sq` with `--concrete`, tfquery automatically appends `mode=managed` to the filter set.

## Examples

**jq filtering (experimental):**
```bash
# Match a single value
tfquery wq --jq '.name == "my-resource"'

# Logical AND
tfquery wq --jq '.name == "my-resource" and .id == "res-123"'

# Logical OR
tfquery wq --jq '.name == "my-resource" or .id == "res-123"'

# Grouping
tfquery wq --jq '(.name == "my-resource" and .id == "res-123") or .type == "aws_instance"'

# Nested field checks
tfquery wq --jq '.nested.inner != null and .nested.inner != ""'
```

`--jq` and `--filter` are mutually exclusive. If both are provided, tfquery
returns an error.

**Basic filtering:**
```bash
# Simple contains
tfquery oq --filter 'name@prod'

# Negation (not contains)
tfquery oq --filter 'name!@prod'

# Exact match
tfquery wq --filter 'status=applied'

# Case-insensitive equality
tfquery oq --filter 'email~admin'

# Starts with
tfquery wq --filter 'name^prod'
```

**Regular expression filtering:**
```bash
# Find workspaces matching a naming pattern (prod-001, prod-002, etc.)
tfquery wq --filter 'name/^prod-\d{3}$'

# Find resources with version numbers
tfquery sq --filter 'version/^\d+\.\d+\.\d+$'

# Match email addresses in organization attributes
tfquery oq --filter 'email/^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$'
```

**Complex filtering:**
```bash
# Multiple filters (comma-delimited)
tfquery oq --filter 'name@prod,created-at>2024-01-01'

# Find items that do not have a given tag
tfquery mq --filter 'tags!@deprecated'

# Combine different operators
tfquery wq --filter 'name^prod,status=applied,!description='

# Field exists
tfquery wq --filter 'name?'

# Field missing
tfquery wq --filter 'description!?'

# Find resources with Hungarian notation naming (sq only)
tfquery sq --filter 'hungarian=true'

# Find resources NOT using Hungarian notation (sq only)
tfquery sq --filter 'hungarian=false'
```

For implementation details, see the `FilterDataset` and `BuildFilters`
functions in the project source (internal/filters/filters.go).
