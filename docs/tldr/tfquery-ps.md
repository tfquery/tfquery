# tfquery-ps

> Summarize Terraform plan output for review and automation.
> Also available as: `tfquery summarize`

- Show only a summary of a Terraform plan.

`terraform plan | tfquery ps`

- Show the full plan output while also including a summary.

`terraform plan | tee >(tfquery ps)`

- Show only resources that will be created.

`terraform plan | tfquery ps --filter 'action=created'`

- Emit summary data in JSON for automation.

`terraform plan | tfquery ps --output json`

- Sort summary rows by action and then resource type.

`terraform plan | tfquery ps --sort action,type`
