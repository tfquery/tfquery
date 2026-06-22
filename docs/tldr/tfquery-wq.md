# tfquery-wq

> Query HCP/TFE workspaces.
> Also available as: `tfquery workspace`

- Display workspaces and include Created At information.

`tfquery wq --attrs created-at`

- Display workspaces in the "hr" org with "prod" in their name.

`tfquery wq --org hr --filter 'name@prod'`
