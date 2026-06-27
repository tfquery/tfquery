# tfquery Flags

tfquery has a rich collection of flags available to each command. Many of these flags are common across all commands. See each command's help for information about unique flags and functionality details.

## Common Flags

| Flag | Description |
|------|-------------|
| `-a`, `--attrs`   | A comma-separated list of attributes to include in the result. See [Attributes](attrs.md) for a much more detailed discussion. |
| `-c`, `--color`   | Enable colored text output. |
| `-f`, `--filter`  | A comma-separated list of filters to apply to the result before it is returned. See [Filters](filters.md) for a much more detailed discussion. |
| `--help` | Show command-specific help. |
| `--json-into` | Write the result as JSON to the specified file. This is a secondary output and is independent of `--output`. |
| `-o`, `--output` | Output format. Valid values are `text` (default), `json`, `yaml` or `raw`. Raw is a JSON dump of the Terraform API response. |
| `-s`, `--sort`    | A comma-separated list of attributes to sort the result by. Reverse sorting is indicated by a leading `-`. |
| `-v`, `--version` | Print tfquery version information and exit. |
| `-t`, `--titles`  | Print attribute name column headings when in text output mode. |
| `--yaml-into` | Write the result as YAML to the specified file. This is a secondary output and is independent of `--output`. |


## Usage

Unless noted otherwise in the command-specific documentation, flags and arguments can appear in any order _except_ for specifying the optional IaC root directory. That argument, if used, _must_ appear immediately following the command. For commands that support aggregate mode (like `sq`), multiple [RootDir] values can be provided, which will aggregate results across all directories.

```sh
# Query the current state assuming the CWD is the
# IaC root directory. CWD is implied.
tfquery sq --sort resource

# Query the current state of a specific IaC root
# directory that might not be CWD.
tfquery sq ${HOME}/myproject/iac --sort name

# Aggregate state across multiple IaC root directories.
tfquery sq ${HOME}/project1/iac ${HOME}/project2/iac --sort name
```

Conflicting flags and arguments will often be silently ignored. For example, the `--titles` flag is only used in text output mode. If `--titles` is used alongside, for example, `--output json`, it is silently ignored.

```sh
# These both produce identical results. --titles
# is silently ignored.
tfquery oq --output json
tfquery oq --output json --titles
```

The `--json-into` and `--yaml-into` flags are additive and are combined with `--output`. The file is written after the primary output is rendered.

```sh
# Display text output in the terminal and save a JSON
# copy to a file simultaneously.
tfquery pq --json-into /tmp/projects.json

# Output to a named-pipe. On Linux, all special file types
# are supported. Windows does not support named pipes.
tfquery pq --json-into /dev/stderr
```