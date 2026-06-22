# tfquery-sq

> Query Terraform state files.
> Also available as: `tfquery state`

- Display state file in current directory and include Created At information.

`tfquery sq --attrs created-at`

- Aggregates the state files in the `iac1/` and `iac2/` directories and displays combined results.

`tfquery sq iac1/ iac2/`

- Display only concrete resources with "vpc" in their type or name.

`tfquery sq --concrete --filter 'resource@vpc'`

- Display the third most recent state file version in JSON format.

`tfquery sq --sv -3 --output json`
