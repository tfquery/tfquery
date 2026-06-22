// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"

	"github.com/urfave/cli/v3"

	"github.com/tfquery/tfquery/internal/meta"
)

// QueryCommandBuilder is a helper that constructs a cli.Command for query
// subcommands (mq, pq, oq, svq, rq, wq) using a consistent pattern.
// It accepts the command name, usage text, optional UsageText, custom flags,
// the action handler, and meta. The builder automatically wires metadata,
// adds tldr/schema flags, applies global flags, and sets up validators.
type QueryCommandBuilder struct {
	Name      string
	Alias     string
	Usage     string
	UsageText string
	Flags     []cli.Flag
	Action    func(context.Context, *cli.Command) error
	Meta      meta.Meta
}

// Build returns a configured cli.Command from the builder.
func (qcb *QueryCommandBuilder) Build() *cli.Command {
	return &cli.Command{
		Name:      qcb.Name,
		Aliases:   []string{qcb.Alias},
		Usage:     qcb.Usage,
		UsageText: qcb.UsageText,
		Metadata: map[string]any{
			"meta": qcb.Meta,
		},
		Flags: append(qcb.Flags, append([]cli.Flag{
			tldrFlag,
			schemaFlag,
		}, NewGlobalFlags(qcb.Name)...)...),
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			ResolveInverseFlags(cmd, qcb.Meta.Args, []string{
				"chop", "color", "local", "titles", "short",
			})

			return ctx, GlobalFlagsValidator(ctx, cmd)
		},
		Action: qcb.Action,
	}
}
