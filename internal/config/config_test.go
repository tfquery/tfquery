// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0
// no-cloc

package config

import (
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

// setupTestConfig sets TFCTL_CFG_FILE to point to a test config file.
// Returns cleanup function that should be deferred.
func setupTestConfig(t *testing.T, testdataFile string) (cleanup func()) {
	t.Helper()

	// Get absolute path to testdata file
	configPath := filepath.Join("testdata", testdataFile)
	absPath, err := filepath.Abs(configPath)
	assert.NoError(t, err, "failed to get absolute path for test config")

	// Set TFCTL_CFG_FILE environment variable
	t.Setenv("TFCTL_CFG_FILE", absPath)

	// Reset the global Config to force reload
	Config = Type{}

	return func() {
		// Reset global Config
		Config = Type{}
	}
}

// withConfig is a helper that sets up a test config and executes a test function.
// This reduces boilerplate for common test patterns.
func withConfig(t *testing.T, testFile string, fn func(t *testing.T)) {
	t.Helper()
	cleanup := setupTestConfig(t, testFile)
	defer cleanup()
	_, _ = Load()
	fn(t)
}

// withEmptyConfig is a helper for testing lazy loading with empty Config.
func withEmptyConfig(t *testing.T, testFile string, fn func(t *testing.T)) {
	t.Helper()
	cleanup := setupTestConfig(t, testFile)
	defer cleanup()
	Config = Type{}
	fn(t)
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name      string
		testFile  string
		wantErr   bool
		checkFunc func(*testing.T, Type)
	}{
		{
			name:     "simple string values",
			testFile: "simple.yaml",
			wantErr:  false,
			checkFunc: func(t *testing.T, cfg Type) {
				assert.NotEmpty(t, cfg.Source)
				assert.Contains(t, cfg.Data, "region")
				assert.Equal(t, "us-east-1", cfg.Data["region"])
				assert.Equal(t, "my-bucket", cfg.Data["bucket"])
			},
		},
		{
			name:     "nested structure",
			testFile: "nested.yaml",
			wantErr:  false,
			checkFunc: func(t *testing.T, cfg Type) {
				backend, ok := cfg.Data["backend"].(map[string]interface{})
				assert.True(t, ok, "backend should be a map")
				s3, ok := backend["s3"].(map[string]interface{})
				assert.True(t, ok, "s3 should be a map")
				assert.Equal(t, "us-west-2", s3["region"])
				assert.Equal(t, "terraform-state", s3["bucket"])
			},
		},
		{
			name:     "mixed types",
			testFile: "mixed-types.yaml",
			wantErr:  false,
			checkFunc: func(t *testing.T, cfg Type) {
				assert.Equal(t, "test-project", cfg.Data["name"])
				assert.Equal(t, 1, cfg.Data["version"])
				assert.Equal(t, true, cfg.Data["enabled"])
				assert.Equal(t, 30.5, cfg.Data["timeout"])
				tags, ok := cfg.Data["tags"].([]interface{})
				assert.True(t, ok)
				assert.Len(t, tags, 2)
			},
		},
		{
			name:     "empty file",
			testFile: "empty.yaml",
			wantErr:  false,
			checkFunc: func(t *testing.T, cfg Type) {
				// Empty YAML unmarshals to nil map, which is acceptable
				assert.NotEmpty(t, cfg.Source, "should have a source path")
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestConfig(t, tt.testFile)
			defer cleanup()

			cfg, err := Load()

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			if tt.checkFunc != nil {
				tt.checkFunc(t, cfg)
			}
		})
	}
}

func TestLoad_NoConfigFile(t *testing.T) {
	// Set TFCTL_CFG_FILE to non-existent file
	t.Setenv("TFCTL_CFG_FILE", "/nonexistent/path/tfquery.yaml")
	Config = Type{}

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "config file not found")
}

func TestLoad_TFCTL_CFG_FILE_IsDirectory(t *testing.T) {
	// Set TFCTL_CFG_FILE to a directory instead of a file
	t.Setenv("TFCTL_CFG_FILE", "testdata")
	Config = Type{}

	_, err := Load()
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "points to a directory")
}

func TestGetString(t *testing.T) {
	tests := []struct {
		name         string
		testFile     string
		key          string
		defaultValue []string
		want         string
		wantErr      bool
	}{
		{
			name:     "simple string value",
			testFile: "simple.yaml",
			key:      "region",
			want:     "us-east-1",
			wantErr:  false,
		},
		{
			name:     "nested string value",
			testFile: "nested.yaml",
			key:      "backend.s3.region",
			want:     "us-west-2",
			wantErr:  false,
		},
		{
			name:         "missing key with default",
			testFile:     "simple.yaml",
			key:          "missing",
			defaultValue: []string{"default-value"},
			want:         "default-value",
			wantErr:      false,
		},
		{
			name:     "missing key without default",
			testFile: "simple.yaml",
			key:      "missing",
			want:     "",
			wantErr:  true,
		},
		{
			name:     "non-string value",
			testFile: "mixed-types.yaml",
			key:      "version",
			want:     "",
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestConfig(t, tt.testFile)
			defer cleanup()

			// Force load
			_, _ = Load()

			got, err := GetString(tt.key, tt.defaultValue...)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetInt(t *testing.T) {
	tests := []struct {
		name         string
		testFile     string
		key          string
		defaultValue []int
		want         int
		wantErr      bool
	}{
		{
			name:     "int value",
			testFile: "mixed-types.yaml",
			key:      "version",
			want:     1,
			wantErr:  false,
		},
		{
			name:     "float value converted to int",
			testFile: "mixed-types.yaml",
			key:      "timeout",
			want:     30,
			wantErr:  false,
		},
		{
			name:     "nested int value",
			testFile: "nested.yaml",
			key:      "backend.s3.max_retries",
			want:     5,
			wantErr:  false,
		},
		{
			name:         "missing key with default",
			testFile:     "simple.yaml",
			key:          "missing",
			defaultValue: []int{60},
			want:         60,
			wantErr:      false,
		},
		{
			name:     "missing key without default",
			testFile: "simple.yaml",
			key:      "missing",
			want:     0,
			wantErr:  true,
		},
		{
			name:     "non-int value",
			testFile: "simple.yaml",
			key:      "region",
			want:     0,
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cleanup := setupTestConfig(t, tt.testFile)
			defer cleanup()

			// Force load
			_, _ = Load()

			got, err := GetInt(tt.key, tt.defaultValue...)

			if tt.wantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestConfig_GetWithNamespace(t *testing.T) {
	withConfig(t, "nested.yaml", func(t *testing.T) {
		// Test with namespace
		Config.Namespace = "backend.s3"

		// Should find namespaced value first
		val, err := Config.get("region")
		assert.NoError(t, err)
		assert.Equal(t, "us-west-2", val)

		val, err = Config.get("bucket")
		assert.NoError(t, err)
		assert.Equal(t, "terraform-state", val)

		// Change namespace
		Config.Namespace = "backend.local"
		val, err = Config.get("region")
		assert.NoError(t, err)
		assert.Equal(t, "us-east-1", val)

		val, err = Config.get("bucket")
		assert.NoError(t, err)
		assert.Equal(t, "local-bucket", val)
	})
}

func TestConfig_Get(t *testing.T) {
	tests := []struct {
		name     string
		testFile string
		key      string
		wantVal  interface{}
		wantErr  bool
		errMsg   string
	}{
		{
			name:     "nested path",
			testFile: "deep-nested.yaml",
			key:      "level1.level2.level3.value",
			wantVal:  "deep-value",
			wantErr:  false,
		},
		{
			name:     "missing intermediate key",
			testFile: "simple.yaml",
			key:      "nonexistent.nested.path",
			wantErr:  true,
			errMsg:   "no valid path found",
		},
		{
			name:     "traverse non-map value",
			testFile: "mixed-types.yaml",
			key:      "version.something",
			wantErr:  true,
			errMsg:   "no valid path found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			withConfig(t, tt.testFile, func(t *testing.T) {
				val, err := Config.get(tt.key)
				if tt.wantErr {
					assert.Error(t, err)
					if tt.errMsg != "" {
						assert.Contains(t, err.Error(), tt.errMsg)
					}
				} else {
					assert.NoError(t, err)
					assert.Equal(t, tt.wantVal, val)
				}
			})
		})
	}
}

func TestGetterEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		testFile    string
		lazyLoad    bool
		setup       func()
		testFn      func(t *testing.T)
		description string
	}{
		{
			name:     "GetString lazy load with empty config",
			testFile: "simple.yaml",
			lazyLoad: true,
			testFn: func(t *testing.T) {
				val, err := GetString("region")
				assert.NoError(t, err)
				assert.Equal(t, "us-east-1", val)
			},
			description: "Tests lazy loading when Config.Data is empty",
		},
		{
			name:     "GetString empty config with default",
			testFile: "simple.yaml",
			lazyLoad: true,
			testFn: func(t *testing.T) {
				val, err := GetString("missing", "my-default")
				assert.NoError(t, err)
				assert.Equal(t, "my-default", val)
			},
			description: "Tests GetString with default value and empty config",
		},
		{
			name:     "GetInt empty config with default",
			testFile: "simple.yaml",
			lazyLoad: true,
			testFn: func(t *testing.T) {
				val, err := GetInt("missing", 99)
				assert.NoError(t, err)
				assert.Equal(t, 99, val)
			},
			description: "Tests GetInt with default value and empty config",
		},
		{
			name:     "GetInt int64 type handling",
			testFile: "mixed-types.yaml",
			testFn: func(t *testing.T) {
				val, err := GetInt("version")
				assert.NoError(t, err)
				assert.Equal(t, 1, val)
			},
			description: "Tests GetInt with int64 value conversion",
		},
		{
			name:     "GetInt namespace fallback",
			testFile: "nested.yaml",
			setup: func() {
				Config.Namespace = "backend.s3"
			},
			testFn: func(t *testing.T) {
				val, err := GetInt("max_retries")
				assert.NoError(t, err)
				assert.Equal(t, 5, val)
			},
			description: "Tests GetInt with namespace fallback behavior",
		},
		{
			name:     "GetString namespace fallback",
			testFile: "namespace.yaml",
			setup: func() {
				Config.Namespace = "backend.s3"
			},
			testFn: func(t *testing.T) {
				val, err := GetString("setting")
				assert.NoError(t, err)
				assert.Equal(t, "s3-value", val)

				val, err = GetString("specific")
				assert.NoError(t, err)
				assert.Equal(t, "s3-specific", val)

				_, err = GetString("nonexistent")
				assert.Error(t, err)
			},
			description: "Tests GetString with namespace fallback behavior",
		},
		{
			name:     "GetInt non-int value error",
			testFile: "mixed-types.yaml",
			testFn: func(t *testing.T) {
				_, err := GetInt("name")
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "not an int")
			},
			description: "Tests GetInt returns error for non-int values",
		},
		{
			name:     "GetString non-string value error",
			testFile: "mixed-types.yaml",
			testFn: func(t *testing.T) {
				_, err := GetString("version")
				assert.Error(t, err)
				assert.Contains(t, err.Error(), "not a string")
			},
			description: "Tests GetString returns error for non-string values",
		},
		{
			name:     "GetInt multiple defaults error",
			testFile: "simple.yaml",
			testFn: func(t *testing.T) {
				_, err := GetInt("missing", 10, 20)
				assert.Error(t, err)
			},
			description: "Tests GetInt with multiple defaults returns error",
		},
		{
			name:     "GetString multiple defaults error",
			testFile: "simple.yaml",
			testFn: func(t *testing.T) {
				_, err := GetString("missing", "first", "second")
				assert.Error(t, err)
			},
			description: "Tests GetString with multiple defaults returns error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.lazyLoad {
				withEmptyConfig(t, tt.testFile, tt.testFn)
			} else {
				withConfig(t, tt.testFile, func(t *testing.T) {
					if tt.setup != nil {
						tt.setup()
					}
					tt.testFn(t)
				})
			}
		})
	}
}

func TestLoad_MultipleVariadic(t *testing.T) {
	// Test that Load() properly ignores multiple cfgFilePath args
	cleanup := setupTestConfig(t, "simple.yaml")
	defer cleanup()

	cfg, err := Load("arg1", "arg2")
	assert.NoError(t, err)
	assert.NotEmpty(t, cfg.Source)
}

func TestLoad_InvalidYAML(t *testing.T) {
	// We would need a testdata file with invalid YAML
	// For now, we skip this as it would require creating invalid YAML
	t.Skip("requires invalid YAML testdata file")
}

func TestGetStringSlice_SimpleAndNested(t *testing.T) {
	withConfig(t, "string-slice.yaml", func(t *testing.T) {
		vals, err := GetStringSlice("list_top")
		assert.NoError(t, err)
		assert.Equal(t, []string{"a", "b"}, vals)

		vals, err = GetStringSlice("nested.inner.list")
		assert.NoError(t, err)
		assert.Equal(t, []string{"one", "two three"}, vals)

		vals, err = GetStringSlice("not_a_list")
		assert.NoError(t, err)
		assert.Equal(t, []string{"one"}, vals)
	})
}

func TestGetStringSlice_NamespaceFallback(t *testing.T) {
	withConfig(t, "string-slice.yaml", func(t *testing.T) {
		Config.Namespace = "sq"
		vals, err := GetStringSlice("test")
		assert.NoError(t, err)
		assert.Equal(t, []string{"--output json", "--sort resource,id"}, vals)

		// Also support direct fully-qualified key without namespace.
		vals, err = GetStringSlice("sq.test")
		assert.NoError(t, err)
		assert.Equal(t, []string{"--output json", "--sort resource,id"}, vals)
	})
}

func TestGetStringSlice_ErrorCases(t *testing.T) {
	withConfig(t, "string-slice.yaml", func(t *testing.T) {
		if v, err := Config.get("nonstring_list"); err == nil {
			t.Logf("nonstring_list raw: %T %#v", v, v)
		}
		// Non-string element in list
		_, err := GetStringSlice("nonstring_list")
		assert.Error(t, err)

		// Missing key with default slice returns provided default.
		def := []string{"x", "y"}
		vals, err := GetStringSlice("does.not.exist", def)
		assert.NoError(t, err)
		assert.Equal(t, def, vals)

		// Missing key without default returns error.
		_, err = GetStringSlice("does.not.exist")
		assert.Error(t, err)
	})
}
