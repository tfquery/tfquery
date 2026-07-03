// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"reflect"

	"github.com/apex/log"
	"github.com/urfave/cli/v3"
)

// QueryActionRunner[T] encapsulates the common query action pattern for all
// query subcommands. It handles steps 1-4 and 6 (GetMeta, short-circuit
// checks, BuildAttrs, schema dumping, and output emission), with step 5
// (data fetching) provided by FetchFn.
type QueryActionRunner[T any] struct {
	CommandName  string
	SchemaType   reflect.Type
	DefaultAttrs []string
	FetchFn      func(context.Context, *cli.Command) ([]T, error)
}

// Run executes the query action with the provided context and command.
func (qar *QueryActionRunner[T]) Run(
	ctx context.Context,
	cmd *cli.Command,
) error {
	// Step 1: GetMeta + debug.
	m := GetMeta(cmd)
	log.Debugf("Executing action for %v", m.Args[1:])

	// Step 2: Short-circuit checks.
	if ShortCircuitTLDR(ctx, cmd, qar.CommandName) {
		return nil
	}
	if DumpSchemaIfRequested(cmd, qar.SchemaType) {
		return nil
	}

	// Step 3: BuildAttrs + debug.
	attrs := BuildAttrs(cmd, true, qar.DefaultAttrs...)
	log.Debugf("attrs: %v", attrs)

	// Step 4: Fetch data.
	results, err := qar.FetchFn(ctx, cmd)
	if err != nil {
		return err
	}

	// Step 5: Emit + return.
	if err := EmitJSONAPISlice(results, attrs, cmd); err != nil {
		return err
	}
	return nil
}

// NewQueryActionRunner creates a QueryActionRunner with the provided
// configuration. It's a convenience factory that reduces boilerplate in
// individual command files.
func NewQueryActionRunner[T any](
	commandName string,
	schemaType reflect.Type,
	defaultAttrs []string,
	fetchFn func(context.Context, *cli.Command) ([]T, error),
) *QueryActionRunner[T] {
	return &QueryActionRunner[T]{
		CommandName:  commandName,
		SchemaType:   schemaType,
		DefaultAttrs: defaultAttrs,
		FetchFn:      fetchFn,
	}
}
