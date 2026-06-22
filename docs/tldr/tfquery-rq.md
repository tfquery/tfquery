# tfquery-rq

> Query HCP/TFE runs for the given workspace.
> Also available as: `tfquery run`

- Display runs and include Created At and Status information.

`tfquery rq --attrs created-at,status`

- Display errored runs in the "prod" workspace of the "hr" org.

`tfquery rq --org hr --workspace prod --filter 'status@errored'`
