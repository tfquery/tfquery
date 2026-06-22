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

	"github.com/tfquery/tfquery/internal/config"
	"github.com/tfquery/tfquery/internal/filters"
	"github.com/tfquery/tfquery/internal/meta"
)

// mqDefaultAttrs specifies the default attributes displayed for registry
// modules in the "mq" command output.
var mqDefaultAttrs = []string{".id", "name"}

// mqCommandAction is the action handler for the "mq" subcommand. It lists
// registry modules for the selected organization, supports --tldr/--schema
// shortcuts, and emits results per common flags.
func mqCommandAction(ctx context.Context, cmd *cli.Command) error {
	be, org, client, err := InitRemoteOrgQuery(ctx, cmd)
	if err != nil {
		return err
	}

	config.Config.Namespace = "mq"

	// Create a fetcher that captures the client in a closure
	fetcher := func(
		ctx context.Context,
		org string,
		opts *tfe.RegistryModuleListOptions,
	) ([]*tfe.RegistryModule, *tfe.Pagination, error) {
		page, err := client.RegistryModules.List(ctx, org, opts)
		if err != nil {
			return nil, nil, err
		}
		return page.Items, page.Pagination, nil
	}

	// Use RemoteQueryFetcherFactory to handle pagination and augmentation
	fn := RemoteQueryFetcherFactory(
		be,
		org,
		fetcher,
		mqServerSideFilterAugmenter,
		"list registry modules",
	)

	return NewQueryActionRunner(
		"mq",
		reflect.TypeOf((*tfe.RegistryModule)(nil)).Elem(),
		mqDefaultAttrs,
		fn,
	).Run(ctx, cmd)
}

// mqServerSideFilterAugmenter augments the registry module list options with
// server-side filters before each API call.
func mqServerSideFilterAugmenter(
	_ context.Context,
	cmd *cli.Command,
	opts *tfe.RegistryModuleListOptions,
) error {
	spec := cmd.String("filter")
	filterList := filters.BuildFilters(spec)

	for _, f := range filterList {
		// We only care about server-side filters.
		if f.ServerSide {
			parts := strings.Split(f.Key, ".")
			switch parts[0] {
			case "provider":
				opts.Provider = f.Value
			case "registry":
				switch f.Value {
				case "public":
					opts.RegistryName = tfe.PublicRegistry
				case "private":
					opts.RegistryName = tfe.PrivateRegistry
				}
			}
		}
	}

	// THINK Other server-sides to include?
	// opts.WildcardName = "*dev*"
	// opts.Include = append(opts.Include, tfe.WSOrganization)

	log.Debugf("opts after augmentation: %+v", opts)

	return nil
}

// mqCommandBuilder constructs the cli.Command for "mq", wiring metadata,
// flags, and action handlers.
func mqCommandBuilder(meta meta.Meta) *cli.Command {
	return (&QueryCommandBuilder{
		Name:      "mq",
		Alias:     "module",
		Usage:     "module registry query",
		UsageText: "tfquery mq [RootDir] [options]",
		Flags: []cli.Flag{
			NewHostFlag("mq", meta.Config.Source),
			NewOrgFlag("mq", meta.Config.Source),
		},
		Action: mqCommandAction,
		Meta:   meta,
	}).Build()
}
