// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tfquery/tfquery/internal/config"
	"github.com/tfquery/tfquery/internal/version"
)

// deduplicateFlags removes duplicate flags from args, keeping the last
// occurrence of each flag. Flags are identified by the "--" or "-" prefix.
// For flags with values (e.g., "--output text"), we track by the flag name
// and keep the last flag+value pair.
func deduplicateFlags(args []string) []string {
	if len(args) <= 2 {
		return args
	}

	// Preserve args[0] (program) and args[1] (command), then dedupe the rest.
	prefix := args[:2]
	rest := args[2:]

	// Track the last position where each flag appears.
	flagPositions := make(map[string]int)

	for i := range rest {
		arg := rest[i]

		if strings.HasPrefix(arg, "-") {
			flagName := arg

			// Handle --flag=value syntax.
			if before, _, ok := strings.Cut(arg, "="); ok {
				flagName = before
			}

			flagPositions[flagName] = i
		}
	}

	// Build result, skipping flags that appear again later.
	var result []string
	for i := 0; i < len(rest); i++ {
		arg := rest[i]

		if strings.HasPrefix(arg, "-") {
			flagName := arg
			if before, _, ok := strings.Cut(arg, "="); ok {
				flagName = before
			}

			// Skip this flag if it appears again later.
			if flagPositions[flagName] > i {
				// If this flag has a separate value arg, skip that too.
				if !strings.Contains(arg, "=") && i+1 < len(rest) &&
					!strings.HasPrefix(rest[i+1], "-") {
					i++
				}

				continue
			}
		}

		result = append(result, arg)
	}

	return append(prefix, result...)
}

// expandFlagSingleValuePreset expands @preset tokens for single-value flags
// (for example --jq) using values from <configRoot>.<preset> in the loaded
// config file.
func expandFlagSingleValuePreset(args []string, longFlag string,
	shortFlag string, configRoot string,
) []string {
	if len(args) <= 2 {
		return args
	}

	for i := 2; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == longFlag || (shortFlag != "" && arg == shortFlag):
			if i+1 < len(args) {
				args[i+1] = expandSinglePresetValue(args[i+1], configRoot)
				i++
			}
		case strings.HasPrefix(arg, longFlag+"="):
			_, value, _ := strings.Cut(arg, "=")
			args[i] = longFlag + "=" + expandSinglePresetValue(value, configRoot)
		case shortFlag != "" && strings.HasPrefix(arg, shortFlag+"="):
			_, value, _ := strings.Cut(arg, "=")
			args[i] = shortFlag + "=" + expandSinglePresetValue(value, configRoot)
		}
	}

	return args
}

// expandFlagValuePresets expands @preset tokens in the value for a flag
// using values from <configRoot>.<preset> in the loaded config file.
//
// args - full command line argument slice.
// longFlag/shortFlag - the flags being processed.
// configRoot - top-level config key used to find presets.
// delimiter - on what character segments are split and rejoined.
func expandFlagValuePresets(args []string, longFlag string, shortFlag string,
	configRoot string, delimiter string,
) []string {
	if len(args) <= 2 {
		return args
	}

	for i := 2; i < len(args); i++ {
		arg := args[i]

		switch {
		case arg == longFlag || arg == shortFlag:
			if i+1 < len(args) {
				args[i+1] = expandPresetSegments(args[i+1], configRoot, delimiter)
				i++
			}
		case strings.HasPrefix(arg, longFlag+"="):
			_, value, _ := strings.Cut(arg, "=")
			args[i] = longFlag + "=" + expandPresetSegments(
				value,
				configRoot,
				delimiter,
			)
		case strings.HasPrefix(arg, shortFlag+"="):
			_, value, _ := strings.Cut(arg, "=")
			args[i] = shortFlag + "=" + expandPresetSegments(
				value,
				configRoot,
				delimiter,
			)
		}
	}

	return args
}

// expandPresetSegments replaces comma-delimited @preset segments using
// <configRoot>.<preset> values from the load config file.
//
// value - is the raw flag value to process.
// configRoot - top-level config key used to find presets.
// delimiter - on what character segments are split and rejoined.
func expandPresetSegments(value string, configRoot string,
	delimiter string,
) string {
	if value == "" || !strings.Contains(value, "@") {
		return value
	}

	if delimiter == "" {
		delimiter = ","
	}

	segments := strings.Split(value, delimiter)
	var expanded []string

	for _, segment := range segments {
		part := strings.TrimSpace(segment)
		if !strings.HasPrefix(part, "@") {
			expanded = append(expanded, part)
			continue
		}

		presetName := strings.TrimPrefix(part, "@")
		if presetName == "" {
			expanded = append(expanded, part)
			continue
		}

		key := configRoot + "." + presetName

		if presetValue, err := config.GetString(key); err == nil {
			expanded = append(expanded, strings.TrimSpace(presetValue))
			continue
		}

		if presetSlice, err := config.GetStringSlice(key); err == nil &&
			len(presetSlice) > 0 {
			expanded = append(expanded, strings.Join(presetSlice, delimiter))
			continue
		}

		// Unknown preset: preserve original token.
		expanded = append(expanded, part)
	}

	return strings.Join(expanded, delimiter)
}

// expandSinglePresetValue replaces a single @preset token using
// <configRoot>.<preset> from the loaded config file.
func expandSinglePresetValue(value string, configRoot string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" || !strings.HasPrefix(trimmed, "@") {
		return value
	}

	presetName := strings.TrimPrefix(trimmed, "@")
	if presetName == "" {
		return value
	}

	key := configRoot + "." + presetName
	presetValue, err := config.GetString(key)
	if err != nil {
		return value
	}

	return strings.TrimSpace(presetValue)
}

// filterDelimiter returns the delimiter used for --filter values.
func filterDelimiter() string {
	delim := ","
	if d, ok := os.LookupEnv("TFQUERY_FILTER_DELIM"); ok && d != "" {
		delim = d
	}

	return delim
}

// handleNakedCommand appends --help if no command is provided.
func handleNakedCommand(args []string) []string {
	if len(args) <= 1 {
		return append(args, "--help")
	}
	return args
}

// handleVersion checks for --version/-v and returns whether it was
// handled.
func handleVersion(args []string) bool {
	for _, a := range args {
		if a == "--version" || a == "-v" {
			fmt.Println(version.Version)
			return true
		}
	}
	return false
}

// injectConfigSet retrieves the config slice for the given key, expands
// each entry by whitespace, and inserts the resulting args at the
// specified index.
func injectConfigSet(args []string, key string, insertIdx int) []string {
	entries, _ := config.GetStringSlice(key)
	if len(entries) == 0 {
		return args
	}

	var expanded []string
	for _, entry := range entries {
		expanded = append(expanded, strings.Fields(entry)...)
	}

	return append(args[:insertIdx], append(expanded, args[insertIdx:]...)...)
}

// injectExplicitSet handles the @set logic for all commands, expanding set
// arguments at the @set position.
func injectExplicitSet(args []string) []string {
	// Look for an explicit @set argument starting from front.
	idx := 2
	set := ""
	setIdx := len(args)

	for i, a := range args[idx:] {
		tokenIdx := idx + i

		// We need to skip --attrs and --filter flags since they also use `@`
		// to indicate a predefined value as opposed to an explicit set.
		if strings.HasPrefix(a, "@") &&
			!shouldSkipExplicitSetToken(args, tokenIdx) {
			set = strings.TrimPrefix(a, "@")
			setIdx = tokenIdx
			args = append(args[:setIdx], args[setIdx+1:]...)
			break
		}
	}

	if set != "" {
		setArgs, _ := config.GetStringSlice(args[1] + "." + set)
		for _, arg := range setArgs {
			parts := strings.Fields(arg)
			args = append(args[:setIdx], append(parts, args[setIdx:]...)...)
			setIdx += len(parts)
		}
	}

	return args
}

// isExistingFile checks if the given path exists and is a file.
func isExistingFile(path string) bool {
	candidatePath := filepath.Clean(path)
	if stat, err := os.Stat(candidatePath); err == nil && !stat.IsDir() {
		return true
	}
	return false
}

// processPsArgs handles argument processing for the ps command.
func processPsArgs(args []string) []string {
	if len(args) > 2 && strings.HasPrefix(args[2], "-") {
		return args
	}

	// Ensure the argument immediately following "ps" is "-" or an existing
	// file.
	if len(args) == 2 || (args[2] != "-" && !isExistingFile(args[2])) {
		args = append(args[:2], append([]string{"-"}, args[2:]...)...)
	}
	return args
}

// processCommandArgs handles command-specific argument processing.
func processCommandArgs(args []string) []string {
	switch {
	case len(args) > 1 && args[1] == "completion":
		// Short-circuit completion: pass args directly.
		return args
	default:
		insertIdx := 2
		// Config sets are injected immediately after any RootDir positionals
		// so that RootDir args remain contiguous and flag override precedence
		// remains intact.
		//
		// For most commands, RootDir is at args[2] (single positional). For
		// sq, there can be multiple RootDir args, so we insert after the
		// contiguous block of RootDir args (before the first flag/--).
		if len(args) > 2 {
			if len(args) > 1 && args[1] == "sq" {
				for i := 2; i < len(args); i++ {
					tok := args[i]
					if tok == "--" || strings.HasPrefix(tok, "-") {
						insertIdx = i
						break
					}
					insertIdx = i + 1
				}
			} else {
				candidatePath := filepath.Clean(args[2])
				if stat, err := os.Stat(candidatePath); err == nil &&
					stat.IsDir() {
					insertIdx = 3
				}
			}
		}

		// Inject sets in reverse precedence order (defaults → nostate →
		// global) so that higher-precedence sets appear later and win on
		// duplicates. After injection, we deduplicate by keeping the last
		// occurrence of each flag.
		args = injectConfigSet(args, args[1]+".defaults", insertIdx)

		if args[1] != "sq" && args[1] != "ps" {
			args = injectConfigSet(args, "nostate", insertIdx)
		}

		args = injectConfigSet(args, "defaults", insertIdx)
		if args[1] != "ps" {
			args = injectExplicitSet(args)
		}
		args = deduplicateFlags(args)
		args = expandFlagValuePresets(
			args,
			"--attrs",
			"-a",
			"presets.attrs",
			",",
		)
		args = expandFlagValuePresets(
			args,
			"--filter",
			"-f",
			"presets.filters",
			filterDelimiter(),
		)
		args = expandFlagSingleValuePreset(args, "--jq", "", "presets.jq")

		if len(args) > 1 && args[1] == "ps" {
			args = processPsArgs(args)
		}

		return args
	}
}

// shouldSkipExplicitSetToken returns true when @token is the value
// argument for attrs/filter flags and should not be treated as an
// explicit set.
func shouldSkipExplicitSetToken(args []string, tokenIdx int) bool {
	if tokenIdx <= 0 || tokenIdx >= len(args) {
		return false
	}

	switch args[tokenIdx-1] {
	case "--attrs", "-a", "--filter", "-f", "--jq":
		return true
	default:
		return false
	}
}
