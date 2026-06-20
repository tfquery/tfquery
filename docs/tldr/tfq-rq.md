# tfq-rq

> Query HCP/TFE runs for the given workspace.
> Also available as: `tfq run`

- Display runs and include Created At and Status information.

`tfq rq --attrs created-at,status`

- Display errored runs in the "prod" workspace of the "hr" org.

`tfq rq --org hr --workspace prod --filter 'status@errored'`
