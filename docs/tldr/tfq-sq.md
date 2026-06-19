# tfctl-sq

> Query Terraform state files.
> Also available as: `tfctl state`

- Display state file in current directory and include Created At information.

`tfq sq --attrs created-at`

- Aggregates the state files in the `iac1/` and `iac2/` directories and displays combined results.

`tfq sq iac1/ iac2/`

- Display only concrete resources with "vpc" in their type or name.

`tfq sq --concrete --filter 'resource@vpc'`

- Display the third most recent state file version in JSON format.

`tfq sq --sv -3 --output json`
