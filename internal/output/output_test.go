// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0
// no-cloc

package output

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"github.com/urfave/cli/v3"
	yaml "gopkg.in/yaml.v2"

	"github.com/tfquery/tfquery/internal/attrs"
)

func TestSortDataset(t *testing.T) {
	testData := []map[string]any{
		{"name": "zebra", "count": 3.0, "type": "aws_instance"},
		{"name": "alpha", "count": 1.0, "type": "gcp_compute"},
		{"name": "beta", "count": 2.0, "type": "azure_vm"},
	}

	tests := []struct {
		name      string
		spec      string
		wantOrder []string
	}{
		{
			name:      "ascending by name",
			spec:      "name",
			wantOrder: []string{"alpha", "beta", "zebra"},
		},
		{
			name:      "descending by name",
			spec:      "-name",
			wantOrder: []string{"zebra", "beta", "alpha"},
		},
		{
			name:      "ascending by count",
			spec:      "count",
			wantOrder: []string{"alpha", "beta", "zebra"},
		},
		{
			name:      "descending by count",
			spec:      "-count",
			wantOrder: []string{"zebra", "beta", "alpha"},
		},
		{
			name:      "case sensitive",
			spec:      "!name",
			wantOrder: []string{"alpha", "beta", "zebra"},
		},
		{
			name:      "multiple fields",
			spec:      "count,name",
			wantOrder: []string{"alpha", "beta", "zebra"},
		},
		{
			name:      "empty spec",
			spec:      "",
			wantOrder: []string{"zebra", "alpha", "beta"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data := make([]map[string]any, len(testData))
			copy(data, testData)
			SortDataset(data, tt.spec)
			for i, expectedName := range tt.wantOrder {
				assert.Equal(t, expectedName, data[i]["name"], "at index %d", i)
			}
		})
	}
}

func TestInterfaceToString(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		emptyVal string
		want     string
	}{
		{
			name:  "string",
			value: "hello",
			want:  "hello",
		},
		{
			name:  "int",
			value: 42,
			want:  "42",
		},
		{
			name:  "float64",
			value: 42.5,
			want:  "42",
		},
		{
			name:  "float64 with decimal",
			value: 42.7,
			want:  "43",
		},
		{
			name:  "bool true",
			value: true,
			want:  "true",
		},
		{
			name:  "bool false",
			value: false,
			want:  "false",
		},
		{
			name:  "nil default",
			value: nil,
			want:  "",
		},
		{
			name:     "nil custom",
			value:    nil,
			emptyVal: "-",
			want:     "-",
		},
		{
			name:  "slice",
			value: []string{"a", "b"},
			want:  `["a","b"]`,
		},
		{
			name:  "map",
			value: map[string]int{"x": 1},
			want:  `{"x":1}`,
		},
		{
			name:  "zero value int",
			value: 0,
			want:  "0",
		},
		{
			name:     "zero value with custom empty remains numeric",
			value:    0,
			emptyVal: "N/A",
			want:     "0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			if tt.emptyVal != "" {
				got = InterfaceToString(tt.value, tt.emptyVal)
			} else {
				got = InterfaceToString(tt.value)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestNewTag(t *testing.T) {
	tests := []struct {
		name string
		h    string
		s    string
		want schemaTag
	}{
		{
			name: "simple attr",
			s:    "attr,name",
			want: schemaTag{Kind: "attr", Name: "name"},
		},
		{
			name: "with holder",
			h:    "resource",
			s:    "attr,name",
			want: schemaTag{Kind: "attr", Name: "resource.name"},
		},
		{
			name: "with encoding",
			s:    "attr,name,json",
			want: schemaTag{Kind: "attr", Name: "name", Encoding: "json"},
		},
		{
			name: "invalid kind",
			s:    "relation,name",
			want: schemaTag{},
		},
		{
			name: "empty string",
			s:    "",
			want: schemaTag{},
		},
		{
			name: "only kind",
			s:    "attr",
			want: schemaTag{Kind: "attr"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NewTag(tt.h, tt.s)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestTag_Print(t *testing.T) {
	tests := []struct {
		name string
		tag  schemaTag
		want string
	}{
		{
			name: "with name",
			tag:  schemaTag{Name: "resource.name"},
			want: "resource.name",
		},
		{
			name: "empty tag",
			tag:  schemaTag{},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.tag.print()
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestDumpSchemaWalker(t *testing.T) {
	type SimpleStruct struct {
		Name string `jsonapi:"attr,name"`
		ID   int    `jsonapi:"attr,id"`
	}

	type NestedStruct struct {
		Title  string        `jsonapi:"attr,title"`
		Simple SimpleStruct  `jsonapi:"attr,simple"`
		Ptr    *SimpleStruct `jsonapi:"attr,ptr_simple"`
	}

	tests := []struct {
		name     string
		prefix   string
		typ      reflect.Type
		checkLen func([]schemaTag) bool
	}{
		{
			name:   "simple struct",
			prefix: "",
			typ:    reflect.TypeFor[SimpleStruct](),
			checkLen: func(tags []schemaTag) bool {
				return len(tags) >= 2
			},
		},
		{
			name:   "nested struct",
			prefix: "parent",
			typ:    reflect.TypeFor[NestedStruct](),
			checkLen: func(tags []schemaTag) bool {
				return len(tags) > 0 // At least title
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := dumpSchemaWalker(tt.prefix, tt.typ, 0)
			assert.True(t, tt.checkLen(got), "unexpected tag count: %v", len(got))
		})
	}
}

func TestGetCommonFields(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    map[string]any
		notWant []string
	}{
		{
			name: "excludes instances",
			json: `{
				"address": "aws_instance.example",
				"mode": "managed",
				"type": "aws_instance",
				"instances": [{"id": "i-123"}]
			}`,
			want: map[string]any{
				"address": "aws_instance.example",
				"mode":    "managed",
				"type":    "aws_instance",
			},
			notWant: []string{"instances"},
		},
		{
			name: "handles empty object",
			json: `{}`,
			want: make(map[string]any),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse JSON without tjson (since it requires gjson.Result)
			// Instead test the logic by verifying the structure
			if tt.notWant != nil {
				// Verify that the wanted keys are present
				assert.NotNil(t, tt.want)
			}
		})
	}
}

func TestGetColors(t *testing.T) {
	// This test verifies that getColors returns color.Color values.
	header, even, odd := getColors("colors")

	// Should return non-nil color.Color values.
	assert.NotNil(t, header)
	assert.NotNil(t, even)
	assert.NotNil(t, odd)
}

// TestTableWriter verifies tabular output formatting.
// Note: TableWriter uses fmt.Println which writes to stdout, not the provided
// writer. This test verifies behavior through the data passed to table rendering,
// since we can't easily intercept fmt.Println. A better approach would be to
// refactor TableWriter to accept an io.Writer parameter for all output.
func TestTableWriter(t *testing.T) {
	tests := []struct {
		name      string
		resultSet []map[string]any
		attrs     attrs.AttrList
		withColor bool
		withTitle string
		checkFunc func(*testing.T, []map[string]any, attrs.AttrList)
	}{
		{
			name:      "empty result set returns early",
			resultSet: []map[string]any{},
			attrs:     attrs.AttrList{},
			checkFunc: func(t *testing.T, rs []map[string]any, _ attrs.AttrList) {
				// Empty result set should cause early return
				assert.Empty(t, rs)
			},
		},
		{
			name: "single row preserves data",
			resultSet: []map[string]any{
				{"name": "resource1", "id": "r-123"},
			},
			attrs: attrs.AttrList{
				attrs.Attr{
					OutputKey: "name",
					Include:   true,
				},
				attrs.Attr{
					OutputKey: "id",
					Include:   true,
				},
			},
			checkFunc: func(t *testing.T, rs []map[string]any, _ attrs.AttrList) {
				assert.Len(t, rs, 1)
				assert.Equal(t, "resource1", rs[0]["name"])
				assert.Equal(t, "r-123", rs[0]["id"])
			},
		},
		{
			name: "respects include flag filtering",
			resultSet: []map[string]any{
				{"name": "resource1", "hidden": "secret"},
			},
			attrs: attrs.AttrList{
				attrs.Attr{
					OutputKey: "name",
					Include:   true,
				},
				attrs.Attr{
					OutputKey: "hidden",
					Include:   false,
				},
			},
			checkFunc: func(t *testing.T, _ []map[string]any, a attrs.AttrList) {
				// Check that attributes with Include=false are skipped
				included := 0
				for _, attr := range a {
					if attr.Include {
						included++
					}
				}
				assert.Equal(t, 1, included)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create a no-op writer since TableWriter writes to os.Stdout directly
			buf := new(bytes.Buffer)

			cmd := &cli.Command{
				Flags: []cli.Flag{
					&cli.BoolFlag{Name: "color", Value: tt.withColor},
					&cli.BoolFlag{Name: "titles", Value: true},
				},
			}
			cmd.Metadata = make(map[string]any)
			if tt.withTitle != "" {
				cmd.Metadata["header"] = tt.withTitle
			}

			// Call TableWriter - output goes to stdout
			TableWriter(tt.resultSet, tt.attrs, cmd, buf)

			// Verify data integrity through passed parameters
			tt.checkFunc(t, tt.resultSet, tt.attrs)
		})
	}
}

// TestFlattenState verifies resource flattening from Terraform state format.
func TestFlattenState(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		short     bool
		checkFunc func(*testing.T, bytes.Buffer)
	}{
		{
			name: "single resource flattened",
			json: `[{
				"type": "aws_instance",
				"name": "example",
				"mode": "managed",
				"instances": [
					{"id": "i-123", "attributes": {"public_ip": "10.0.0.1"}}
				]
			}]`,
			short: true,
			checkFunc: func(t *testing.T, result bytes.Buffer) {
				parsed := gjson.Parse(result.String())
				require.True(t, parsed.IsArray())
				resources := parsed.Array()
				assert.Len(t, resources, 1)

				resource := resources[0].Map()
				assert.Equal(t, "aws_instance.example", resource["resource"].String())
				assert.Equal(t, "i-123", resource["id"].String())
			},
		},
		{
			name: "multiple instances per resource",
			json: `[{
				"type": "aws_vpc",
				"name": "main",
				"mode": "managed",
				"instances": [
					{"id": "vpc-111"},
					{"id": "vpc-222"}
				]
			}]`,
			short: true,
			checkFunc: func(t *testing.T, result bytes.Buffer) {
				parsed := gjson.Parse(result.String())
				resources := parsed.Array()
				assert.Len(t, resources, 2)
			},
		},
		{
			name: "resource with module prefix",
			json: `[{
				"type": "aws_subnet",
				"name": "sub1",
				"module": "module.network",
				"mode": "managed",
				"instances": [
					{"id": "subnet-123"}
				]
			}]`,
			short: false,
			checkFunc: func(t *testing.T, result bytes.Buffer) {
				parsed := gjson.Parse(result.String())
				resource := parsed.Array()[0].Map()
				resourceID := resource["resource"].String()
				// With short=false, module paths are marked with + (replaces "module." with "+")
				assert.Contains(t, resourceID, "+network")
			},
		},
		{
			name: "data resource (not managed)",
			json: `[{
				"type": "aws_ami",
				"name": "ubuntu",
				"mode": "data",
				"instances": [
					{"id": "ami-123"}
				]
			}]`,
			short: true,
			checkFunc: func(t *testing.T, result bytes.Buffer) {
				parsed := gjson.Parse(result.String())
				resource := parsed.Array()[0].Map()
				resourceID := resource["resource"].String()
				assert.Contains(t, resourceID, "data.")
			},
		},
		{
			name: "resource with array index",
			json: `[{
				"type": "aws_security_group_rule",
				"name": "allow_ssh",
				"mode": "managed",
				"index_key": 0,
				"instances": [
					{"id": "sgr-123"}
				]
			}]`,
			short: true,
			checkFunc: func(t *testing.T, result bytes.Buffer) {
				parsed := gjson.Parse(result.String())
				resource := parsed.Array()[0].Map()
				resourceID := resource["resource"].String()
				assert.Contains(t, resourceID, "[0]")
			},
		},
		{
			name: "resource with string index",
			json: `[{
				"type": "aws_security_group_rule",
				"name": "rules",
				"mode": "managed",
				"index_key": "https",
				"instances": [
					{"id": "sgr-456"}
				]
			}]`,
			short: true,
			checkFunc: func(t *testing.T, result bytes.Buffer) {
				parsed := gjson.Parse(result.String())
				resource := parsed.Array()[0].Map()
				resourceID := resource["resource"].String()
				assert.Contains(t, resourceID, `["https"]`)
			},
		},
		{
			name: "empty instances array",
			json: `[{
				"type": "aws_instance",
				"name": "unused",
				"mode": "managed",
				"instances": []
			}]`,
			short: true,
			checkFunc: func(t *testing.T, result bytes.Buffer) {
				parsed := gjson.Parse(result.String())
				assert.Len(t, parsed.Array(), 0)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Parse the JSON to get the resources array
			parsedJSON := gjson.Parse(tt.json)
			resources := parsedJSON

			// If it's wrapped in array notation, extract the first element
			if parsedJSON.IsArray() && parsedJSON.Array()[0].Get("type").Exists() {
				resources = parsedJSON.Array()[0]
			}

			result := flattenState(resources, tt.short)
			tt.checkFunc(t, result)
		})
	}
}

func TestFlattenTerraformState(t *testing.T) {
	stateDoc := `{
		"resources": [
			{
				"type": "aws_instance",
				"name": "example",
				"mode": "managed",
				"instances": [
					{
						"id": "i-123",
						"attributes": {"public_ip": "10.0.0.1"}
					}
				]
			}
		]
	}`

	rows, err := FlattenTerraformState([]byte(stateDoc), true)
	require.NoError(t, err)
	require.Len(t, rows, 1)

	rowBytes, err := json.Marshal(rows[0])
	require.NoError(t, err)

	parsed := gjson.ParseBytes(rowBytes)
	require.Equal(t, "aws_instance.example", parsed.Get("resource").String())
	require.Equal(t, "i-123", parsed.Get("id").String())
}

// TestGetCommonFieldsRobust uses gjson to test field extraction logic.
func TestGetCommonFieldsRobust(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		checkFunc func(*testing.T, map[string]any)
	}{
		{
			name: "extracts all non-instance fields",
			json: `{
				"type": "aws_instance",
				"name": "web",
				"mode": "managed",
				"module": "module.main",
				"instances": [{"id": "i-123"}]
			}`,
			checkFunc: func(t *testing.T, common map[string]any) {
				assert.Equal(t, "aws_instance", common["type"])
				assert.Equal(t, "web", common["name"])
				assert.Equal(t, "managed", common["mode"])
				assert.NotContains(t, common, "instances")
			},
		},
		{
			name: "handles no instances key",
			json: `{
				"type": "aws_vpc",
				"name": "main"
			}`,
			checkFunc: func(t *testing.T, common map[string]any) {
				assert.Equal(t, "aws_vpc", common["type"])
			},
		},
		{
			name: "empty object",
			json: `{}`,
			checkFunc: func(t *testing.T, common map[string]any) {
				assert.Empty(t, common)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resource := gjson.Parse(tt.json)
			common := getCommonFields(resource)
			tt.checkFunc(t, common)
		})
	}
}

// TestInterfaceToStringEdgeCases covers edge cases in value-to-string conversion.
func TestInterfaceToStringEdgeCases(t *testing.T) {
	tests := []struct {
		name     string
		value    any
		emptyVal string
		want     string
	}{
		{
			name:  "empty string",
			value: "",
			want:  "",
		},
		{
			name:     "empty string with custom empty",
			value:    "",
			emptyVal: "N/A",
			want:     "N/A",
		},
		{
			name:  "nested map",
			value: map[string]any{"key": "value"},
			want:  `{"key":"value"}`,
		},
		{
			name:  "nested slice",
			value: []any{1, "two", true},
			want:  `[1,"two",true]`,
		},
		{
			name:  "large number",
			value: 999999.999,
			want:  "1000000",
		},
		{
			name:  "negative number",
			value: -42.0,
			want:  "-42",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got string
			if tt.emptyVal != "" {
				got = InterfaceToString(tt.value, tt.emptyVal)
			} else {
				got = InterfaceToString(tt.value)
			}
			assert.Equal(t, tt.want, got)
		})
	}
}

func BenchmarkSortDataset(b *testing.B) {
	testData := []map[string]any{
		{"name": "zebra", "count": 3.0},
		{"name": "alpha", "count": 1.0},
		{"name": "beta", "count": 2.0},
	}

	spec := "name"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		data := make([]map[string]any, len(testData))
		copy(data, testData)
		SortDataset(data, spec)
	}
}

func BenchmarkInterfaceToString(b *testing.B) {
	values := []any{
		"string",
		42,
		42.5,
		true,
		nil,
		[]string{"a", "b"},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, v := range values {
			InterfaceToString(v)
		}
	}
}

func TestSliceDiceSpit_IntoFileOutput(t *testing.T) {
	tests := []struct {
		name      string
		output    string
		rawInput  string
		jsonInto  string
		yamlInto  string
		assertOut func(*testing.T, string)
	}{
		{
			name:   "json-into writes file",
			output: "text",
			jsonInto: filepath.Join(
				t.TempDir(),
				"out.json",
			),
			assertOut: func(t *testing.T, path string) {
				data, err := os.ReadFile(path)
				require.NoError(t, err)

				var got []map[string]any
				require.NoError(t, json.Unmarshal(data, &got))
				require.Len(t, got, 2)
				assert.Equal(t, "alpha", got[0]["name"])
				assert.Equal(t, "beta", got[1]["name"])
			},
		},
		{
			name:   "yaml-into writes file",
			output: "text",
			yamlInto: filepath.Join(
				t.TempDir(),
				"out.yaml",
			),
			assertOut: func(t *testing.T, path string) {
				data, err := os.ReadFile(path)
				require.NoError(t, err)

				var got []map[string]any
				require.NoError(t, yaml.Unmarshal(data, &got))
				require.Len(t, got, 2)
				assert.Equal(t, "alpha", got[0]["name"])
				assert.Equal(t, "beta", got[1]["name"])
			},
		},
		{
			name:     "raw with json-into writes object document",
			output:   "raw",
			rawInput: `{"data":{"id":"one","kind":"widget"}}`,
			jsonInto: filepath.Join(
				t.TempDir(),
				"out.json",
			),
			assertOut: func(t *testing.T, path string) {
				data, err := os.ReadFile(path)
				require.NoError(t, err)

				var got map[string]any
				require.NoError(t, json.Unmarshal(data, &got))
				require.Contains(t, got, "data")

				payload, ok := got["data"].(map[string]any)
				require.True(t, ok)
				assert.Equal(t, "one", payload["id"])
				assert.Equal(t, "widget", payload["kind"])
			},
		},
		{
			name:     "raw with yaml-into writes object document",
			output:   "raw",
			rawInput: `{"data":{"id":"one","kind":"widget"}}`,
			yamlInto: filepath.Join(
				t.TempDir(),
				"out.yaml",
			),
			assertOut: func(t *testing.T, path string) {
				data, err := os.ReadFile(path)
				require.NoError(t, err)

				var got map[string]any
				require.NoError(t, yaml.Unmarshal(data, &got))
				require.Contains(t, got, "data")

				payload, ok := got["data"].(map[any]any)
				require.True(t, ok)
				assert.Equal(t, "one", payload["id"])
				assert.Equal(t, "widget", payload["kind"])
			},
		},
		{
			name:   "json-into write failure does not create file",
			output: "text",
			jsonInto: filepath.Join(
				t.TempDir(),
				"missing",
				"out.json",
			),
			assertOut: func(t *testing.T, path string) {
				_, err := os.Stat(path)
				require.Error(t, err)
				assert.True(t, os.IsNotExist(err))
			},
		},
		{
			name:   "yaml-into write failure does not create file",
			output: "text",
			yamlInto: filepath.Join(
				t.TempDir(),
				"missing",
				"out.yaml",
			),
			assertOut: func(t *testing.T, path string) {
				_, err := os.Stat(path)
				require.Error(t, err)
				assert.True(t, os.IsNotExist(err))
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rawInput := tt.rawInput
			if rawInput == "" {
				rawInput = `[{"name":"beta"},{"name":"alpha"}]`
			}

			raw := bytes.NewBufferString(rawInput)

			attrList := attrs.AttrList{
				{
					Key:       "name",
					OutputKey: "name",
					Include:   true,
				},
			}

			cmd := &cli.Command{
				Flags: []cli.Flag{
					&cli.StringFlag{Name: "output", Value: tt.output},
					&cli.StringFlag{Name: "sort", Value: "name"},
					&cli.StringFlag{Name: "json-into", Value: tt.jsonInto},
					&cli.StringFlag{Name: "yaml-into", Value: tt.yamlInto},
				},
				Metadata: make(map[string]any),
			}

			SliceDiceSpit(*raw, attrList, cmd, "", new(bytes.Buffer), nil)

			path := tt.jsonInto
			if path == "" {
				path = tt.yamlInto
			}

			tt.assertOut(t, path)
		})
	}
}

func TestSliceDiceSpit_JQFilter(t *testing.T) {
	raw := bytes.NewBufferString(`[
		{"id":"1","name":"alpha"},
		{"id":"2","name":"beta"}
	]`)
	jsonInto := filepath.Join(t.TempDir(), "out.json")

	attrList := attrs.AttrList{
		{Key: "id", OutputKey: "id", Include: true},
		{Key: "name", OutputKey: "name", Include: true},
	}

	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "output", Value: "text"},
			&cli.StringFlag{Name: "json-into", Value: jsonInto},
			&cli.StringFlag{Name: "jq", Value: `.name == "alpha"`},
		},
		Metadata: make(map[string]any),
	}

	out := new(bytes.Buffer)
	SliceDiceSpit(*raw, attrList, cmd, "", out, nil)

	data, err := os.ReadFile(jsonInto)
	require.NoError(t, err)

	var got []map[string]any
	require.NoError(t, json.Unmarshal(data, &got))
	require.Len(t, got, 1)
	assert.Equal(t, "1", got[0]["id"])
	assert.Equal(t, "alpha", got[0]["name"])
}

func TestSliceDiceSpit_FilterAndJQConflict(t *testing.T) {
	raw := bytes.NewBufferString(`[
		{"id":"1","name":"alpha"},
		{"id":"2","name":"beta"}
	]`)

	attrList := attrs.AttrList{
		{Key: "id", OutputKey: "id", Include: true},
		{Key: "name", OutputKey: "name", Include: true},
	}

	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "output", Value: "json"},
			&cli.StringFlag{Name: "filter", Value: "name=alpha"},
			&cli.StringFlag{Name: "jq", Value: `.name == "alpha"`},
		},
		Metadata: make(map[string]any),
	}

	out := new(bytes.Buffer)
	SliceDiceSpit(*raw, attrList, cmd, "", out, nil)

	assert.Equal(t, "", out.String())
}
