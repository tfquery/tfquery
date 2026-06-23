# tfquery jq Queries

The `--jq` flag provides an alternate filtering mode using jq query syntax. This mode is **experimental** and evaluates jq expressions against your query results.

The `--jq` implementation uses the [gojq](https://github.com/itchyny/gojq) library, a Go implementation of jq. For complete jq documentation and syntax reference, visit the [official jq manual](https://stedolan.github.io/jq/manual/).

## Overview

- The `--jq` flag accepts a single jq query string.
- The `--jq` value can be either a jq expression or an `@preset` reference from
  `presets.jq` in your config file.
- The query is evaluated against each row in the result set using the output attribute names (not the raw API keys).
- A row is included in results when the jq expression evaluates to a truthy value.
- `--jq` and `--filter` are mutually exclusive; you cannot use both flags in the same command.
- Shell quoting is required for jq expressions containing special characters or whitespace.

## jq Presets

You can store reusable jq queries under `presets.jq` in your tfquery config and
reference them with `@name`.

Example config:

```yaml
presets:
  jq:
    query1: '.updated > "2025-06-01"'
    query2: '.name | contains("cloud")'
```

Example usage:

```bash
tfquery wq --jq @query1
tfquery oq --jq @query2
```

## How jq Queries Work with tfquery

When you use `--jq`, each result row becomes the input to your jq expression. The row contains the attributes available in your output (as defined by `--attrs` or the command defaults).

For example, with `tfquery oq`, each organization becomes an object like:
```json
{
  "id": "org-...",
  "name": "my-org",
  "email": "admin@example.com"
}
```

Your jq query then operates on these objects. Truthiness in jq follows these rules:
- `false` and `null` are falsey.
- Everything else (including `0`, empty strings, empty arrays) is truthy.

## Operators and Functions

jq provides a rich set of operators and functions. Here are some commonly used ones:

- `==`, `!=` : equality / inequality
- `and`, `or`, `not` : logical operators
- `|` : pipe operator (pass output of left as input to right)
- `contains(str)` : substring/value containment test
- `startswith(str)` : prefix test
- `endswith(str)` : suffix test
- `ascii_downcase`, `ascii_upcase` : case transformation
- `type` : get the type of a value
- `//` : alternative operator (use right side if left is false/null)
- `select(condition)` : emit input if condition is true
- `map(expr)` : transform array elements
- `keys` : get object keys

See the [jq manual](https://stedolan.github.io/jq/manual/) for the full reference.

## Behavior Notes

- jq queries are evaluated after attribute projection, so use output attribute names (e.g., `.name`, not `.attributes.name`).
- If an attribute is not included in `--attrs`, it will not be available in your jq expression.
- jq runtime errors (e.g., type mismatches) will cause the command to fail with an error message.
- Filters are applied before attribute transformations, so transformations in `--attrs` won't affect query matching.

## Examples

**Basic matching:**
```bash
# Exact equality
tfquery oq --jq '.name == "my-org"'

# Substring matching
tfquery oq --jq '.name | contains("prod")'

# Case-insensitive substring
tfquery oq --jq '(.name | ascii_downcase) | contains("prod")'
```

**Logical combinations:**
```bash
# AND: both conditions must be true
tfquery wq --jq '.name | contains("prod") and .status == "applied"'

# OR: either condition can be true
tfquery oq --jq '.name | contains("prod") or .name | contains("dev")'

# Grouping with parentheses
tfquery sq --jq '((.type // "") | contains("security")) and ((.resource // "") | contains("8"))'
```

**Null-safe operations:**
```bash
# Use // to provide a default if a field is null or missing
tfquery mq --jq '(.description // "") | contains("important")'

# Check if a field exists and is not empty
tfquery wq --jq '.email != null and .email != ""'
```

**Pattern matching:**
```bash
# Prefix check
tfquery sq --jq '.type | startswith("aws_")'

# Suffix check
tfquery oq --jq '.email | endswith("@example.com")'

# Multiple patterns with OR
tfquery sq --jq '(.type | startswith("aws_")) or (.type | startswith("google_"))'
```

**Advanced:**
```bash
# Using select: emit if condition is true (alternative to just filtering)
tfquery wq --jq 'select(.status == "active" and .workspace_count > 5)'

# Type checking
tfquery oq --jq '.metadata | type == "object"'

# Piping multiple operations
tfquery sq --jq '.resource_type | ascii_downcase | contains("instance")'
```

## Mutual Exclusivity with --filter

The `--filter` and `--jq` flags cannot be used together. If both are provided, tfquery will return an error:

```bash
# This will fail
tfquery oq --filter 'name@prod' --jq '.status == "applied"'

# Error: flags --filter and --jq are mutually exclusive
```

Choose one filtering approach based on your needs:
- Use `--filter` for simple, high-performance filtering with tfquery's native syntax.
- Use `--jq` for complex expressions or when you're already familiar with jq.

## Limitations and Future Work

- `--jq` is currently experimental and subject to change.
- I/O and environment functions are intentionally disabled for security.
