// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

// no-cloc
package driller

import (
	"embed"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

//go:embed testdata/*.yaml
var testDataFS embed.FS

// drillerTestCase represents a single test case for TestDriller.
type drillerTestCase struct {
	Name        string         `yaml:"name"`
	JSON        map[string]any `yaml:"json"`
	Path        string         `yaml:"path"`
	ExpectedStr string         `yaml:"expectedStr"`
	IsNil       bool           `yaml:"isNil"`
	IsArray     bool           `yaml:"isArray"`
}

// loadTestData loads test data from embedded YAML files.
func loadTestData(filename string, v any) error {
	data, err := testDataFS.ReadFile("testdata/" + filename)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, v)
}

func TestDriller(t *testing.T) {
	var tests []drillerTestCase
	err := loadTestData("driller_cases.yaml", &tests)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Convert map to JSON string for Driller function.
			jsonBytes, err := json.Marshal(tt.JSON)
			require.NoError(t, err)
			result := Driller(string(jsonBytes), tt.Path)

			if tt.IsNil {
				// Result should not exist or be null
				if result.Exists() && result.Type.String() != "Null" {
					t.Errorf("Expected nil/empty result but got: %v", result.Value())
				}
				return
			}

			if !result.Exists() {
				t.Errorf("Expected result but got nil/empty")
				return
			}

			if tt.IsArray {
				if !result.IsArray() {
					t.Errorf("Expected array but got: %v (type: %T)", result.Value(), result.Value())
				}
				return
			}

			val := result.String()
			if val != tt.ExpectedStr {
				t.Errorf("Expected %q but got %q", tt.ExpectedStr, val)
			}
		})
	}
}
