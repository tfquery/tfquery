# tfq Attributes

The `--attrs` flag is one of tfq's most powerful features, allowing you to specify exactly which data fields to extract and how to format them. Understanding the `--attrs` syntax enables you to create precise results tailored to your needs.

## Prerequisites

To effectively use `--attrs`, you should be familiar with:
- **Terraform/OpenTofu state file structure** - Understanding how resources and their attributes are organized
- **JSON path notation** - How to navigate nested JSON objects using dot notation
- **Terraform API schemas** - The structure of data returned by HCP Terraform/Enterprise APIs

Use the `--schema` flag with any command (except `sq`) to explore available attributes.

## Basic Syntax

The `--attrs` flag accepts a comma-separated list of attribute specifications:

```
tfq command --attrs spec1,spec2,spec3
```

Each specification has the format:
```
json_path:output_name:transform_spec
```

Where:
- `json_path` - The JSON path to extract (required)
- `output_name` - Column name in output (optional)
- `transform_spec` - Data transformation rules (optional)

## JSON Path Extraction

### Root vs Attributes Paths

**Attributes path (default):**
```sh
# Extracts from .attributes.email
tfq oq --attrs email
```

**Root path (starts with `.`):**
```sh
# Extracts from .id (root level)
tfq oq --attrs .id
```

**Nested paths:**
```sh
# Deep nested extraction
tfq wq --attrs vcs-repo.identifier
tfq pq --attrs permissions.can-create-workspaces
```

### Examples

**Organizations (`oq`):**
```sh
# Basic attributes
tfq oq --attrs name,email,created-at

# Root level data
tfq oq --attrs .id,.type

# Mixed paths
tfq oq --attrs .id,email,external-id
```

**Workspaces (`wq`):**
```sh
# Workspace details
tfq wq --attrs name,terraform-version,working-directory

# VCS information
tfq wq --attrs vcs-repo.identifier,vcs-repo.branch

# Permissions and settings
tfq wq --attrs auto-apply,queue-all-runs
```

## Output Naming

Control how columns appear in your output:

```sh
# Default: uses last segment of JSON path
tfq oq --attrs created-at
# Output column: "created-at"

# Custom name: specify after colon
tfq oq --attrs created-at:Created
# Output column: "Created"

# Multiple custom names
tfq oq --attrs email:Admin,created-at:Date
# Output columns: "Admin", "Date"
```

## Data Transformations

Transform data as it's extracted using transformation specifications.

### Case Transformations

```sh
# Convert to uppercase
tfq oq --attrs name::U

# Convert to lowercase
tfq oq --attrs name::L

# Mixed transformations
tfq oq --attrs name::U,email::L
```

### Length Transformations

```sh
# Truncate to first N characters
tfq oq --attrs name::10
# "my-long-organization" → "my-long-or"

# Compress long strings (show beginning and end)
tfq oq --attrs name::-8
# "my-long-organization" → "my-l..on"
```

### Time Transformations

```sh
# Convert UTC to local timezone (requires TZ environment variable)
tfq oq --attrs created-at::t

# Example with TZ set:
export TZ="America/New_York"
tfq oq --attrs created-at::t
# "2023-01-15T10:30:00Z" → "2023-01-15T05:30:00EST"
```

### Combined Transformations

```sh
# Multiple transforms applied in sequence
tfq oq --attrs name::U10        # Uppercase, then truncate to 10 chars
tfq oq --attrs created-at::tL    # Convert timezone, then lowercase
tfq oq --attrs email:Admin:L15   # Custom name, lowercase, truncate to 15
```

## Advanced Usage

### Exclusion

Exclude attributes from processing (useful for filtering/sorting only):

```sh
# Include in processing but not in output
tfq oq --attrs 'name,email' --filter 'name@prod'
```

### Global Transformations

Apply transformations to all attributes:

```sh
# Make all output uppercase
tfq oq --attrs '*::U,name,email,created-at'
```

### Schema Discovery

Explore available attributes before crafting queries:

```sh
# Show all available attributes for organizations
tfq oq --schema

# Common output:
# created-at
# email
# external-id
# name
# permissions
# collaborator-auth-policy
# ...
```

## Practical Examples

### Audit Report
```sh
# Create detailed org audit with custom formatting
tfq oq --attrs name:Organization:U,email:Admin,created-at:Created::t \
  --output json > org_audit.json
```

### Resource Inventory
```sh
# Workspace inventory with VCS info
tfq wq --attrs name::20,terraform-version:TF_Ver::U,vcs-repo.identifier:Repo::-30
```

### Filtered Extraction
```sh
# Get production workspace details
tfq wq --filter 'name@prod' \
  --attrs name:Workspace,working-directory:Path,auto-apply:Auto::U
```

### State Analysis
```sh
# Analyze state file resources
tfq sq --attrs type::15,name::25,provider::10
```

## Tips and Best Practices

1. **Start with `--schema`** - Always explore available attributes first.
2. **Use meaningful output names** - Make reports self-documenting.
3. **Test transformations** - Try transform specs on sample data first.
4. **Combine with filtering** - Use `--attrs` and `--filter` together for powerful queries.
5. **Consider output format** - JSON output preserves full data for further processing.

## Error Handling

**Invalid paths** - tfq will show empty values for non-existent paths
**Invalid transforms** - Bad transformation specs are ignored
**Type mismatches** - Only string values can be transformed; others pass through unchanged

Understanding these attribute extraction patterns unlocks tfq's full querying power, enabling you to extract exactly the data you need in the format you want.
