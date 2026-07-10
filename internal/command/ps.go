// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/apex/log"
	"github.com/urfave/cli/v3"

	"github.com/tfquery/tfquery/internal/attrs"
	"github.com/tfquery/tfquery/internal/config"
	"github.com/tfquery/tfquery/internal/meta"
	"github.com/tfquery/tfquery/internal/output"
)

// ansiColorRegex matches ANSI escape sequences used for coloring terminal
// output. Matches patterns like ESC[1m, ESC[0m, ESC[31m, etc.
var ansiColorRegex = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// PlanResource represents a parsed resource action from the plan output.
type PlanResource struct {
	Resource string `json:"resource"`
	Action   string `json:"action"`
}

// psDefaultAttrs specifies the default attributes displayed for plan resources.
var psDefaultAttrs = []string{".resource", ".action"}

// psCommandAction is the action handler for the "ps" subcommand. It reads
// Terraform plan output from a file or stdin, extracts resource action lines,
// and renders them according to output flags.
func psCommandAction(ctx context.Context, cmd *cli.Command) error {
	config.Config.Namespace = "ps"

	meta := GetMeta(cmd)
	log.Debugf("Executing action for %v", meta.Args[1:])
	if ShortCircuitTLDR(ctx, cmd, "ps") {
		return nil
	}

	psPrepareRender(cmd)
	header := psHeader(cmd)
	if header != "" {
		cmd.Metadata["header"] = header
	} else {
		delete(cmd.Metadata, "header")
	}

	planInput := psPlanInput(cmd.Args().Slice())
	input, closeFn, err := psOpenInput(planInput)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := closeFn(); closeErr != nil {
			log.Errorf("failed to close plan file: %v", closeErr)
		}
	}()

	// Parse the plan output and get resource actions
	resources, err := parsePlanOutput(input, cmd.Bool("concrete"))
	if err != nil {
		return err
	}

	// Convert resources to the format expected by output framework
	// The output framework expects either a JSON array of objects or
	// a document with a specific structure.
	var jsonData []byte
	if jsonData, err = json.Marshal(resources); err != nil {
		return fmt.Errorf("failed to marshal dataset: %w", err)
	}

	// Use the output framework to display results
	var raw bytes.Buffer
	raw.Write(jsonData)

	output.SliceDiceSpit(raw, psBuildAttrs(cmd), cmd, "", os.Stdout, nil)

	return nil
}

func psBuildAttrs(cmd *cli.Command) attrs.AttrList {
	return BuildAttrs(cmd, true, psDefaultAttrs...)
}

func psPrepareRender(cmd *cli.Command) {
	if psIsTextOutput(cmd) {
		return
	}

	_ = cmd.Set("titles", "false")
}

func psHeader(cmd *cli.Command) string {
	if !psIsTextOutput(cmd) {
		return ""
	}

	header := "\nPlan action summary"
	if cmd.String("filter") != "" || cmd.String("jq") != "" {
		header += " (filtered)"
	}

	return header + ":"
}

func psIsTextOutput(cmd *cli.Command) bool {
	mode := strings.ToLower(strings.TrimSpace(cmd.String("output")))
	if mode == "" {
		mode = "text"
	}

	return mode == "text"
}

func psPlanInput(args []string) string {
	if len(args) > 0 && args[0] != "-" {
		return args[0]
	}

	return "-"
}

func psOpenInput(path string) (io.Reader, func() error, error) {
	if path == "-" {
		return os.Stdin, func() error { return nil }, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		return nil, nil, fmt.Errorf("plan file does not exist: %s", path)
	}

	if info.IsDir() {
		return nil, nil, fmt.Errorf("plan input cannot be a directory: %s", path)
	}

	input, err := os.Open(path)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to open plan file: %w", err)
	}

	return input, input.Close, nil
}

// parsePlanOutput reads the plan input and extracts resource action lines.
// Format: # <resource-path> will be <action>
// Example: # module.myapp[0].aws_s3_bucket.s3_loggingbucket will be created
func parsePlanOutput(input io.Reader, concrete bool) ([]PlanResource, error) {
	// Regex to match lines like:
	// # <resource-path> will be <action>
	// We capture: resource path and the action (everything between "will/must be" and end of line)
	resourceLineRegex := regexp.MustCompile(
		`^\s*#\s+(.+?)\s+(?:will|must)\s+be\s+(.+?)\s*$`,
	)

	// Regex to match data source reads like:
	// data.aws_caller_identity.validator: Reading...
	// module.data.aws_caller_identity.validator: Reading...
	dataReadLineRegex := regexp.MustCompile(
		`^\s*(.+?):\s+Reading\.\.\.\s*$`,
	)

	var resources []PlanResource

	scanner := bufio.NewScanner(input)
	for scanner.Scan() {
		line := scanner.Text()

		// Strip ANSI color codes from the line.
		line = ansiColorRegex.ReplaceAllString(line, "")

		// Try to match the resource action pattern
		if matches := resourceLineRegex.FindStringSubmatch(line); len(matches) == 3 {
			resources = append(resources, PlanResource{
				Resource: matches[1],
				Action:   matches[2],
			})
			continue
		}

		// Try to match data read pattern. We'll only do this when concrete is false
		// so that the summary is no polluted with data reads.
		if !concrete {
			if matches := dataReadLineRegex.FindStringSubmatch(line); len(matches) == 2 {
				resource := strings.TrimSpace(matches[1])
				if strings.HasPrefix(resource, "data.") || strings.Contains(resource, ".data.") {
					resources = append(resources, PlanResource{
						Resource: resource,
						Action:   "read",
					})
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading plan input: %w", err)
	}

	return resources, nil
}

// psCommandBuilder constructs the "ps" subcommand.
func psCommandBuilder(meta meta.Meta) *cli.Command {
	flags := NewGlobalFlags("ps")

	// Remove the --attrs flag since ps doesn't use it.
	var ps []cli.Flag
	for _, flag := range flags {
		if flag.Names()[0] != "attrs" {
			ps = append(ps, flag)
		}
	}

	return &cli.Command{
		Name:      "ps",
		Aliases:   []string{"summarize"},
		Usage:     "plan summary",
		UsageText: "tfquery ps [plan-file]",
		Metadata:  map[string]any{"meta": meta},
		Flags: append(ps, []cli.Flag{
			&cli.BoolFlag{
				Name:    "concrete",
				Aliases: []string{"k"},
				Usage:   "only include concrete resources",
				Value:   false,
			},
		}...),
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			ResolveInverseFlags(cmd, meta.Args, []string{"color", "local", "titles"})

			return ctx, GlobalFlagsValidator(ctx, cmd)
		},
		Action: psCommandAction,
	}
}
