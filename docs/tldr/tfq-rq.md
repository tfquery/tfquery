# tfctl-rq

> Query HCP/TFE runs for the given workspace.
> Also available as: `tfctl run`

- Display runs and include Created At and Status information.

`tfq rq --attrs created-at,status`

- Display errored runs in the "prod" workspace of the "hr" org.

`tfq rq --org hr --workspace prod --filter 'status@errored'`
