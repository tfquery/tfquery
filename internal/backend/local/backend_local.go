// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package local

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/apex/log"
	tfe "github.com/hashicorp/go-tfe"
	"github.com/urfave/cli/v3"

	"github.com/tfquery/tfquery/internal/differ"
	"github.com/tfquery/tfquery/internal/svutil"
)

// BackendLocal is a struct that represents a local backend configuration.
// https://developer.hashicorp.com/terraform/language/backend/local
type BackendLocal struct {
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
		Type   string `json:"type" validate:"eq=local"`
		Config struct {
			Path         string `json:"path"`
			WorkspaceDir string `json:"workspace_dir"`
		} `json:"config"`
		Hash int `json:"hash"`
	} `json:"backend"`
}

func (be *BackendLocal) DiffStates(ctx context.Context, cmd *cli.Command) ([][]byte, error) {
	// Fixup diffArgs
	svSpecs := []string{"CSV~1", "CSV~0"}

	diffArgs := differ.ParseDiffArgs(ctx, cmd)

	switch len(diffArgs) {
	case 0:
		// No args, so use the last two states.
	case 1:
		if strings.HasPrefix(diffArgs[0], "+") {
			// limit := 9999
			// if l, err := strconv.Atoi(diffArgs[0][1:]); err == nil {
			// 	limit = l
			// }

			stateVersionList, err := be.StateVersions( /* TODO limit */ )
			if err != nil {
				return nil, fmt.Errorf("failed to get state version list: %v", err)
			}

			selectedVersions := differ.SelectStateVersions(stateVersionList)

			log.Debugf("selectedVersions: %d", len(selectedVersions))

			if len(selectedVersions) == 0 {
				return nil, nil
			} else if len(selectedVersions) == 2 {
				svSpecs[0] = selectedVersions[1].ID
				svSpecs[1] = selectedVersions[0].ID
			}
		} else {
			svSpecs[0] = diffArgs[0]
		}
	case 2:
		svSpecs = diffArgs
	}

	states, _ := be.States(svSpecs[0], svSpecs[1])

	return states, nil
}

func (be *BackendLocal) Runs() ([]*tfe.Run, error) {
	return nil, fmt.Errorf("not implemented")
}

func (be *BackendLocal) State() ([]byte, error) {
	sv := be.Cmd.String("sv")
	states, err := be.States(sv)
	if err != nil {
		return nil, err
	}
	return states[0], nil
}

// stateFile pairs a discovered state version with the raw document bytes that
// scan() already read from disk, so callers can avoid double reading the file.
type stateFile struct {
	version *tfe.StateVersion
	body    []byte
}

// StateVersions implements backend.Backend. It scans be.RootDir for state and
// backup files and returns minimal tfe.StateVersion metadata for each.
func (be *BackendLocal) StateVersions(augmenter ...func(context.Context, *cli.Command, *tfe.StateVersionListOptions) error) ([]*tfe.StateVersion, error) {
	files, err := be.scan()
	if err != nil {
		return nil, err
	}

	versions := make([]*tfe.StateVersion, 0, len(files))
	for _, f := range files {
		versions = append(versions, f.version)
	}

	return versions, nil
}

func (be *BackendLocal) States(specs ...string) ([][]byte, error) {
	var results [][]byte

	files, err := be.scan()
	if err != nil {
		return nil, err
	}

	// Index the bodies scan() already read so resolved versions can be served
	// without a second trip to disk.
	candidates := make([]*tfe.StateVersion, 0, len(files))
	bodies := make(map[string][]byte, len(files))
	for _, f := range files {
		candidates = append(candidates, f.version)
		bodies[f.version.JSONDownloadURL] = f.body
	}

	versions, err := svutil.Resolve(candidates, specs...)
	if err != nil {
		return nil, err
	}
	log.Debugf("versions: %v", versions)

	// Now pound through the found versions and return each of their state bodies.
	for _, v := range versions {
		body, ok := bodies[v.JSONDownloadURL]
		if !ok {
			// A file-path spec (svutil.resolveFileSpec) resolves to a path that
			// was never globbed, so it isn't in the scan. Read it directly.
			if body, err = os.ReadFile(v.JSONDownloadURL); err != nil {
				return nil, fmt.Errorf("failed to read state file: %w", err)
			}
		}
		results = append(results, body)
	}

	return results, nil
}

func (be *BackendLocal) String() string {
	return be.Backend.Config.Path
}

func (be *BackendLocal) Type() (string, error) {
	return be.Backend.Type, nil
}

// scan globs be.RootDir (honoring any .terraform/environment workspace
// override) for state and backup files, reading each exactly once. It returns
// them newest-first by mod time, carrying both the tfe.StateVersion metadata
// (ID from filename, CreatedAt from file timestamp, Serial from the document)
// and the raw body. Other Backend types cache their listings; we don't here,
// since local filesystem access should be lickity split.
func (be *BackendLocal) scan() ([]stateFile, error) {
	// If there's a .terraform/environment file, we need to use that to
	// determine the workspace directory.
	if be.EnvOverride == "" {
		envFile := filepath.Join(be.RootDir, ".terraform/environment")
		if envFileData, err := os.ReadFile(envFile); err == nil {
			be.EnvOverride = string(bytes.TrimSpace(envFileData))
		}
	}

	envPath := ""
	if be.EnvOverride != "" {
		envPath = filepath.Join("terraform.tfstate.d", be.EnvOverride)
	}

	files, err := filepath.Glob(filepath.Join(be.RootDir, envPath, "terraform.tfstate*"))
	if err != nil {
		return nil, err
	}
	type fileInfo struct {
		path string
		mod  int64
	}
	var infos []fileInfo
	for _, f := range files {
		stat, err := os.Stat(f)
		if err != nil {
			continue
		}
		infos = append(infos, fileInfo{f, stat.ModTime().UnixNano()})
	}
	// Sort by mod time, descending
	sort.Slice(infos, func(i, j int) bool {
		return infos[i].mod > infos[j].mod
	})

	var found []stateFile
	for _, info := range infos {
		// Read the body once; serial is parsed from it and the bytes are retained
		// so States() doesn't have to read the file again.
		body, err := os.ReadFile(info.path)
		if err != nil {
			continue
		}

		// We care about just grabbing serial out of the doc.
		var doc struct {
			Serial int64 `json:"serial"`
		}

		if err := json.Unmarshal(body, &doc); err != nil {
			continue
		}

		found = append(found, stateFile{
			version: &tfe.StateVersion{
				ID:        filepath.Base(info.path),
				CreatedAt: time.Unix(0, info.mod),
				Serial:    doc.Serial,
				// We're stealing this attribute and using it as the full path to state.
				JSONDownloadURL: info.path,
			},
			body: body,
		})
	}

	return found, nil
}
