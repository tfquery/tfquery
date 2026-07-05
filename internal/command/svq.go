// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"reflect"

	"github.com/apex/log"
	"github.com/hashicorp/go-tfe"
	"github.com/urfave/cli/v3"

	"github.com/tfquery/tfquery/internal/config"
	"github.com/tfquery/tfquery/internal/meta"
)

// svqDefaultAttrs specifies the default attributes displayed for state
// versions in the "svq" command output.
var svqDefaultAttrs = []string{".id", "serial", "created-at"}

// svqCommandAction is the action handler for the "svq" subcommand. It lists
// state versions via the active backend, supports --tldr/--schema shortcuts,
// and emits results per common flags.
func svqCommandAction(ctx context.Context, cmd *cli.Command) error {
	config.Config.Namespace = "svq"

	be, err := InitLocalBackendQuery(ctx, cmd)
	if err != nil {
		return err
	}

	fn := func(ctx context.Context, cmd *cli.Command) ([]*tfe.StateVersion, error) {
		return be.StateVersions(SvqServerSideFilterAugmenter)
	}

	return NewQueryActionRunner(
		"svq",
		reflect.TypeFor[tfe.StateVersion](),
		svqDefaultAttrs,
		fn,
	).Run(ctx, cmd)
}

// SvqServerSideFilterAugmenter augments the StateVersionListOptions with
// server-side filters extracted from the --filter flag. Flags with
// ServerSide=true populate matching fields in opts based on the filter key
// prefix (project, tag, or xtag). For tag filters, dot-separated keys are
// parsed to extract the tag name and add create TagBinding entries.
// NOTE The signature departure from the typical factory pattern used by other
// commands - this func is public.
// NOTE Unimplemented for now as StateVersionListOptions has no server-side
// filter fields.
func SvqServerSideFilterAugmenter(
	_ context.Context,
	cmd *cli.Command,
	opts *tfe.StateVersionListOptions,
) error {
	log.Debugf("opts after augmentation: %+v", opts)
	return nil
}

// svqCommandBuilder constructs the cli.Command for "svq", wiring metadata,
// flags, and action handlers.
func svqCommandBuilder(meta meta.Meta) *cli.Command {
	return (&QueryCommandBuilder{
		Name:      "svq",
		Alias:     "state-version",
		Usage:     "state version query",
		UsageText: "tfquery svq [RootDir] [options]",
		Flags: []cli.Flag{
			&cli.IntFlag{
				Name:    "limit",
				Aliases: []string{"l"},
				Usage:   "limit state versions returned",
				Value:   99999,
			},
			NewHostFlag("svq"),
			NewOrgFlag("svq"),
			workspaceFlag,
		},
		Action: svqCommandAction,
		Meta:   meta,
	}).Build()
}
