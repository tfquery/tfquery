# NAME

tfctl ps — Plan summary

# DESCRIPTION

Show a summary of the given plan.

# USAGE

`tfq ps [planfile] [options]`


# OPTIONS

| Flag | Default | Description |
|------| ------- |-------------|
| --color/--no-color |  | Colorize text output. |
| --filter/-f string |  | Comma-separated list of filters to apply to results |
| --jq string |  | jq query string to apply to results. |
| --json-into string |  | secondary output path to write JSON output to. |
| --local/--no-local |  | Show timestamps in local time according to your OS. |
| --output/-o string |  | Output format (text (default), json, yaml, csv). [More info](#output)|
| --padding int | 1 | Number of spaces to pad columns in text output. |
| --sort/-s string |  | Comma-separated list of fields to sort by. [More info](#sort)|
| --titles/--no-titles |  | Include column headings in text output. |
| --tldr |  | Show tldr page if a client is installed. |
| --yaml-into string |  | secondary output path to write YAML output to. |



  ## output
  The `text` output format is a human-friendly table format. The `json`, `yaml`, and `csv` formats are machine-friendly and can be used for further processing.

  ## sort
  Prefix field names with `-` to sort in descending order.  For example, `--sort=-created-at,name` sorts first by Created At (newest first), then by Name (A-Z).




# EXAMPLES

**Show only a summary of a Terraform plan.**

```sh
terraform plan | tfq ps
```


**Show the full plan output while also including a summary.**

```sh
terraform plan | tee >(tfq ps)
```


