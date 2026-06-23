# tfquery-ps

> Show a summary of the given plan.
> Also available as: `tfquery summarize`

- Show only a summary of a Terraform plan.

`terraform plan | tfquery ps`

- Show the full plan output while also including a summary.

`terraform plan | tee >(tfquery ps)`
