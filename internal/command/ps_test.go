// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0
// no-cloc

package command

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/urfave/cli/v3"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// testPlanOutput is the shared example plan output used across all ps tests.
var testPlanOutput = `
An execution plan has been generated and is shown below.
Resource actions are indicated with the following symbols:
  + create
  ~ update in-place
 <= read (data resources)

Terraform will perform the following actions:

  # module.myapp.aws_s3_bucket.bucket will be created
  + resource "aws_s3_bucket" "bucket" {
      + bucket = "my-bucket"
    }

  # aws_instance.web will be updated in-place
  ~ resource "aws_instance" "web" {
      ~ instance_type = "t2.micro" -> "t3.micro"
    }

data.aws_caller_identity.validator: Reading...
module.data.aws_caller_identity.validator: Reading...
module.foo.data.bar: Reading...

Plan: 1 to add, 1 to change, 0 to destroy.
`

func TestParsePlanOutput(t *testing.T) {
	reader := strings.NewReader(testPlanOutput)
	resources, err := parsePlanOutput(reader, false)
	assert.NoError(t, err)

	expected := []PlanResource{
		{Resource: "module.myapp.aws_s3_bucket.bucket", Action: "created"},
		{Resource: "aws_instance.web", Action: "updated in-place"},
		{Resource: "data.aws_caller_identity.validator", Action: "read"},
		{Resource: "module.data.aws_caller_identity.validator", Action: "read"},
		{Resource: "module.foo.data.bar", Action: "read"},
	}

	assert.Equal(t, expected, resources)
}

func TestParsePlanOutputConcrete(t *testing.T) {
	reader := strings.NewReader(testPlanOutput)
	resources, err := parsePlanOutput(reader, true)
	assert.NoError(t, err)

	// With concrete=true, data source reads are excluded.
	expected := []PlanResource{
		{Resource: "module.myapp.aws_s3_bucket.bucket", Action: "created"},
		{Resource: "aws_instance.web", Action: "updated in-place"},
	}

	assert.Equal(t, expected, resources)
}

func TestPsBuildAttrs(t *testing.T) {
	cmd := &cli.Command{
		Flags: []cli.Flag{&cli.StringFlag{Name: "attrs", Value: ""}},
	}

	got := psBuildAttrs(cmd)
	require.Len(t, got, 2)
	assert.Equal(t, "resource", got[0].OutputKey)
	assert.Equal(t, "action", got[1].OutputKey)
}

func TestPsHeader(t *testing.T) {
	cmd := &cli.Command{
		Flags: []cli.Flag{&cli.StringFlag{Name: "filter", Value: "name@prod"}},
	}
	assert.Equal(t, "\nPlan action summary (filtered):", psHeader(cmd))

	cmd = &cli.Command{
		Flags: []cli.Flag{&cli.StringFlag{Name: "jq", Value: `.action == "created"`}},
	}
	assert.Equal(t, "\nPlan action summary (filtered):", psHeader(cmd))

	cmd = &cli.Command{}
	assert.Equal(t, "\nPlan action summary:", psHeader(cmd))

	cmd = &cli.Command{Flags: []cli.Flag{&cli.StringFlag{Name: "output", Value: "json"}}}
	assert.Equal(t, "", psHeader(cmd))
}

func TestPsPlanInput(t *testing.T) {
	assert.Equal(t, "-", psPlanInput([]string{}))
	assert.Equal(t, "-", psPlanInput([]string{"-"}))
	assert.Equal(t, "plan.txt", psPlanInput([]string{"plan.txt"}))
}

func TestPsOpenInput(t *testing.T) {
	t.Run("stdin", func(t *testing.T) {
		r, closeFn, err := psOpenInput("-")
		require.NoError(t, err)
		require.NotNil(t, r)
		require.NotNil(t, closeFn)
		assert.NoError(t, closeFn())
	})

	t.Run("missing file", func(t *testing.T) {
		_, _, err := psOpenInput("/this/path/does/not/exist.plan")
		require.Error(t, err)
		assert.Contains(t, err.Error(), "plan file does not exist")
	})

	t.Run("directory", func(t *testing.T) {
		_, _, err := psOpenInput(t.TempDir())
		require.Error(t, err)
		assert.Contains(t, err.Error(), "plan input cannot be a directory")
	})

	t.Run("file", func(t *testing.T) {
		path := filepath.Join(t.TempDir(), "plan.txt")
		require.NoError(t, os.WriteFile(path, []byte("plan"), 0o600))

		r, closeFn, err := psOpenInput(path)
		require.NoError(t, err)
		require.NotNil(t, r)
		require.NotNil(t, closeFn)
		assert.NoError(t, closeFn())
	})
}

func TestPsPrepareRender(t *testing.T) {
	cmd := &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "output", Value: "json"},
			&cli.BoolFlag{Name: "titles", Value: true},
		},
	}

	psPrepareRender(cmd)
	assert.False(t, cmd.Bool("titles"))

	cmd = &cli.Command{
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "output", Value: "text"},
			&cli.BoolFlag{Name: "titles", Value: true},
		},
	}

	psPrepareRender(cmd)
	assert.True(t, cmd.Bool("titles"))
}
