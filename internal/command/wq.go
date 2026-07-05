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

	"github.com/tfquery/tfquery/internal/filters"
	"github.com/tfquery/tfquery/internal/meta"
)

// wqDefaultAttrs specifies the default attributes displayed for workspaces
// in the "wq" command output.
var wqDefaultAttrs = []string{".id", "name"}

// wqCommandAction is the action handler for the "wq" subcommand. It lists
// workspaces for the selected organization.
func wqCommandAction(ctx context.Context, cmd *cli.Command) error {
	// We need to build the builder inside the action so we can access the
	// client. The builder will handle backend/org init, but we need a way to
	// pass the client-bound fetcher. Let's use a custom approach.
	be, org, client, err := InitRemoteOrgQuery(ctx, cmd)
	if err != nil {
		return err
	}

	// Create a fetcher that captures the client in a closure
	fetcher := func(
		ctx context.Context,
		org string,
		opts *tfe.WorkspaceListOptions,
	) ([]*tfe.Workspace, *tfe.Pagination, error) {
		page, err := client.Workspaces.List(ctx, org, opts)
		if err != nil {
			return nil, nil, err
		}
		return page.Items, page.Pagination, nil
	}

	// Manually call RemoteQueryFetcherFactory and QueryActionRunner since we
	// already have be, org, client initialized
	fn := RemoteQueryFetcherFactory(
		be,
		org,
		fetcher,
		wqServerSideFilterAugmenter,
		"list workspaces",
	)

	return NewQueryActionRunner(
		"wq",
		reflect.TypeFor[tfe.Workspace](),
		wqDefaultAttrs,
		fn,
	).Run(ctx, cmd)
}

// wqServerSideFilterAugmenter augments the WorkspaceListOptions with
// server-side filters extracted from the --filter flag. Flags with
// ServerSide=true populate matching fields in opts based on the filter key
// prefix (project, tag, or xtag). For tag filters, dot-separated keys are
// parsed to extract the tag name and create TagBinding entries.
func wqServerSideFilterAugmenter(
	_ context.Context,
	cmd *cli.Command,
	opts *tfe.WorkspaceListOptions,
) error {
	spec := cmd.String("filter")
	filterList := filters.BuildFilters(spec)

	for _, f := range filterList {
		// We only care about server-side filters.
		if f.ServerSide {
			parts := strings.Split(f.Key, ".")
			if len(parts) > 1 {
				switch parts[0] {
				case "name":
					opts.Search = f.Value
				case "project":
					opts.ProjectID = f.Value
				case "tag":
					opts.TagBindings = append(opts.TagBindings, &tfe.TagBinding{
						Key:   parts[1],
						Value: f.Value,
					})
				case "xtag":
					opts.ExcludeTags = parts[1]
				}
			}
		}
	}

	log.Debugf("opts after augmentation: %+v", opts)

	return nil
}

// wqCommandBuilder constructs the cli.Command for "wq", wiring metadata,
// flags, and action handlers.
func wqCommandBuilder(meta meta.Meta) *cli.Command {
	return (&QueryCommandBuilder{
		Name:      "wq",
		Alias:     "workspace",
		Usage:     "workspace query",
		UsageText: "tfquery wq [RootDir] [options]",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "limit",
				Aliases: []string{"l"},
				Usage:   "limit workspaces returned",
				Value:   99999,
			},
			NewHostFlag("wq", meta.Config.Source),
			NewOrgFlag("wq", meta.Config.Source),
		},
		Action: wqCommandAction,
		Meta:   meta,
	}).Build()
}
