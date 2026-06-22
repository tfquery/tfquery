// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package cloud

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/urfave/cli/v3"

	"github.com/tfquery/tfquery/internal/backend/remote"
	"github.com/tfquery/tfquery/internal/config"
)

// BackendCloud represents a Terraform Cloud or Enterprise backend
// configuration, which is a specific type of remote backend. It contains the
// necessary fields to configure and authenticate with TFC/TFE, and provides a
// method to transform itself into a BackendRemote for use by tfquery.
type BackendCloud struct {
	// Runtime context
	Cmd *cli.Command
	Ctx context.Context

	// Configuration overrides
	EnvOverride string
	RootDir     string `json:"-" validate:"dir"`

	// Version info
	TerraformVersion string `json:"terraform_version" validate:"semver"`
	Version          int    `json:"version" validate:"gte=4"`

	// Backend configuration
	Backend struct {
		Type   string `json:"type"`
		Hash   int    `json:"hash"`
		Config struct {
			Hostname     string `json:"hostname" validate:"hostname"`
			Organization string `json:"organization" validate:"required"`
			Token        any    `json:"token"`
			Workspaces   struct {
				Name    string            `json:"name"`
				Project string            `json:"project"`
				Tags    map[string]string `json:"-"`
			} `json:"workspaces"`
		} `json:"config"`
	} `json:"backend"`
}

// Token retrieves the token from the environment variable, config file, or
// the credentials file, in that order.
func (be *BackendCloud) Token() (string, error) {
	var token string

	// Figure out if Token needs to be overridden by an environment variable.
	// The precedence is:
	// 1. TF_TOKEN_app_terraform_io
	// 2. TF_TOKEN
	// 3. Token in the config file
	// 4. Token in the TF credentials file.
	hostname := strings.ReplaceAll(be.Backend.Config.Hostname, ".", "_")
	if token = os.Getenv("TF_TOKEN_" + hostname); token == "" {
		token = os.Getenv("TF_TOKEN")
	}

	// If token was overridden by an environment variable, use that value and go
	// home early.
	if token != "" {
		return token, nil
	}

	token, _ = be.Backend.Config.Token.(string)

	// Once we're here, token may have existed already in the config file or it
	// may have been overridden by an environment variable. If it's still empty,
	// we need to try to get it from the credentials file.
	if token == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to get user home directory: %w", err)
		}

		credsFile := home + "/.terraform.d/credentials.tfrc.json"
		data, err := os.ReadFile(credsFile)
		if err != nil {
			return "", fmt.Errorf("failed to read credentials file: %w", err)
		}

		var creds struct {
			Credentials map[string]struct {
				Token string `json:"token"`
			} `json:"credentials"`
		}

		if err := json.Unmarshal(data, &creds); err != nil {
			return "", fmt.Errorf("failed to unmarshal credentials file: %w", err)
		}

		if cred, ok := creds.Credentials[be.Backend.Config.Hostname]; ok {
			return cred.Token, nil
		}
	}

	return token, nil
}

func (be *BackendCloud) Transform2Remote(ctx context.Context, cmd *cli.Command) *remote.BackendRemote {
	beRemote := remote.BackendRemote{Ctx: ctx, Cmd: cmd}

	beRemote.RootDir = be.RootDir
	beRemote.Version = be.Version
	beRemote.TerraformVersion = be.TerraformVersion
	beRemote.EnvOverride = be.EnvOverride
	beRemote.Backend.Type = "remote"
	beRemote.Backend.Hash = be.Backend.Hash

	host := be.Backend.Config.Hostname
	if host == "" {
		host = cmd.String("host")
	}
	beRemote.Backend.Config.Hostname = host

	// Organization precedence: --org > terraform.backend{} > tfquery.yaml
	// Detect if --org is explicitly set to a value different from tfquery.yaml
	// config so tfquery.yaml doesn't override backend values.
	flagOrg := cmd.String("org")
	// Attempt to read namespaced and global org from tfquery.yaml to infer defaults
	var cfgOrg string
	if ns := cmd.Name; ns != "" {
		if v, err := config.GetString(ns + ".org"); err == nil {
			cfgOrg = v
		}
	}
	if cfgOrg == "" {
		if v, err := config.GetString("org"); err == nil {
			cfgOrg = v
		}
	}

	// Start from backend value
	org := be.Backend.Config.Organization
	// If flag is provided and differs from cfg default, prefer it
	if flagOrg != "" && flagOrg != cfgOrg {
		org = flagOrg
	} else if org == "" {
		// Backend empty: fall back to flag (even if from cfg)
		org = flagOrg
	}
	beRemote.Backend.Config.Organization = org

	beRemote.Backend.Config.Workspaces.Name = be.Backend.Config.Workspaces.Name
	beRemote.Backend.Config.Token, _ = beRemote.Token()

	return &beRemote
}
