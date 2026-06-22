// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"context"
	"fmt"

	"github.com/apex/log"
	"github.com/hashicorp/go-tfe"
	"github.com/urfave/cli/v3"

	"github.com/tfquery/tfquery/internal/backend"
	"github.com/tfquery/tfquery/internal/backend/remote"
)

// RemoteOrgListFetcher[T, O] is the signature for a function that performs
// the actual API list call for a resource type. It takes the context, org,
// options (mutable), and returns items, pagination, or error.
// T is the result type (e.g., *tfe.Workspace), O is the options type
// (e.g., tfe.WorkspaceListOptions).
type RemoteOrgListFetcher[T, O any] func(
	context.Context,
	string,
	*O,
) ([]T, *tfe.Pagination, error)

// InitLocalBackendQuery initializes a local backend connection for queries
// that operate on local state. It returns the backend or an error if
// initialization fails.
func InitLocalBackendQuery(ctx context.Context, cmd *cli.Command) (
	backend.Backend,
	error,
) {
	be, err := backend.NewBackend(ctx, *cmd)
	if err != nil {
		return nil, err
	}
	log.Debugf("be: %v", be)
	return be, nil
}

// InitRemoteOrgQuery initializes a remote backend connection for queries that
// operate exclusively on organizations. It returns the backend, organization
// name, and TFE client, or an error if initialization fails.
func InitRemoteOrgQuery(
	ctx context.Context,
	cmd *cli.Command,
) (*remote.BackendRemote, string, *tfe.Client, error) {
	be, err := remote.NewBackendRemote(ctx, cmd, remote.BuckNaked())
	if err != nil {
		return nil, "", nil, err
	}
	log.Debugf("be: %v", be)

	client, err := be.Client()
	if err != nil {
		return nil, "", nil, err
	}
	log.Debugf("client: %v", client.BaseURL())

	org, err := be.Organization()
	if err != nil {
		return nil, "", nil, fmt.Errorf(
			"failed to resolve organization: %w",
			err,
		)
	}

	return be, org, client, nil
}

// OrgQueryErrorContext is a helper to construct remote.ErrorContext for
// organization-related queries (mq, pq). It requires the backend and
// organization.
func OrgQueryErrorContext(
	be *remote.BackendRemote,
	org string,
	operation string,
) remote.ErrorContext {
	return remote.ErrorContext{
		Host:      be.Backend.Config.Hostname,
		Org:       org,
		Operation: operation,
		Resource:  "organization",
	}
}
