// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0
// no-cloc

package util

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseRootDir(t *testing.T) {
	tests := []struct {
		name     string
		rootDir  string
		setupDir func(t *testing.T) string
		wantEnv  string
		wantErr  bool
		errIs    error
	}{
		{
			name: "absolute_path_no_env",
			setupDir: func(t *testing.T) string {
				return t.TempDir()
			},
			wantEnv: "",
			wantErr: false,
		},
		{
			name: "absolute_path_with_env",
			setupDir: func(t *testing.T) string {
				tmpDir := t.TempDir()
				return tmpDir + "::prod"
			},
			wantEnv: "prod",
			wantErr: false,
		},
		{
			name: "relative_path_no_env",
			setupDir: func(t *testing.T) string {
				tmpDir := t.TempDir()
				oldCwd, err := os.Getwd()
				if err != nil {
					t.Fatalf("failed to get cwd: %v", err)
				}
				err = os.Chdir(filepath.Dir(tmpDir))
				if err != nil {
					t.Fatalf("failed to chdir: %v", err)
				}
				t.Cleanup(func() {
					_ = os.Chdir(oldCwd)
				})
				return filepath.Base(tmpDir)
			},
			wantEnv: "",
			wantErr: false,
		},
		{
			name: "relative_path_with_env",
			setupDir: func(t *testing.T) string {
				tmpDir := t.TempDir()
				oldCwd, err := os.Getwd()
				if err != nil {
					t.Fatalf("failed to get cwd: %v", err)
				}
				err = os.Chdir(filepath.Dir(tmpDir))
				if err != nil {
					t.Fatalf("failed to chdir: %v", err)
				}
				t.Cleanup(func() {
					_ = os.Chdir(oldCwd)
				})
				return filepath.Base(tmpDir) + "::staging"
			},
			wantEnv: "staging",
			wantErr: false,
		},
		{
			name: "nonexistent_directory",
			setupDir: func(_ *testing.T) string {
				return "/nonexistent/path/that/does/not/exist"
			},
			wantErr: true,
			errIs:   os.ErrNotExist,
		},
		{
			name: "file_not_directory",
			setupDir: func(t *testing.T) string {
				tmpDir := t.TempDir()
				tmpFile := filepath.Join(tmpDir, "file.txt")
				err := os.WriteFile(tmpFile, []byte("test"), 0o600)
				if err != nil {
					t.Fatalf(
						"failed to create temp file: %v",
						err,
					)
				}
				return tmpFile
			},
			wantErr: true,
			errIs:   os.ErrInvalid,
		},
		{
			name: "empty_env_override",
			setupDir: func(t *testing.T) string {
				tmpDir := t.TempDir()
				return tmpDir + "::"
			},
			wantEnv: "",
			wantErr: false,
		},
		{
			name: "multiple_colons_separator",
			setupDir: func(t *testing.T) string {
				tmpDir := t.TempDir()
				return tmpDir + "::dev::extra"
			},
			wantEnv: "dev",
			wantErr: true,
		},
		{
			name: "env_with_whitespace",
			setupDir: func(t *testing.T) string {
				tmpDir := t.TempDir()
				return tmpDir + "::  staging  "
			},
			wantEnv: "  staging  ",
			wantErr: false,
		},
		{
			name: "empty_root_dir",
			setupDir: func(_ *testing.T) string {
				return ""
			},
			wantErr: true,
		},
		{
			name: "dot_relative_path",
			setupDir: func(t *testing.T) string {
				tmpDir := t.TempDir()
				oldCwd, err := os.Getwd()
				if err != nil {
					t.Fatalf("failed to get cwd: %v", err)
				}
				err = os.Chdir(tmpDir)
				if err != nil {
					t.Fatalf("failed to chdir: %v", err)
				}
				t.Cleanup(func() {
					_ = os.Chdir(oldCwd)
				})
				return "."
			},
			wantEnv: "",
			wantErr: false,
		},
		{
			name: "parent_relative_path",
			setupDir: func(t *testing.T) string {
				tmpDir := t.TempDir()
				subDir := filepath.Join(tmpDir, "subdir")
				err := os.Mkdir(subDir, 0o750)
				if err != nil {
					t.Fatalf(
						"failed to create subdir: %v",
						err,
					)
				}
				oldCwd, err := os.Getwd()
				if err != nil {
					t.Fatalf("failed to get cwd: %v", err)
				}
				err = os.Chdir(subDir)
				if err != nil {
					t.Fatalf("failed to chdir: %v", err)
				}
				t.Cleanup(func() {
					_ = os.Chdir(oldCwd)
				})
				return ".."
			},
			wantEnv: "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rootDir := tt.setupDir(t)

			dir, env, err := ParseRootDir(rootDir)

			if tt.wantErr {
				assert.Error(t, err)
				if tt.errIs != nil {
					assert.ErrorIs(t, err, tt.errIs)
				}
				return
			}

			assert.NoError(t, err)
			assert.NotEmpty(t, dir)
			assert.DirExists(t, dir)
			assert.Equal(t, tt.wantEnv, env)
			assert.True(t, filepath.IsAbs(dir))
		})
	}
}
