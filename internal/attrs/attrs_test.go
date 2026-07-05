// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0
// no-cloc

package attrs

import (
	"embed"
	"fmt"
	"testing"
	"time"

	"github.com/dustin/go-humanize"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"
)

//go:embed testdata/*.yaml
var testDataFS embed.FS

// testSetCase represents a single test case for TestAttrList_Set.
type testSetCase struct {
	Name      string `yaml:"name"`
	Initial   []Attr `yaml:"initial"`
	Value     string `yaml:"value"`
	WantLen   int    `yaml:"wantLen"`
	WantAttrs []Attr `yaml:"wantAttrs"`
	WantErr   bool   `yaml:"wantErr"`
}

// testTransformCase represents a single test case for TestAttr_Transform.
type testTransformCase struct {
	Name          string            `yaml:"name"`
	TransformSpec string            `yaml:"transformSpec"`
	Input         any               `yaml:"input"`
	EnvVars       map[string]string `yaml:"envVars"`
	Want          any               `yaml:"want"`
	Description   string            `yaml:"description"`
}

// testGlobalTransformCase represents a test case for SetGlobalTransformSpec.
type testGlobalTransformCase struct {
	Name      string   `yaml:"name"`
	Initial   []Attr   `yaml:"initial"`
	WantSpecs []string `yaml:"wantSpecs"`
	WantErr   bool     `yaml:"wantErr"`
}

// testStringCase represents a test case for AttrList_String.
type testStringCase struct {
	Name     string `yaml:"name"`
	AttrList []Attr `yaml:"attrList"`
	Want     string `yaml:"want"`
}

// loadTestData loads test data from embedded YAML files.
func loadTestData(filename string, v any) error {
	data, err := testDataFS.ReadFile("testdata/" + filename)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, v)
}

func TestAttrList_Set(t *testing.T) {
	var tests []testSetCase
	err := loadTestData("set_cases.yaml", &tests)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			a := AttrList(tt.Initial)
			err := a.Set(tt.Value)

			if tt.WantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, a, tt.WantLen)

			if tt.WantAttrs != nil {
				for i, want := range tt.WantAttrs {
					assert.Equal(t, want.Key, a[i].Key, "attr[%d].Key", i)
					assert.Equal(t, want.OutputKey, a[i].OutputKey, "attr[%d].OutputKey", i)
					assert.Equal(t, want.Include, a[i].Include, "attr[%d].Include", i)
					assert.Equal(t, want.TransformSpec, a[i].TransformSpec, "attr[%d].TransformSpec", i)
				}
			}
		})
	}
}

func TestAttrList_SetGlobalTransformSpec(t *testing.T) {
	var tests []testGlobalTransformCase
	err := loadTestData("global_transform_cases.yaml", &tests)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			a := AttrList(tt.Initial)
			err := a.SetGlobalTransformSpec()

			if tt.WantErr {
				assert.Error(t, err)
				return
			}

			assert.NoError(t, err)
			assert.Len(t, a, len(tt.WantSpecs))

			for i, wantSpec := range tt.WantSpecs {
				assert.Equal(t, wantSpec, a[i].TransformSpec, "attr[%d].TransformSpec", i)
			}
		})
	}
}

func TestAttr_Transform(t *testing.T) {
	var tests []testTransformCase
	err := loadTestData("transform_cases.yaml", &tests)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			// Set up environment variables
			for k, v := range tt.EnvVars {
				t.Setenv(k, v)
			}

			attr := Attr{TransformSpec: tt.TransformSpec}
			got := attr.Transform(tt.Input)

			// Handle dynamic expectations for time transforms that now rely on
			// the system's local time rather than TZ environment variables.
			if s, ok := tt.Want.(string); ok && s == "DYNAMIC_LOCAL_TIME" {
				in, ok := tt.Input.(string)
				require.True(t, ok, "input must be RFC3339 string")
				tParsed, err := time.Parse(time.RFC3339, in)
				require.NoError(t, err)
				loc := time.Now().Location()
				want := tParsed.In(loc).Format("2006-01-02T15:04:05MST")
				assert.Equal(t, want, got, "local time conversion should match")
				return
			}

			if s, ok := tt.Want.(string); ok && s == "DYNAMIC_RELATIVE_TIME" {
				in, ok := tt.Input.(string)
				require.True(t, ok, "input must be RFC3339 string")
				tParsed, err := time.Parse(time.RFC3339, in)
				require.NoError(t, err)
				loc := time.Now().Location()
				want := humanize.Time(tParsed.In(loc))
				assert.Equal(t, want, fmt.Sprintf("%v", got))
				return
			}

			assert.Equal(t, tt.Want, got)
		})
	}
}

func TestAttrList_String(t *testing.T) {
	var tests []testStringCase
	err := loadTestData("string_cases.yaml", &tests)
	require.NoError(t, err)

	for _, tt := range tests {
		t.Run(tt.Name, func(t *testing.T) {
			a := AttrList(tt.AttrList)
			got := a.String()
			assert.Equal(t, tt.Want, got)
		})
	}
}

func TestAttrList_Type(t *testing.T) {
	a := AttrList{}
	assert.Equal(t, "list", a.Type())
}

// We validate local time transformation using the system's current location
// only, with no dependence on TZ environment variables.
func TestAttr_Transform_Time_LocalUsesSystemZone(t *testing.T) {
	input := "2024-01-15T10:00:00Z"
	attr := Attr{TransformSpec: "t"}
	loc := time.Now().Location()
	got := fmt.Sprintf("%v", attr.Transform(input))

	tParsed, err := time.Parse(time.RFC3339, input)
	require.NoError(t, err)
	want := tParsed.In(loc).Format("2006-01-02T15:04:05MST")
	assert.Equal(t, want, got)
}
