// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0
// no-cloc

package command

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"

	"github.com/tfquery/tfquery/internal/attrs"
)

func TestFormatIacrootForOutput(t *testing.T) {
	tests := []struct {
		name     string
		iacroot  string
		baseDir  string
		relative bool
		want     string
	}{
		{
			name:     "absolute_when_relative_false",
			iacroot:  "/tmp/a/b",
			baseDir:  "/tmp/a",
			relative: false,
			want:     "/tmp/a/b",
		},
		{
			name:     "relative_when_enabled",
			iacroot:  "/tmp/a/b",
			baseDir:  "/tmp/a",
			relative: true,
			want:     "b",
		},
		{
			name:     "dot_when_same_path",
			iacroot:  "/tmp/a",
			baseDir:  "/tmp/a",
			relative: true,
			want:     ".",
		},
		{
			name:     "fallback_when_basedir_empty",
			iacroot:  "/tmp/a/b",
			baseDir:  "",
			relative: true,
			want:     "/tmp/a/b",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, transformIacroot(tt.iacroot, tt.baseDir, tt.relative))
		})
	}
}

func TestChopPrefix_EmptyDataset(t *testing.T) {
	data := []map[string]any{}
	chopPrefix(data)
	assert.Equal(t, 0, len(data))
}

func TestChopPrefix_NoStringValues(t *testing.T) {
	data := []map[string]any{
		{"count": 1},
		{"count": 2},
	}
	// No string values to process
	chopPrefix(data)
	assert.Equal(t, 1, data[0]["count"])
	assert.Equal(t, 2, data[1]["count"])
}

func TestChopPrefix_SingleValueAllCommonSegments(t *testing.T) {
	data := []map[string]any{
		{"resource": "aws.s3.bucket.prod"},
	}
	// Single entry: all its segments are trivially "common"
	// But chopping 2 segments would leave 2, which is allowed
	chopPrefix(data)
	assert.Equal(t, "..bucket.prod", data[0]["resource"])
}

func TestChopPrefix_TwoCommonLeadingSegments(t *testing.T) {
	data := []map[string]any{
		{"resource": "aws.s3.bucket1.x"},
		{"resource": "aws.s3.bucket2.x"},
		{"resource": "aws.s3.bucket3.x"},
	}
	// All entries have "aws.s3" as leading segments, so chop should remove it
	chopPrefix(data)
	assert.Equal(t, "..bucket1.x", data[0]["resource"])
	assert.Equal(t, "..bucket2.x", data[1]["resource"])
	assert.Equal(t, "..bucket3.x", data[2]["resource"])
}

func TestChopPrefix_DifferentThirdSegment(t *testing.T) {
	data := []map[string]any{
		{"resource": "aws.s3.prod.server1"},
		{"resource": "aws.s3.dev.server2"},
		{"resource": "aws.s3.staging.server3"},
	}
	// All have "aws.s3" in common, but differ on third segment
	// So only "aws.s3" is removed
	chopPrefix(data)
	assert.Equal(t, "..prod.server1", data[0]["resource"])
	assert.Equal(t, "..dev.server2", data[1]["resource"])
	assert.Equal(t, "..staging.server3", data[2]["resource"])
}

func TestChopPrefix_OneCommonSegmentOnly_NoChop(t *testing.T) {
	data := []map[string]any{
		{"resource": "aws.s3.bucket"},
		{"resource": "aws.ec2.instance"},
		{"resource": "aws.rds.database"},
	}
	// Only "aws" is common, but we require at least 2, so no change
	chopPrefix(data)
	assert.Equal(t, "aws.s3.bucket", data[0]["resource"])
	assert.Equal(t, "aws.ec2.instance", data[1]["resource"])
	assert.Equal(t, "aws.rds.database", data[2]["resource"])
}

func TestChopPrefix_NoCommonSegments_NoChop(t *testing.T) {
	data := []map[string]any{
		{"resource": "a.b.c"},
		{"resource": "x.y.z"},
		{"resource": "m.n.o"},
	}
	// No common leading segments
	chopPrefix(data)
	assert.Equal(t, "a.b.c", data[0]["resource"])
	assert.Equal(t, "x.y.z", data[1]["resource"])
	assert.Equal(t, "m.n.o", data[2]["resource"])
}

func TestChopPrefix_MultipleStringFields(t *testing.T) {
	data := []map[string]any{
		{"resource": "aws.s3.bucket1.x", "type": "aws.s3.prod"},
		{"resource": "aws.s3.bucket2.x", "type": "aws.s3.dev"},
		{"resource": "aws.s3.bucket3.x", "type": "aws.s3.staging"},
	}
	// "resource" field can be chopped (4 segments, removing 2 leaves 2)
	// "type" field can't be chopped (3 segments, removing 2 would leave 1)
	chopPrefix(data)
	assert.Equal(t, "..bucket1.x", data[0]["resource"])
	assert.Equal(t, "aws.s3.prod", data[0]["type"]) // unchanged
	assert.Equal(t, "..bucket2.x", data[1]["resource"])
	assert.Equal(t, "aws.s3.dev", data[1]["type"]) // unchanged
	assert.Equal(t, "..bucket3.x", data[2]["resource"])
	assert.Equal(t, "aws.s3.staging", data[2]["type"]) // unchanged
}

func TestChopPrefix_MixedStringAndNonString(t *testing.T) {
	data := []map[string]any{
		{"resource": "aws.s3.bucket1.prod", "id": 123},
		{"resource": "aws.s3.bucket2.dev", "id": 456},
	}
	// Non-string values are ignored during processing
	chopPrefix(data)
	assert.Equal(t, "..bucket1.prod", data[0]["resource"])
	assert.Equal(t, 123, data[0]["id"]) // unchanged
	assert.Equal(t, "..bucket2.dev", data[1]["resource"])
	assert.Equal(t, 456, data[1]["id"]) // unchanged
}

func TestChopPrefix_ExactMatchNoRemainder(t *testing.T) {
	data := []map[string]any{
		{"resource": "aws.s3"},
		{"resource": "aws.s3"},
	}
	// Common segments are "aws.s3" (2 segments) with no remainder
	// The prefix "aws.s3." won't match because neither value has a dot after "s3"
	chopPrefix(data)
	assert.Equal(t, "aws.s3", data[0]["resource"]) // unchanged
	assert.Equal(t, "aws.s3", data[1]["resource"]) // unchanged
}

func TestChopPrefix_DifferentLengths_PartialMatch(t *testing.T) {
	data := []map[string]any{
		{"resource": "aws.s3.x.y"},
		{"resource": "aws.s3.prod.server1"},
		{"resource": "aws.s3.dev.server2"},
	}
	// Common segments are "aws.s3", all entries have at least 4 segments
	// so chopping "aws.s3" leaves at least 2 segments
	chopPrefix(data)
	assert.Equal(t, "..x.y", data[0]["resource"])
	assert.Equal(t, "..prod.server1", data[1]["resource"])
	assert.Equal(t, "..dev.server2", data[2]["resource"])
}

func TestParseSqRootArgs(t *testing.T) {
	tests := []struct {
		name string
		args []string
		want []string
	}{
		{
			name: "no_args",
			args: []string{"tfquery", "sq"},
			want: nil,
		},
		{
			name: "flag_immediately_after_sq",
			args: []string{"tfquery", "sq", "--attrs", "arn"},
			want: []string{},
		},
		{
			name: "single_root_then_flag",
			args: []string{"tfquery", "sq", "dir1", "--attrs", "arn"},
			want: []string{"dir1"},
		},
		{
			name: "multiple_roots_then_flag",
			args: []string{"tfquery", "sq", "dir1", "dir2", "~/dir3", "--attrs", "arn"},
			want: []string{"dir1", "dir2", "~/dir3"},
		},
		{
			name: "stops_at_double_dash",
			args: []string{"tfquery", "sq", "dir1", "--", "--attrs", "arn"},
			want: []string{"dir1"},
		},
		{
			name: "alias_state_still_parses",
			args: []string{"tfquery", "state", "dir1", "--attrs", "arn"},
			want: []string{"dir1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, parseSqRootArgs(tt.args))
		})
	}
}

func TestBuildAggregatedRawRow(t *testing.T) {
	t.Run("valid_json_document", func(t *testing.T) {
		doc := []byte(`{"version":4,"resources":[{"type":"aws_s3_bucket"}]}`)

		row, err := buildAggregatedRawRow(doc, "/tmp/root-a")
		assert.NoError(t, err)
		assert.Equal(t, "/tmp/root-a", row["iacroot"])

		stateDoc, ok := row["state"].(map[string]any)
		assert.True(t, ok)
		assert.Equal(t, float64(4), stateDoc["version"])

		resources, ok := stateDoc["resources"].([]any)
		assert.True(t, ok)
		assert.Equal(t, 1, len(resources))

		_, marshalErr := json.Marshal(row)
		assert.NoError(t, marshalErr)
	})

	t.Run("invalid_json_document", func(t *testing.T) {
		doc := []byte(`{"version":4,`)

		row, err := buildAggregatedRawRow(doc, "/tmp/root-a")
		assert.Error(t, err)
		assert.Nil(t, row)
	})
}

func TestOutpuSingleRootRaw(t *testing.T) {
	t.Run("writes_raw_document_when_enabled", func(t *testing.T) {
		cmd := &cli.Command{
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "output", Value: "raw"},
			},
			Metadata: make(map[string]any),
		}

		doc := []byte(`{"version":4,"serial":3}`)
		var out bytes.Buffer

		handled := outputSingleRootRaw(
			true,
			doc,
			attrs.AttrList{},
			cmd,
			nil,
			&out,
		)

		assert.True(t, handled)
		assert.Equal(t, string(doc), out.String())
	})

	t.Run("no_write_when_disabled", func(t *testing.T) {
		cmd := &cli.Command{
			Flags: []cli.Flag{
				&cli.StringFlag{Name: "output", Value: "raw"},
			},
			Metadata: make(map[string]any),
		}

		doc := []byte(`{"version":4,"serial":3}`)
		var out bytes.Buffer

		handled := outputSingleRootRaw(
			false,
			doc,
			attrs.AttrList{},
			cmd,
			nil,
			&out,
		)

		assert.False(t, handled)
		assert.Equal(t, "", out.String())
	})
}
