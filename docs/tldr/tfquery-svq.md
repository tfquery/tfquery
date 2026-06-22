# tfquery-svq

> Query Terraform state version history.
> Also available as: `tfquery state-version`

- Display state file history for current directory and include Created At information.

`tfquery svq --attrs created-at`

- Display the five most recent state file versions and include the YYYY-MM-DD portion of the Created At information.

`tfquery svq --limit 5 --attrs created-at::10`
