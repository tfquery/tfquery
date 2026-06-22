// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package backend

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/apex/log"
	"github.com/hashicorp/go-tfe"
	"github.com/urfave/cli/v3"

	"github.com/tfquery/tfquery/internal/backend/cloud"
	"github.com/tfquery/tfquery/internal/backend/local"
	"github.com/tfquery/tfquery/internal/backend/remote"
	"github.com/tfquery/tfquery/internal/backend/s3"
	"github.com/tfquery/tfquery/internal/meta"
)

// Type holds common backend resolution context and flags.
type Type struct {
	// Runtime context
	Cmd *cli.Command
	Ctx context.Context

	// Configuration
	EnvOverride string
	RootDir     string `json:"-" validate:"dir"`

	// Version info (currently unused)
	// TerraformVersion string `json:"terraform_version" validate:"semver"`
	// Version          int    `json:"version" validate:"gte=4"`
}

// Backend abstracts Terraform/OpenTofu backend interactions needed by the
// application.
type Backend interface {
	Runs() ([]*tfe.Run, error)
	// State() returns the CSV~0 state document.
	State() ([]byte, error)
	// States() returns the state documents specified by the specs.
	States(...string) ([][]byte, error)
	// StateVersions accepts an optional augmenter function to apply
	// server-side filters. Only remote backends use this; local and S3 ignore it.
	StateVersions(augmenter ...func(context.Context, *cli.Command, *tfe.StateVersionListOptions) error) ([]*tfe.StateVersion, error)
	String() string
	Type() (string, error)
}

// SelfDiffer is implemented by backends that can diff state snapshots without
// an external differ.
type SelfDiffer interface {
	DiffStates(ctx context.Context, cmd *cli.Command) ([][]byte, error)
}

// NewBackend returns the appropriate Backend implementation for the working
// directory represented by the resolved root dir in command metadata.
func NewBackend(ctx context.Context, cmd cli.Command) (Backend, error) {
	cmdMeta := cmd.Metadata["meta"].(meta.Meta)
	log.Debugf("NewBackend: meta: %v", cmdMeta)

	configFile, configErr := os.Stat(filepath.Join(cmdMeta.RootDir, ".terraform", "terraform.tfstate"))
	stateFile, stateErr := os.Stat(filepath.Join(cmdMeta.RootDir, "terraform.tfstate"))
	envFile, envErr := os.Stat(filepath.Join(cmdMeta.RootDir, ".terraform", "environment"))
	_, _, _ = configFile, stateFile, envFile // Used only for existence check, values unused

	// Maybe we're in a non-sq command and just need a naked remote. This will be
	// when config, state and env are all in error meaning none of them exist.
	if configErr != nil && stateErr != nil && envErr != nil {
		return remote.NewBackendRemote(ctx, &cmd, remote.BuckNaked())
	}

	// If terraform.tfstate exists but .terraform/terraform.tfstate doesn't,
	// infer local backend. This is an empty terraform.backend {} block use case.
	if configErr != nil && stateErr == nil {
		return local.NewBackendLocal(ctx, &cmd,
			local.FromRootDir(cmdMeta.RootDir),
			local.WithEnvOverride(cmdMeta.Env),
		)
	}

	// If .terraform/terraform.tfstate and terraform.tfstate don't exist but
	// .terraform/environment does, we're in a local backend with multi-workspace
	// configuration. The environment file points to the workspace directory.
	if configErr != nil && stateErr != nil && envErr == nil {
		return local.NewBackendLocal(ctx, &cmd,
			local.FromRootDir(cmdMeta.RootDir),
			local.WithEnvOverride(cmdMeta.Env),
		)
	}

	// Peek at the backend type so we can switch on it.
	// TODO: We're double reading the file. Once in peek() and once in the New().
	backendType, err := peek(cmdMeta)
	if err != nil {
		return nil, err
	}

	var result Backend
	switch backendType {
	case "cloud":
		var cloudBackend *cloud.BackendCloud
		cloudBackend, err = cloud.NewBackendCloud(ctx, &cmd,
			cloud.FromRootDir(cmdMeta.RootDir),
			cloud.WithEnvOverride(cmdMeta.Env),
		)
		// Preserve prior behavior: return transformed backend alongside any error.
		result = cloudBackend.Transform2Remote(ctx, &cmd)
	case "local":
		result, err = local.NewBackendLocal(ctx, &cmd,
			local.FromRootDir(cmdMeta.RootDir),
			local.WithEnvOverride(cmdMeta.Env),
		)
	case "remote":
		result, err = remote.NewBackendRemote(ctx, &cmd,
			remote.FromRootDir(cmdMeta.RootDir),
			remote.WithEnvOverride(cmdMeta.Env),
			remote.WithSvOverride(),
		)
	case "s3":
		result, err = s3.NewBackendS3(ctx, &cmd,
			s3.FromRootDir(cmdMeta.RootDir),
			s3.WithEnvOverride(cmdMeta.Env),
			s3.WithSvOverride(),
		)
	default:
		return nil, fmt.Errorf("unknown type %s: %w", backendType, err)
	}

	return result, err
}

// peek returns the backend type by reading the local terraform state file.
func peek(cmdMeta meta.Meta) (string, error) {
	raw, err := os.ReadFile(filepath.Join(cmdMeta.RootDir, ".terraform", "terraform.tfstate"))
	if err != nil {
		return "", err
	}

	var parsedData map[string]json.RawMessage
	if err := json.Unmarshal(raw, &parsedData); err != nil {
		return "", fmt.Errorf("can't peek: %w", err)
	}

	if err := json.Unmarshal(parsedData["backend"], &parsedData); err != nil {
		return "", fmt.Errorf("can't peek: %w", err)
	}

	var backendType string
	if err := json.Unmarshal(parsedData["type"], &backendType); err != nil {
		return "", fmt.Errorf("can't peek: %w", err)
	}
	log.Debugf("type: %s", backendType)

	return backendType, nil
}
