// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"reflect"

	"github.com/hashicorp/go-tfe"
	"github.com/urfave/cli/v3"

	"github.com/tfquery/tfquery/internal/config"
	"github.com/tfquery/tfquery/internal/meta"
)

// rqDefaultAttrs specifies the default attributes displayed for runs in
// the "rq" command output.
var rqDefaultAttrs = []string{".id", "created-at", "status"}

// rqCommandAction is the action handler for the "rq" subcommand. It lists
// runs via the active backend, supports --tldr/--schema shortcuts, and
// emits results per common flags.
func rqCommandAction(ctx context.Context, cmd *cli.Command) error {
	config.Config.Namespace = "rq"

	be, err := InitLocalBackendQuery(ctx, cmd)
	if err != nil {
		return err
	}

	// Create a fetcher that delegates to the backend
	fetcher := func(
		ctx context.Context,
		org string,
		opts *tfe.RunListOptions,
	) ([]*tfe.Run, *tfe.Pagination, error) {
		runs, err := be.Runs()
		if err != nil {
			return nil, nil, err
		}
		// Local backend doesn't support pagination, return all results
		return runs, &tfe.Pagination{NextPage: 0}, nil
	}

	// Use RemoteQueryFetcherFactory to handle augmentation
	// (though local backend doesn't support it)
	fn := RemoteQueryFetcherFactory(
		nil, // no backend for error context (local backend)
		"",  // no org needed
		fetcher,
		rqServerSideFilterAugmenter,
		"list runs",
	)

	return NewQueryActionRunner(
		"rq",
		reflect.TypeOf((*tfe.Run)(nil)).Elem(),
		rqDefaultAttrs,
		fn,
	).Run(ctx, cmd)
}

// rqServerSideFilterAugmenter returns immediately without augmenting options.
// Local backend queries do not support server-side filtering.
func rqServerSideFilterAugmenter(
	_ context.Context,
	_ *cli.Command,
	_ *tfe.RunListOptions,
) error {
	return nil
}

// rqCommandBuilder constructs the cli.Command for "rq", wiring metadata,
// flags, and action handlers.
func rqCommandBuilder(meta meta.Meta) *cli.Command {
	return (&QueryCommandBuilder{
		Name:      "rq",
		Alias:     "run",
		Usage:     "run query",
		UsageText: "tfquery rq [RootDir] [options]",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "limit",
				Aliases: []string{"l"},
				Usage:   "limit runs returned",
				Value:   99999,
			},
			NewHostFlag("rq"),
			NewOrgFlag("rq"),
			workspaceFlag,
		},
		Action: rqCommandAction,
		Meta:   meta,
	}).Build()
}
