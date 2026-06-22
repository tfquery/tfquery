# tfquery Quick Start Tutorial

## Prerequisites

- `tfquery` installed and in your PATH. If installed via Homebrew or the Debian package, manual pages are available (e.g., `man tfquery`).
  - If you installed from a tarball, see "Install man pages (from tarball)" and "Install TLDR pages (from tarball)" in [installation.md](installation.md) to set up local man and TLDR pages.
- An access token ([`terraform login`](https://developer.hashicorp.com/terraform/cli/commands/login) or [manual configuration](https://developer.hashicorp.com/terraform/cli/config/config-file#credentials)) for remote queries stored in HCP or TFE workspaces,
- A Terraform workspace to query (local or remote).

## Tutorial Overview

This tutorial demonstrates core tfquery concepts using organization queries (**oq**), but these patterns apply to all tfquery commands. While examples reference Terraform Cloud/Enterprise, tfquery works equally well with both Terraform and OpenTofu workspaces and backends.

## Supported Backends

tfquery automatically detects and works with these backend types:

- **Local** - Standard `terraform.tfstate` files stored locally.
- **S3** - State stored in AWS S3 buckets with standard AWS authentication.
- **Cloud** - HCP Terraform (formerly Terraform Cloud) with `cloud` backend configuration.
- **Remote** - Terraform Enterprise and HCP Terraform with `remote` backend configuration.

**Note:** Backend support covers the features we needed first. Not all capabilities of each backend are covered. If you need additional functionality or a new backend, please open an issue or submit a PR.

**Note:** Encrypted OpenTofu state files are supported with automatic detection and prompting for decryption keys.

No additional configuration is needed - tfquery reads your existing Terraform/OpenTofu backend configuration and authenticates using your current credentials.

## Getting Started

For **tfquery** in general, or any query command, help is always available --
```sh
# General tfquery help
tfquery --help

# oq command-specific help
tfquery oq --help
```

**tfquery** follows a typical CLI command pattern where the query command is specified followed by optional flags and arguments.

```sh
# List all HCP/TFE Organizations found on host
tfquery oq

# Expected output:
# NAME           ID
# my-org         org-123abc
# test-org       org-456def

# List those same Organizations, sorted by name
# and output the results in JSON.
tfquery oq --sort name --output json

# Expected output:
# {"data":[{"id":"org-456def","type":"organizations","attributes":{"name":"test-org"}}, ...]}

# Include the organizations created date
tfquery oq --attrs created-at

# Expected output:
# NAME           CREATED-AT
# my-org         2023-01-15T10:30:00Z
# test-org       2023-02-20T14:45:00Z
```

Unless a specific command's documentation notes otherwise, many of the flags (`--sort`, `--output`, etc) have identical usage across all commands.  See [Flags](flags.md) for detailed information about each.

The `--attrs/-a` flag deserves special mention and is documented in detail at [Attributes](attrs.md).  **tfquery** can query almost any attribute returned by the [Terraform API](https://developer.hashicorp.com/terraform/cloud-docs/api-docs).  Additionally, the results can be transformed in a variety of ways.

Each command, except sq, includes a `--schema` flag that will list common attributes that might be used.

```sh
# Show common oq query attributes
tfquery oq --schema

# Expected output:
# created-at
# email
# external-id
# name
# permissions
# ...

# Any of those attributes can then be used in a query
tfquery oq --attrs email

# Expected output:
# NAME           EMAIL
# my-org         admin@example.com
# test-org       test@example.com

# Multiple attributes can be comma-separated
tfquery oq --attrs email,created-at

# Expected output:
# NAME           EMAIL                CREATED-AT
# my-org         admin@example.com    2023-01-15T10:30:00Z
# test-org       test@example.com     2023-02-20T14:45:00Z
```

## Putting It All Together

Let's walk through a complete workflow to find a specific organization and get detailed information:

```sh
# 1. List all organizations to see what's available
tfquery oq

# 2. Filter to find production-related orgs
tfquery oq --filter 'name@prod'

# Expected output:
# NAME           ID
# prod-main      org-789xyz
# staging-prod   org-101abc

# 3. Get detailed info including admin email and creation date for automation
tfquery oq --filter 'name@prod' --attrs email,created-at --output json

# Expected output:
# {"data":[{"type":"organizations","attributes":{"name":"prod-main","email":"prod-admin@example.com","created-at":"2023-01-10T08:00:00Z"}}, ...]}
```

## JSON Output for Automation

One of the common use cases for tfquery is in automation workflows. Using `--output json` allows output from tfquery to be easily consumed by the next step in an automation pipeline.

```sh
# Get organization data in JSON format for automation
tfquery oq --attrs email,created-at --output json

# Expected output:
# {"data":[{"type":"organizations","attributes":{"name":"my-org","email":"admin@example.com","created-at":"2023-01-15T10:30:00Z"}}, ...]}

# Example: Extract emails for notification automation
tfquery oq --attrs email --output json | jq -r '.data[].attributes.email' | while read email; do
  echo "Sending notification to: $email"
  # curl -X POST ... or other notification logic
done

# Example: Find orgs created in the last 30 days and export to CSV
tfquery oq --attrs created-at --output json | \
  jq -r '.data[] | select(.attributes."created-at" > (now - 2592000) | strftime("%Y-%m-%dT%H:%M:%SZ")) | [.attributes.name, .attributes."created-at"] | @csv' > recent_orgs.csv
```

## Next Steps

Now that you understand the basics:
- Try other commands: `tfquery wq` (workspaces), `tfquery sq` (state).
- Explore advanced filtering with `tfquery command --help --examples`.
- See [Flags](flags.md) for complete reference.