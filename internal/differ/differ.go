// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package differ

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/apex/log"
	"github.com/urfave/cli/v3"
	"github.com/yudai/gojsondiff"
	"github.com/yudai/gojsondiff/formatter"

	"github.com/tfquery/tfquery/internal/meta"
)

// Diff compares two states.
func Diff(ctx context.Context, cmd *cli.Command, states [][]byte) error {
	log.Debugf(">> differ()")

	if len(states[0]) == 0 || len(states[1]) == 0 {
		return nil
	}

	log.Debugf("len(states): %d %d", len(states[0]), len(states[1]))

	differ := gojsondiff.New()

	delta, err := differ.Compare(states[0], states[1])
	if err != nil {
		return fmt.Errorf("failed to compare states: %w", err)
	}

	if delta.Modified() {
		var jdoc map[string]interface{}
		if err := json.Unmarshal(states[0], &jdoc); err != nil {
			return fmt.Errorf("failed to unmarshal state: %w", err)
		}

		filter := cmd.String("diff_filter")

		for key := range strings.SplitSeq(filter, ",") {
			if key != "" {
				delete(jdoc, key)
			}
		}

		config := formatter.AsciiFormatterConfig{
			ShowArrayIndex: false,
			Coloring:       true,
		}

		formatter := formatter.NewAsciiFormatter(jdoc, config)
		diffString, err := formatter.Format(delta)
		if err != nil {
			return err
		}

		fmt.Fprintln(os.Stdout, diffString)
		return nil
	}

	fmt.Fprintln(os.Stdout, "The states are identical.")
	return nil
}

func ParseDiffArgs(ctx context.Context, cmd *cli.Command) (args []string) {
	meta := cmd.Metadata["meta"].(meta.Meta)

	diffFound := false
	for _, a := range meta.Args {
		// Diff is found, so put us in collect mode.
		if a == "--diff" {
			diffFound = true
			continue
		}

		// We've collected the max diff args, bail out.
		if len(args) == 2 {
			return
		}

		if diffFound {
			// If the next arg up is a flag, bail out.  The definition of what is a
			// flag is a little indeterminate.

			_, itsAnInt := strconv.Atoi(a)
			if a == "+" ||
				strings.HasPrefix(strings.ToUpper(a), "CSV~") ||
				itsAnInt == nil ||
				!strings.HasPrefix(a, "-") {
				args = append(args, a)
			} else {
				return
			}
		}
	}

	return
}
