# Configuration

tfq configuration is made in a YAML file. Environment variables can override certain settings at runtime.

## Configuration File

tfq reads its configuration from a YAML file. The file location is determined by the following precedence:

1. `TFQ_CFG_FILE` environment variable (if set and non-empty)
2. OS-specific user config directory:
   - Linux/Unix: `$HOME/.config/tfq/tfq.yaml`
   - macOS: `$HOME/Library/Application Support/tfq/tfq.yaml`
   - Windows: `%APPDATA%\tfq\tfq.yaml`

If the specified file is not found or cannot be parsed, tfq will error.

### Configuration Structure

See [tfq.yaml](tfq.yaml) for a complete reference of all available configuration options, including command-specific settings and defaults.

Preset references used by `--attrs`, `--filter`, and `--jq` are defined under
the top-level `presets` key (`presets.attrs`, `presets.filters`,
`presets.jq`).

## Environment Variable Overrides

The following environment variables override configuration file settings at runtime:

### `TFQ_CFG_FILE`

Specifies the full path to a tfq configuration file in YAML format.

**Usage:**
```bash
export TFQ_CFG_FILE=$HOME/.config/tfq/prod.yaml
tfq sq
```

### `TFQ_CACHE`

Controls whether tfq caches query results. Caching is enabled by default.

**Valid values:**
- Not set or empty: Caching enabled.
- `0` or `false`: Caching disabled.
- Any other value: Caching enabled.

**Usage:**
```bash
# Disable caching for this invocation
TFQ_CACHE=0 tfq sq

# Re-enable caching
unset TFQ_CACHE
```

### `TFQ_CACHE_DIR`

Specifies a custom directory for storing cached query results.

**Usage:**
```bash
export TFQ_CACHE_DIR=/mnt/fast-storage/tfq-cache
tfq sq
```

**Precedence:**
1. `TFQ_CACHE_DIR` (if set and non-empty)
2. OS-specific user cache directory (see Configuration File section above)

## Examples

### Use a custom config file and cache directory

```bash
export TFQ_CFG_FILE=$HOME/.tfq-prod.yaml
export TFQ_CACHE_DIR=$HOME/.cache/tfq-prod
tfq oq
```

### Disable caching for a single command

```bash
TFQ_CACHE=0 tfq sq --attrs arn
```

### Use a shared cache on a network drive

```bash
export TFQ_CACHE_DIR=/mnt/shared/tfq-cache
tfq wq
```

### Override all settings

```bash
TFQ_CFG_FILE=/etc/tfq/production.yaml \
TFQ_CACHE=0 \
tfq pq --sort created-at
```