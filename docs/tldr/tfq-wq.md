# tfctl-wq

> Query HCP/TFE workspaces.
> Also available as: `tfctl workspace`

- Display workspaces and include Created At information.

`tfq wq --attrs created-at`

- Display workspaces in the "hr" org with "prod" in their name.

`tfq wq --org hr --filter 'name@prod'`
