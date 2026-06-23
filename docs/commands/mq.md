# NAME

tfquery mq — HCP/TFE Module Query

tfquery module

# DESCRIPTION

Query HCP/TFE modules.

# USAGE

`tfquery mq [options]`


# OPTIONS

| Flag | Default | Description |
|------| ------- |-------------|
| --attrs/-a string |  | Comma-separated list of attributes to include in results. [More info](#attrs)|
| --color/--no-color |  | Colorize text output. |
| --filter/-f string |  | Comma-separated list of filters to apply to results |
| --host/-h string | app.terraform.io | HCP/TFE host to use. Overrides the backend. |
| --jq string |  | jq query string to apply to results. |
| --json-into string |  | secondary output path to write JSON output to. |
| --limit int | unlimited | Limit the result set to `int` results. |
| --local/--no-local |  | Show timestamps in local time according to your OS. |
| --org string |  | Organization to use for all commands. Overrides the backend. |
| --output/-o string |  | Output format (text (default), json, yaml, csv). [More info](#output)|
| --padding int | 1 | Number of spaces to pad columns in text output. |
| --schema |  | List of attributes for use with `--attrs`, `--filter`, and '--sort'. |
| --sort/-s string |  | Comma-separated list of fields to sort by. [More info](#sort)|
| --titles/--no-titles |  | Include column headings in text output. |
| --tldr |  | Show tldr page if a client is installed. |
| --yaml-into string |  | secondary output path to write YAML output to. |



  ## attrs
  There are many possible attributes depending on the command. Common attributes include: `id`, `type`, `name`, `created-at`, `updated-at`, etc. But, each command has it's own schema. Other than the `sq` command, use the `--schema` flag to see the full list of available attributes.

There is also a feature-rich syntax for transforming attribute values. See [Attributes](../attrs.md) for details.

  ## output
  The `text` output format is a human-friendly table format. The `json`, `yaml`, and `csv` formats are machine-friendly and can be used for further processing.

  ## sort
  Prefix field names with `-` to sort in descending order.  For example, `--sort=-created-at,name` sorts first by Created At (newest first), then by Name (A-Z).




# EXAMPLES

**Display modules and include Created At information.**

```sh
tfquery mq --attrs created-at
```


**Display modules in the "hr" org with "iam" in their name.**

```sh
tfquery mq --org hr --filter 'name@iam'
```


