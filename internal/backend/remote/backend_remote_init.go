// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package remote

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/apex/log"
	"github.com/urfave/cli/v3"

	"github.com/tfquery/tfquery/internal/config"
)

type BackendRemoteOption = func(ctx context.Context, cmd *cli.Command, be *BackendRemote) error

// BuckNaked gives non-state related commands a bare minimum remote config so
// they have enough info to connect to a server.
func BuckNaked() BackendRemoteOption {
	return func(ctx context.Context, cmd *cli.Command, be *BackendRemote) error {
		// THINK Revisit how host is determined across all backend types.
		// If host is not set explicitly (we're BuckNaked) *AND* there is a host:
		// entry in the config, use it.  Otherwise, just fall through and continue
		// with the default.
		if !cmd.IsSet("host") {
			if cfgHost, _ := config.GetString("host"); cfgHost != "" {
				_ = cmd.Set("host", cfgHost)
				be.Backend.Config.Hostname = cfgHost
			}
		} else {
			be.Backend.Config.Hostname = cmd.String("host")
			be.Backend.Config.Token, _ = be.Token()
		}

		log.Debugf("BuckNaked(): hostname: %s", be.Backend.Config.Hostname)

		return nil
	}
}

// FromRootDir build a remote config from the provided IAC root directory.
func FromRootDir(rootDir string, required ...bool) BackendRemoteOption {
	return func(ctx context.Context, cmd *cli.Command, be *BackendRemote) error {
		// Is rootDir a relative or absolute path?
		if filepath.IsAbs(rootDir) {
			be.RootDir = rootDir
		} else {
			cwd, _ := os.Getwd()
			be.RootDir = filepath.Join(cwd, rootDir)
		}

		log.Debugf("NewBackendRemote FromRootDir(): rootDir = %s", be.RootDir)

		err := be.load()

		// Return no error is required is present and false.
		if len(required) > 0 && !required[0] {
			return nil
		}
		return err
	}
}

// NewBackendRemote returns a BackendRemote object that implements the Backend
// interface. It is load()ed from the config file found in the rootDir.
func NewBackendRemote(ctx context.Context, cmd *cli.Command, options ...BackendRemoteOption) (*BackendRemote, error) {
	options = append([]BackendRemoteOption{WithDefaults()}, options...)

	be := &BackendRemote{Ctx: ctx, Cmd: cmd}

	for _, opt := range options {
		if err := opt(ctx, cmd, be); err != nil {
			return nil, err
		}
	}

	return be, nil
}

func WithDefaults() BackendRemoteOption {
	return func(ctx context.Context, cmd *cli.Command, be *BackendRemote) error {
		cwd, _ := os.Getwd()
		be.RootDir = cwd
		be.Version = 4
		be.TerraformVersion = "0.0.0"
		be.Backend.Type = "remote"

		return nil
	}
}

func WithEnvOverride(env string) BackendRemoteOption {
	return func(ctx context.Context, cmd *cli.Command, be *BackendRemote) error {
		if env != "" {
			be.EnvOverride = env
		}
		return nil
	}
}

func WithSvOverride() BackendRemoteOption {
	return func(ctx context.Context, cmd *cli.Command, be *BackendRemote) error {
		sv := cmd.String("sv")
		if sv != "" {
			be.SvOverride = sv
		}
		return nil
	}
}

// load reads the terraform config file and unmarshals it into the BackendRemote
// struct. It is simply a convenience method to make NewBackendRemote more
// readable.
func (be *BackendRemote) load() error {
	tfFile := be.RootDir + "/.terraform/terraform.tfstate"
	data, err := os.ReadFile(tfFile)
	if err != nil {
		return fmt.Errorf("failed to read local config file: %w", err)
	}

	var temp BackendRemote
	if err := json.Unmarshal(data, &temp); err != nil {
		return fmt.Errorf("failed to unmarshal local config file: %w", err)
	}

	if temp.Backend.Type != "remote" {
		return fmt.Errorf("%w: backend type is not remote: %s", errors.New("bad"), temp.Backend.Type)
	}

	be.Version = temp.Version
	be.TerraformVersion = temp.TerraformVersion
	be.Backend = temp.Backend

	if be.Backend.Config.Token == nil {
		token, _ := be.Token()
		be.Backend.Config.Token = token
	}

	return nil
}
