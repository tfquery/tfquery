// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"reflect"

	"github.com/apex/log"
	"github.com/hashicorp/go-tfe"
	"github.com/urfave/cli/v3"

	"github.com/tfquery/tfquery/internal/backend/remote"
	"github.com/tfquery/tfquery/internal/config"
	"github.com/tfquery/tfquery/internal/filters"
	"github.com/tfquery/tfquery/internal/meta"
)

var oqDefaultAttrs = []string{"external-id:id", ".id:name"}

// oqCommandAction is the action handler for the "oq" subcommand. It lists
// organizations from the configured host, supports --tldr/--schema
// short-circuit behavior, and emits output per common flags.
func oqCommandAction(ctx context.Context, cmd *cli.Command) error {
	config.Config.Namespace = "oq"

	be, err := remote.NewBackendRemote(ctx, cmd, remote.BuckNaked())
	if err != nil {
		return err
	}

	client, err := be.Client()
	if err != nil {
		return err
	}

	fn := func(ctx context.Context, cmd *cli.Command) ([]*tfe.Organization, error) {
		options := tfe.OrganizationListOptions{
			ListOptions: DefaultListOptions,
		}
		return PaginateWithOptions(
			ctx,
			cmd,
			&options,
			func(ctx context.Context, opts *tfe.OrganizationListOptions) (
				[]*tfe.Organization,
				*tfe.Pagination,
				error,
			) {
				page, err := client.Organizations.List(ctx, opts)
				if err != nil {
					return nil, nil, err
				}
				return page.Items, page.Pagination, nil
			},
			oqServerSideFilterAugmenter,
		)
	}

	return NewQueryActionRunner(
		"oq",
		reflect.TypeFor[tfe.Organization](),
		oqDefaultAttrs,
		fn,
	).Run(ctx, cmd)
}

// oqServerSideFilterAugmenter augments the OrganizationListOptions with
// server-side filters extracted from the --filter flag.
func oqServerSideFilterAugmenter(
	_ context.Context,
	cmd *cli.Command,
	opts *tfe.OrganizationListOptions,
) error {
	spec := cmd.String("filter")
	filterList := filters.BuildFilters(spec)

	for _, f := range filterList {
		// We only care about server-side filters.
		if f.ServerSide && f.Key == "query" {
			opts.Query = f.Value
		}
	}

	log.Debugf("opts after augmentation: %+v", opts)
	return nil
}

// oqCommandBuilder constructs the cli.Command for "oq", configuring metadata,
// flags, and the associated action/validator.
func oqCommandBuilder(meta meta.Meta) *cli.Command {
	return (&QueryCommandBuilder{
		Name:      "oq",
		Alias:     "org",
		Usage:     "organization query",
		UsageText: "tfquery oq [RootDir] [options]",
		Flags: []cli.Flag{
			NewHostFlag("oq", meta.Config.Source),
		},
		Action: oqCommandAction,
		Meta:   meta,
	}).Build()
}
