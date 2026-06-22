// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"reflect"

	"github.com/hashicorp/go-tfe"
	"github.com/hashicorp/jsonapi"
	"github.com/urfave/cli/v3"

	"github.com/tfquery/tfquery/internal/attrs"
	"github.com/tfquery/tfquery/internal/backend/remote"
	"github.com/tfquery/tfquery/internal/config"
	"github.com/tfquery/tfquery/internal/meta"
	"github.com/tfquery/tfquery/internal/output"
)

// DefaultListOptions provides the standard pagination starting point for all
// remote API list operations.
var DefaultListOptions = tfe.ListOptions{
	PageNumber: 1,
	PageSize:   100,
}

// BuildAttrs constructs an AttrList with defaults and optional extras from
// --attrs, then applies the global transform spec.
func BuildAttrs(cmd *cli.Command, defaults ...string) attrs.AttrList {
	var attrList attrs.AttrList
	if cmd != nil {
		if attrsFromConfig, err := config.GetStringSlice(cmd.Name + ".attrs"); err == nil && len(attrsFromConfig) > 0 {
			defaults = attrsFromConfig
		}
	}

	//nolint:errcheck,gosec // AttrList.Set errors are logged internally
	{
		for _, defaultAttr := range defaults {
			attrList.Set(defaultAttr)
		}
		if extras := cmd.String("attrs"); extras != "" {
			attrList.Set(extras)
		}
		attrList.SetGlobalTransformSpec()
	}

	return attrList
}

// DumpSchemaIfRequested writes the JSON schema for the provided type to stdout
// when --schema is set, and returns true if it handled the request.
func DumpSchemaIfRequested(cmd *cli.Command, t reflect.Type) bool {
	if cmd.Bool("schema") {
		output.DumpSchema("", t, nil)
		return true
	}
	return false
}

// EmitJSONAPISlice marshals a slice as JSONAPI and passes it to the common
// output routine.
func EmitJSONAPISlice(results any, al attrs.AttrList, cmd *cli.Command) error {
	var raw bytes.Buffer
	if err := jsonapi.MarshalPayload(&raw, results); err != nil {
		return fmt.Errorf("failed to marshal payload: %w", err)
	}
	output.SliceDiceSpit(raw, al, cmd, "data", os.Stdout, nil)
	return nil
}

// GetMeta returns the meta.Meta stored in the command's Metadata. If missing
// or of an unexpected type, it returns the zero value.
func GetMeta(cmd *cli.Command) meta.Meta {
	if cmd == nil || cmd.Metadata == nil {
		return meta.Meta{}
	}
	if m, ok := cmd.Metadata["meta"].(meta.Meta); ok {
		return m
	}
	return meta.Meta{}
}

// PaginateWithOptions[T, O] is a generic paginator that drives paginated API
// calls with mutable options. It handles pagination logic and returns all
// collected results. The augmenter callback (if provided) is called before
// each API invocation, allowing options customization (e.g., setting filters
// or tags). The fetcher callback encapsulates the actual API call and must
// return results, pagination info, and any error.
func PaginateWithOptions[T, O any](
	ctx context.Context,
	cmd *cli.Command,
	options *O,
	fetcher func(context.Context, *O) ([]T, *tfe.Pagination, error),
	augmenter Augmenter[O],
) ([]T, error) {
	var results []T

	// Paginate through pages
	for {
		// Invoke augmenter before each page (to allow options mutation)
		if augmenter != nil {
			if err := augmenter(ctx, cmd, options); err != nil {
				return nil, err
			}
		}

		// Fetch current page
		items, pagination, err := fetcher(ctx, options)
		if err != nil {
			return nil, err
		}

		results = append(results, items...)

		// Check if there are more pages
		if pagination.NextPage == 0 {
			break
		}

		// Increment page number for next iteration
		setPageNumber(options, pagination.NextPage)
	}

	return results, nil
}

// RemoteQueryFetcherFactory creates a generic fetch function for remote
// org-based queries. It handles the common pagination and augmentation logic,
// delegating only the API call itself to the provided fetcher, and handling
// errors with the provided operation name for context.
func RemoteQueryFetcherFactory[T, O any](
	be *remote.BackendRemote,
	org string,
	fetcher RemoteOrgListFetcher[T, O],
	augmenter Augmenter[O],
	operation string,
) func(context.Context, *cli.Command) ([]T, error) {
	return func(ctx context.Context, cmd *cli.Command) ([]T, error) {
		options := new(O)
		// Set DefaultListOptions on the options struct's ListOptions field
		setListOptionsDefaults(options)

		results, err := PaginateWithOptions(
			ctx,
			cmd,
			options,
			func(ctx context.Context, opts *O) ([]T, *tfe.Pagination, error) {
				items, pagination, err := fetcher(ctx, org, opts)
				if err != nil {
					ctxErr := OrgQueryErrorContext(be, org, operation)
					return nil, nil, remote.FriendlyTFE(err, ctxErr)
				}
				return items, pagination, nil
			},
			augmenter,
		)
		return results, err
	}
}

// ShortCircuitTLDR checks the --tldr flag and, if present and available,
// runs `tldr tfquery <subcmd>` and returns true so the caller can exit early.
func ShortCircuitTLDR(ctx context.Context, cmd *cli.Command, subcmd string) bool {
	if cmd.Bool("tldr") {
		if _, err := exec.LookPath("tldr"); err == nil {
			c := exec.CommandContext(ctx, "tldr", "tfquery", subcmd)
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			_ = c.Run()
		}
		return true
	}
	return false
}

// setListOptionsDefaults uses reflection to set DefaultListOptions on a
// struct's ListOptions field.
func setListOptionsDefaults(options any) {
	v := reflect.ValueOf(options).Elem()
	lo := v.FieldByName("ListOptions")
	if lo.IsValid() && lo.CanSet() {
		lo.Set(reflect.ValueOf(DefaultListOptions))
	}
}

// setPageNumber uses reflection to set the PageNumber field in the options
// struct. It assumes the struct has a ListOptions.PageNumber field (standard
// in tfe API options).
func setPageNumber(options any, pageNumber int) {
	v := reflect.ValueOf(options).Elem()
	lo := v.FieldByName("ListOptions")
	if !lo.IsValid() {
		return
	}
	pn := lo.FieldByName("PageNumber")
	if pn.IsValid() && pn.CanSet() {
		pn.SetInt(int64(pageNumber))
	}
}
