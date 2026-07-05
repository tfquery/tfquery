// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package command

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/apex/log"
	"github.com/urfave/cli/v3"

	"github.com/tfquery/tfquery/internal/attrs"
	"github.com/tfquery/tfquery/internal/backend"
	"github.com/tfquery/tfquery/internal/config"
	"github.com/tfquery/tfquery/internal/differ"
	"github.com/tfquery/tfquery/internal/meta"
	"github.com/tfquery/tfquery/internal/output"
	"github.com/tfquery/tfquery/internal/state"
	"github.com/tfquery/tfquery/internal/util"
)

// sqCommandAction is the action handler for the "sq" subcommand. It reads
// Terraform state (including optional decryption),and emits results per flags.
func sqCommandAction(ctx context.Context, cmd *cli.Command) error {
	config.Config.Namespace = "sq"

	m := GetMeta(cmd)
	log.Debugf("Executing action for %v", m.Args[1:])

	// Bail out early if we're just dumping tldr.
	if ShortCircuitTLDR(ctx, cmd, "sq") {
		return nil
	}

	// Collect all the iacroot dirs passed as positional args (eg. after "sq" but
	// before any flags). If none were passed, inject CWD.
	roots := parseSqRootArgs(m.Args)
	if len(roots) == 0 {
		roots = []string{m.RootDir}
	}

	// We'll ned to know if we're dealing with multiple roots at various spots.
	multiRoot := len(roots) > 1

	// --diff doesn't make sense with multiple roots because diffing two
	// different states seems useless compared to diffing two versions of the
	// same state.
	if multiRoot && cmd.Bool("diff") {
		return fmt.Errorf("--diff is not supported with multiple RootDir args")
	}

	// Setup helper to be run after dataset is in its final form and simply needs
	// final cosmetic transformations.
	postProcess := func(dataset []map[string]any) ([]map[string]any, error) {
		if cmd.Bool("count") {
			dataset = countResources(dataset)
		}

		if cmd.Bool("chop") {
			chopPrefix(dataset)
		}
		return dataset, nil
	}

	// Make sure to add the concrete filter on --concrete.
	if cmd.Bool("concrete") {
		filter := cmd.String("filter")
		if filter != "" {
			filter += ","
		}
		filter += "mode=managed"
		_ = cmd.Set("filter", filter)
	}

	// Add iacroot attrs if we're in multi-mode.
	relativeIacroot := false
	defaultAttrs := []string{"!.mode", "!.type", ".resource", "id", "name"}
	if multiRoot {
		defaultAttrs = append(defaultAttrs, `.iacroot`)
		// Should the iacroot attribute be relative to CWD?
		relativeIacroot, _ = config.GetBool("relative_iacroot", false)
	}

	// If this is --count mode, we need to supply our own attrs and ignore any
	// user-supplied values. The net effect will be that --count will cause
	// --attrs to be silently ignored.
	// THINK Emit an debug log for this condition?
	isCount := cmd.Bool("count")
	if isCount {
		defaultAttrs = []string{".mode", ".type", ".count"}
	}

	attrs := BuildAttrs(cmd, !isCount, defaultAttrs...)
	log.Debugf("built: defaultAttrs: %v; attrs: %v", defaultAttrs, attrs)
	if multiRoot && !isCount {
		includeIacrootAttribute(&attrs)
	}

	log.Debugf("defaultAttrs: %v; attrs: %v", defaultAttrs, attrs)

	// Save the original metadata so that we can transform it with root specific
	// values as we iterate over the roots. This way, we keep the common or
	// default metadata for each root.
	originalMeta := GetMeta(cmd)
	defer func() {
		cmd.Metadata["meta"] = originalMeta
	}()

	// Preserve previous single-root --diff behavior while keeping the common
	// root iteration path for non-diff output aggregation.
	if cmd.Bool("diff") {
		be, err := backend.NewBackend(ctx, *cmd)
		if err != nil {
			return err
		}

		if selfDiffer, ok := be.(backend.SelfDiffer); ok {
			states, diffErr := selfDiffer.DiffStates(ctx, cmd)
			if diffErr != nil {
				log.Errorf("diff error: %v", diffErr)
				return diffErr
			}

			return differ.Diff(ctx, cmd, states)
		}

		log.Debug("Backend does not implement SelfDiffer")
	}

	var combined []map[string]any
	rawOutput := cmd.String("output") == "raw"
	aggregateRawOutput := multiRoot && rawOutput
	singleRawOutput := !multiRoot && rawOutput
	resolvedPassphrase := ""

	// Iterate over all the roots, combining each state into the common dataset.
	// Each root's state is processed independently up through the point of
	// flattening, at which point we have uniform representations that are then
	// aggregated.
	for _, root := range roots {
		wd, env, err := util.ParseRootDir(root)
		if err != nil {
			return fmt.Errorf("failed to parse rootDir (%s): %w", root, err)
		}

		m2 := originalMeta
		m2.RootDir = wd
		m2.Env = env
		cmd.Metadata["meta"] = m2

		be, err := backend.NewBackend(ctx, *cmd)
		if err != nil {
			return err
		}
		log.Debugf("typBe: %v", be)

		doc, err := be.State()
		if err != nil {
			return err
		}

		doc, resolvedPassphrase, err = decryptStateIfNeeded(cmd, doc, resolvedPassphrase)
		if err != nil {
			return err
		}

		// We're problably most commonly in single-root mode when using raw, so
		// check for that and bail out early if so.
		if outputSingleRootRaw(singleRawOutput, doc, attrs, cmd, postProcess, os.Stdout) {
			return nil
		}

		if aggregateRawOutput {
			iacroot := transformIacroot(wd, m.StartingDir, relativeIacroot)

			rawRow, err := buildAggregatedRawRow(doc, iacroot)
			if err != nil {
				return err
			}

			combined = append(combined, rawRow)
			continue
		}

		rows, err := output.FlattenTerraformState(doc, !cmd.Bool("short"))
		if err != nil {
			return err
		}

		if multiRoot {
			iacroot := transformIacroot(wd, m.StartingDir, relativeIacroot)

			for i := range rows {
				rows[i]["iacroot"] = iacroot

				if rowAttrs, ok := rows[i]["attributes"].(map[string]any); ok {
					rowAttrs["iacroot"] = iacroot
				} else if rowAttrs, ok := rows[i]["attributes"].(map[string]any); ok {
					rowAttrs["iacroot"] = iacroot
				} else if rows[i]["attributes"] == nil {
					rows[i]["attributes"] = map[string]any{"iacroot": iacroot}
				}
			}
		}

		combined = append(combined, rows...)
	}

	jsonBytes, err := json.Marshal(combined)
	if err != nil {
		return err
	}

	var raw bytes.Buffer
	raw.Write(jsonBytes)
	output.SliceDiceSpit(raw, attrs, cmd, "", os.Stdout, postProcess)

	return nil
}

// sqCommandBuilder constructs the cli.Command for "sq", wiring metadata,
// flags, and action/validator handlers.
func sqCommandBuilder(meta meta.Meta) *cli.Command {
	return &cli.Command{
		Name:      "sq",
		Aliases:   []string{"state"},
		Usage:     "state query",
		UsageText: "tfquery sq [RootDir] [options]",
		Metadata: map[string]any{
			"meta": meta,
		},
		Flags: append([]cli.Flag{
			&cli.BoolFlag{
				Name:    "chop",
				Aliases: []string{"no-chop"},
				Usage:   "chop common resource prefix from names",
				Value:   false,
			},
			&cli.BoolFlag{
				Name:    "concrete",
				Aliases: []string{"k"},
				Usage:   "only include concrete resources",
				Value:   false,
			},
			&cli.BoolFlag{
				Name:  "count",
				Usage: "summarize state by resource type",
				Value: false,
			},
			&cli.BoolFlag{
				Name:  "diff",
				Usage: "find difference between state versions",
				Value: false,
			},
			&cli.StringFlag{
				Name:   "diff_filter",
				Hidden: true,
				Value:  "check_results",
			},
			&cli.IntFlag{
				Name:   "limit",
				Hidden: true,
				Usage:  "limit state versions returned",
				Value:  99999,
			},
			&cli.BoolFlag{
				Name:    "short",
				Aliases: []string{"no-short"},
				Usage:   "include full resource name paths",
				Value:   false,
			},
			&cli.StringFlag{
				Name:  "passphrase",
				Usage: "encrypted state passphrase",
			},
			&cli.StringFlag{
				Name:        "sv",
				Usage:       "state version to query",
				Value:       "0",
				HideDefault: true,
			},
			// We don't want sq to get default host and org values from the config.
			// Instead, we'll depend on the backend or, in exceptional cases, explicit
			// --host and --org flags.
			NewHostFlag("sq"),
			NewOrgFlag("sq"),
			tldrFlag,
			workspaceFlag,
		}, NewGlobalFlags("sq")...,
		),
		Before: func(ctx context.Context, cmd *cli.Command) (context.Context, error) {
			ResolveInverseFlags(cmd, meta.Args, []string{
				"chop", "color", "local", "titles", "short",
			})

			return ctx, GlobalFlagsValidator(ctx, cmd)
		},
		Action: sqCommandAction,
	}
}

// chopPrefix scans all dot-delimited string values in the dataset and removes
// leading segments that are identical across all entries. Starting from
// the left, it removes each segment that matches in all entries, then
// stops when it encounters a position where segments differ. Removed
// segments are replaced with "..".
func chopPrefix(dataset []map[string]any) {
	if len(dataset) == 0 {
		return
	}

	// Collect all string values by key across all entries
	// Map from key -> list of (entryIdx, segments)
	type segmentedValue struct {
		entryIdx int
		segments []string
	}

	keyValues := make(map[string][]segmentedValue)

	for entryIdx, entry := range dataset {
		for key, val := range entry {
			if str, ok := val.(string); ok {
				segments := strings.Split(str, ".")
				keyValues[key] = append(keyValues[key], segmentedValue{entryIdx: entryIdx, segments: segments})
			}
		}
	}

	// For each key, find and apply the common prefix
	for key, values := range keyValues {
		if len(values) == 0 {
			continue
		}

		// Find common leading segments for this key
		var commonCount int
		for segIdx := 0; ; segIdx++ {
			// Check if all entries have a segment at this position
			if segIdx >= len(values[0].segments) {
				break
			}

			// Get the segment value from the first entry
			expectedSeg := values[0].segments[segIdx]

			// Check if all entries have the same segment at this position
			allMatch := true
			for _, val := range values {
				if segIdx >= len(val.segments) || val.segments[segIdx] != expectedSeg {
					allMatch = false
					break
				}
			}

			if !allMatch {
				break
			}

			commonCount++
		}

		// Need at least 2 common segments to be worth chopping.
		if commonCount < 2 {
			continue
		}

		// Never chop past the second-to-last segment. Ensure at least 2
		// segments remain in all values after chopping.
		minSegments := len(values[0].segments)
		for _, val := range values {
			if len(val.segments) < minSegments {
				minSegments = len(val.segments)
			}
		}
		maxChop := minSegments - 2
		if maxChop < 2 {
			continue
		}
		if commonCount > maxChop {
			commonCount = maxChop
		}

		// Build the prefix to remove
		prefixSegs := values[0].segments[:commonCount]
		prefixToRemove := strings.Join(prefixSegs, ".") + "."

		// Remove the common prefix from all entries that have this key
		for _, val := range values {
			originalValue := strings.Join(val.segments, ".")
			if strings.HasPrefix(originalValue, prefixToRemove) {
				dataset[val.entryIdx][key] = ".." + originalValue[len(prefixToRemove):]
			}
		}
	}
}

type countKey struct {
	Mode string
	Type string
}

// countResources takes the resource records extracted from state file(s) and
// counts them by mode and type, returning a new dataset.
func countResources(dataset []map[string]any) []map[string]any {
	counts := make(map[countKey]int)

	// Count occurrences
	for _, entry := range dataset {
		mode, ok1 := entry["mode"].(string)
		resourceType, ok2 := entry["type"].(string)
		if !ok1 || !ok2 {
			continue
		}

		key := countKey{
			Mode: mode,
			Type: resourceType,
		}
		counts[key]++
	}

	// Convert to output format
	result := make([]map[string]any, 0, len(counts))
	for key, count := range counts {
		result = append(result, map[string]any{
			"mode":  key.Mode,
			"type":  key.Type,
			"count": count,
		})
	}

	return result
}

func decryptStateIfNeeded(cmd *cli.Command, doc []byte, passphrase string) ([]byte, string, error) {
	var jsonData map[string]any
	if err := json.Unmarshal(doc, &jsonData); err != nil {
		return doc, passphrase, nil
	}
	if _, exists := jsonData["encrypted_data"]; !exists {
		return doc, passphrase, nil
	}

	if passphrase == "" {
		// First, look to the flag for passphrase value.
		passphrase = cmd.String("passphrase")

		// Issue 14 - Next look in env and use it if found.
		if passphrase == "" {
			passphrase = os.Getenv("TFQUERY_PASSPHRASE")
		}

		// Finally, prompt for passphrase.
		if passphrase == "" {
			prompted, _ := state.GetPassphrase()
			passphrase = prompted
		}
	}

	decrypted, err := state.DecryptOpenTofuState(doc, passphrase)
	if err != nil {
		return nil, passphrase, fmt.Errorf("failed to decrypt: %w", err)
	}

	return decrypted, passphrase, nil
}

func includeIacrootAttribute(all *attrs.AttrList) {
	if all == nil {
		return
	}

	for i := range *all {
		if (*all)[i].OutputKey == "iacroot" {
			(*all)[i].Include = true
			return
		}
	}

	// If iacroot isn't already present, add a root-level accessor.
	//nolint:errcheck,gosec // AttrList.Set errors are logged internally
	all.Set(".iacroot")
}

func parseSqRootArgs(args []string) []string {
	if len(args) < 3 {
		return nil
	}

	roots := []string{}
	for i := 2; i < len(args); i++ {
		tok := args[i]
		if tok == "--" || strings.HasPrefix(tok, "-") {
			break
		}
		roots = append(roots, tok)
	}

	return roots
}

// transformIacroot transforms the iacroot value based on the relative flag.
func transformIacroot(iacroot string, baseDir string, relative bool) string {
	if !relative || iacroot == "" || baseDir == "" {
		return iacroot
	}

	rel, err := filepath.Rel(baseDir, iacroot)
	if err != nil {
		return iacroot
	}

	return rel
}

// buildAggregatedRawRow takes the raw JSON document and iacroot for each root
// and builds a single row for the aggregated raw output. We do this to ensure
// that the final aggregated output has identity built in and also a consistent
// structure with the non-raw output, which allows us to use the same output
// formatting logic for both.
func buildAggregatedRawRow(doc []byte, iacroot string) (map[string]any, error) {
	var stateDoc any
	if err := json.Unmarshal(doc, &stateDoc); err != nil {
		return nil, fmt.Errorf("failed to unmarshal aggregated raw state: %w", err)
	}

	return map[string]any{
		"iacroot": iacroot,
		"state":   stateDoc,
	}, nil
}

// outputSingleRootRaw handles the specific case of single-root raw output. If
// the conditions are met, it will output the verbatim state and return true.
func outputSingleRootRaw(
	singleRawOutput bool,
	doc []byte,
	attrs attrs.AttrList,
	cmd *cli.Command,
	postProcess func([]map[string]any) ([]map[string]any, error),
	w io.Writer,
) bool {
	if !singleRawOutput {
		return false
	}

	var raw bytes.Buffer
	raw.Write(doc)
	output.SliceDiceSpit(raw, attrs, cmd, "", w, postProcess)

	return true
}
