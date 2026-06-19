// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package util

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ParseRootDir parses a RootDir string and returns the absolute directory and
// any optional environment override. It returns an error if the fs entry does
// not exist, is empty or is not a directory.
func ParseRootDir(rootDir string) (string, string, error) {
	if rootDir == "" {
		return "", "", os.ErrInvalid
	}

	var dir, env string

	// How many "::" are there?
	switch strings.Count(rootDir, "::") {
	case 0:
		// No env override, so just use the rootDir as is.
	case 1:
		// One env override, so split on "::" and use the first part as the
		// rootDir and the second part as the env override.
	default:
		// More than one "::" is invalid.
		return "", "", fmt.Errorf("multiple '::' seperators are not supported")
	}

	// First, split the path to see if there is an ::env override.
	parts := strings.Split(rootDir, "::")
	if len(parts) > 1 {
		env = parts[1]
	}

	// Now determine if the actual root directory (parts[0]) is absolute or
	// relative. If it is relative, make it absolute.
	if !filepath.IsAbs(parts[0]) {
		cwd, err := os.Getwd()
		if err != nil {
			return "", "", err
		}
		dir = filepath.Join(cwd, parts[0])
	} else {
		dir = parts[0]
	}

	// If the rootDir is not a directory, return an error.
	if r, err := os.Stat(dir); err != nil {
		return "", "", err
	} else if !r.IsDir() {
		return "", "", os.ErrInvalid
	}

	return dir, env, nil
}
