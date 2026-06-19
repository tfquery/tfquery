# tfctl-svq

> Query Terraform state version history.
> Also available as: `tfctl state-version`

- Display state file history for current directory and include Created At information.

`tfq svq --attrs created-at`

- Display the five most recent state file versions and include the YYYY-MM-DD portion of the Created At information.

`tfq svq --limit 5 --attrs created-at::10`
