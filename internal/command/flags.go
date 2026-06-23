// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"os/exec"

	altsrc "github.com/urfave/cli-altsrc/v3"
	yaml "github.com/urfave/cli-altsrc/v3/yaml"
	"github.com/urfave/cli/v3"
)

var (
	schemaFlag *cli.BoolFlag = &cli.BoolFlag{
		Name:        "schema",
		Usage:       "dump the schema",
		HideDefault: true,
	}

	tldrFlag *cli.BoolFlag = &cli.BoolFlag{
		Name:        "tldr",
		Usage:       "show tldr page",
		Hidden:      !pathHas("tldr"),
		HideDefault: true,
	}

	workspaceFlag *cli.StringFlag = &cli.StringFlag{
		Name:    "workspace",
		Aliases: []string{"w"},
		Usage:   "workspace to use for query. Overrides the backend",
		Sources: cli.NewValueSourceChain(
			cli.EnvVar("TFCTL_WORKSPACE"),
		),
		Value: "",
	}
)

func NewGlobalFlags(params ...string) []cli.Flag {
	flags := []cli.Flag{
		&cli.StringFlag{
			Name:    "attrs",
			Aliases: []string{"a"},
			Usage:   "comma-separated list of attributes to include in results",
		},
		&cli.BoolFlag{
			Name:    "color",
			Aliases: []string{"no-color"},
			Usage:   "enable colored text output",
			Value:   false,
		},
		&cli.StringFlag{
			Name:    "filter",
			Aliases: []string{"f"},
			Usage:   "comma-separated list of filters to apply to results",
		},
		&cli.StringFlag{
			Name:  "jq",
			Usage: "jq query string to apply to results",
		},
		&cli.StringFlag{
			Name:  "json-into",
			Usage: "secondary output path to write JSON output to",
		},
		&cli.BoolFlag{
			Name:    "local",
			Aliases: []string{"no-local"},
			Usage:   "show local timestamps",
			Value:   false,
		},
		&cli.StringFlag{
			Name:    "output",
			Aliases: []string{"o"},
			Usage:   "output format",
			Value:   "text",
			Validator: func(value string) error {
				return FlagValidators(value, OutputValidator)
			},
		},
		&cli.IntFlag{
			Name:  "padding",
			Usage: "additional inter-column spacing in text output",
			Value: 5,
		},
		&cli.StringFlag{
			Name:    "sort",
			Aliases: []string{"s"},
			Usage:   "comma-separated list of attributes to sort the results by",
		},
		&cli.BoolFlag{
			Name:    "titles",
			Aliases: []string{"no-titles"},
			Usage:   "show titles with text output (--titles=0 to disable preset)",
			Value:   false,
		},
		&cli.StringFlag{
			Name:  "yaml-into",
			Usage: "secondary output path to write YAML output to",
		},
	}

	return flags
}

// NewHostFlag constructs a cli.StringFlag for the "host" flag, optionally
// namespaced to a command and config file. params[1] is the config file. Note
// that currently the sq command does not include params[1], thereby forcing the
// host to be derived from the backend or explicit flag.
func NewHostFlag(params ...string) *cli.StringFlag {
	hostFlag := &cli.StringFlag{
		Name:    "host",
		Aliases: []string{"h"},
		Usage:   "host to use for all commands. Overrides the backend",
		Sources: cli.NewValueSourceChain(
			cli.EnvVar("TFCTL_HOST"),
			cli.EnvVar("TF_CLOUD_HOSTNAME"),
		),
		Value: "app.terraform.io",
	}

	if len(params) < 2 {
		return hostFlag
	}

	hostFlag = NameSpacedValueChainFlagFromConfigFile(params[0], params[1], hostFlag)
	return hostFlag
}

// NewOrgFlag constructs a cli.StringFlag for the "org" flag, optionally
// namespaced to a command and config file. params[1] is the config file. Note
// that currently the sq command does not include params[1], thereby forcing the
// org to be derived from the backend or explicit flag.
func NewOrgFlag(params ...string) *cli.StringFlag {
	orgFlag := &cli.StringFlag{
		Name:  "org",
		Usage: "organization to use for all commands. Overrides the backend",
		Sources: cli.NewValueSourceChain(
			cli.EnvVar("TFCTL_ORG"),
			cli.EnvVar("TF_CLOUD_ORGANIZATION"),
		),
	}

	// params[0] is the tfquery config file. We only want to refer to it in non-
	// state commands, such as mq. For state commands, such as sq, we don't want
	// to infer a value and instead derive it from the .terraform/
	// terraform.tfstate.

	if len(params) < 2 {
		return orgFlag
	}

	orgFlag = NameSpacedValueChainFlagFromConfigFile(params[0], params[1], orgFlag)
	return orgFlag
}

// NameSpacedValueChainFlagFromConfigFile adds namespaced and global config file
// sources to the given flag's Sources chain.
func NameSpacedValueChainFlagFromConfigFile(ns string, path string, flag *cli.StringFlag) *cli.StringFlag {
	src := yaml.YAML(ns+"."+flag.Name, altsrc.StringSourcer(path))
	flag.Sources.Chain = append(flag.Sources.Chain, src)

	src = yaml.YAML(flag.Name, altsrc.StringSourcer(path))
	flag.Sources.Chain = append(flag.Sources.Chain, src)

	return flag
}

// ResolveInverseFlags scans args for --flag and --no-flag patterns and sets
// the flag value based on the last occurrence (last one wins). This enables
// presets to be overridden on the command line.
func ResolveInverseFlags(cmd *cli.Command, args []string, flags []string) {
	for _, f := range flags {
		for i := len(args) - 1; i >= 0; i-- {
			if args[i] == "--"+f {
				_ = cmd.Set(f, "true")
				break
			} else if args[i] == "--no-"+f {
				_ = cmd.Set(f, "false")
				break
			}
		}
	}
}

// pathHas checks if the given key exists in cfg.Source.
func pathHas(target string) bool {
	_, err := exec.LookPath(target)
	return err == nil
}
