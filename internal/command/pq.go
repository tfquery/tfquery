// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"reflect"
	"strings"

	"github.com/apex/log"
	"github.com/hashicorp/go-tfe"
	"github.com/urfave/cli/v3"

	"github.com/tfquery/tfquery/internal/backend/remote"
	"github.com/tfquery/tfquery/internal/config"
	"github.com/tfquery/tfquery/internal/filters"
	"github.com/tfquery/tfquery/internal/meta"
)

var pqDefaultAttrs = []string{".id", "name"}

// pqCommandAction is the action handler for the "pq" subcommand. It lists
// projects for the selected organization, supports --tldr/--schema
// short-circuit behavior, and emits output per common flags.
func pqCommandAction(ctx context.Context, cmd *cli.Command) error {
	config.Config.Namespace = "oq"

	be, org, client, err := InitRemoteOrgQuery(ctx, cmd)
	if err != nil {
		return err
	}

	fn := func(ctx context.Context, cmd *cli.Command) ([]*tfe.Project, error) {
		options := tfe.ProjectListOptions{
			ListOptions: DefaultListOptions,
		}
		return PaginateWithOptions(
			ctx,
			cmd,
			&options,
			func(ctx context.Context, opts *tfe.ProjectListOptions) (
				[]*tfe.Project,
				*tfe.Pagination,
				error,
			) {
				page, err := client.Projects.List(ctx, org, opts)
				if err != nil {
					ctxErr := OrgQueryErrorContext(
						be,
						org,
						"list projects",
					)
					return nil, nil, remote.FriendlyTFE(
						err,
						ctxErr,
					)
				}
				return page.Items, page.Pagination, nil
			},
			pqServerSideFilterAugmenter,
		)
	}

	return NewQueryActionRunner(
		"pq",
		reflect.TypeFor[tfe.Project](),
		pqDefaultAttrs,
		fn,
	).Run(ctx, cmd)
}

// pqServerSideFilterAugmenter augments the ProjectListOptions with
// server-side filters extracted from the --filter flag.
func pqServerSideFilterAugmenter(
	_ context.Context,
	cmd *cli.Command,
	opts *tfe.ProjectListOptions,
) error {
	// THINK Should we do this?
	// Include tag info.
	opts.Include = append(opts.Include, tfe.ProjectEffectiveTagBindings)

	spec := cmd.String("filter")
	filterList := filters.BuildFilters(spec)

	for _, f := range filterList {
		// We only care about server-side filters.
		if !f.ServerSide {
			continue
		}

		parts := strings.Split(f.Key, ".")

		if len(parts) > 1 && parts[0] == "tag" {
			opts.TagBindings = append(opts.TagBindings, &tfe.TagBinding{
				Key:   parts[1],
				Value: f.Value,
			})
			continue
		}

		if f.Key == "name" {
			opts.Query = f.Value
		}
	}

	log.Debugf("opts after augmentation: %+v", opts)
	return nil
}

// pqCommandBuilder constructs the cli.Command for "pq", wiring metadata,
// flags, and action/validator handlers.
func pqCommandBuilder(meta meta.Meta) *cli.Command {
	return (&QueryCommandBuilder{
		Name:  "pq",
		Alias: "project",
		Usage: "project query",
		Flags: []cli.Flag{
			NewHostFlag("pq", meta.Config.Source),
			NewOrgFlag("pq", meta.Config.Source),
		},
		Action: pqCommandAction,
		Meta:   meta,
	}).Build()
}
