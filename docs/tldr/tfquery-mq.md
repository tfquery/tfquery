# tfquery-mq

> Query HCP/TFE modules.
> Also available as: `tfquery module`

- Display modules and include Created At information.

`tfquery mq --attrs created-at`

- Display modules in the "hr" org with "iam" in their name.

`tfquery mq --org hr --filter 'name@iam'`
