// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0
// no-cloc

package command

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/urfave/cli/v3"

	"github.com/tfquery/tfquery/internal/config"
)

func TestBuildAttrs_UsesCommandPresetDefaults(t *testing.T) {
	originalConfig := config.Config
	t.Cleanup(func() {
		config.Config = originalConfig
	})

	config.Config = config.Type{
		Data: map[string]any{
			"mq": map[string]any{
				"attrs": []any{".cfg"},
			},
		},
		Namespace: "mq",
	}

	cmd := &cli.Command{
		Name: "mq",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "attrs"},
		},
	}

	attrs := BuildAttrs(cmd, true, ".default")

	assert.Equal(t, "cfg:cfg:", attrs.String())
}

func TestBuildAttrs_SkipsCommandPresetDefaults(t *testing.T) {
	originalConfig := config.Config
	t.Cleanup(func() {
		config.Config = originalConfig
	})

	config.Config = config.Type{
		Data: map[string]any{
			"mq": map[string]any{
				"attrs": []any{".cfg"},
			},
		},
		Namespace: "mq",
	}

	cmd := &cli.Command{
		Name: "mq",
		Flags: []cli.Flag{
			&cli.StringFlag{Name: "attrs"},
		},
	}

	attrs := BuildAttrs(cmd, false, ".default")

	assert.Equal(t, "default:default:", attrs.String())
}
