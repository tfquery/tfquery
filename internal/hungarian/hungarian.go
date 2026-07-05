// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package hungarian

import (
	"regexp"
	"slices"
	"strings"
)

// IsHungarian returns true if any component of the Terraform type (split by
// '_') appears in the name of that resource. Matching is case-insensitive and
// checks both substring containment and token equality when the name is split
// by non-alphanumeric chars and camelCase boundaries.
func IsHungarian(typ string, name string) bool {
	if typ == "" || name == "" {
		return false
	}

	typeLower := strings.ToLower(typ)
	nameLower := strings.ToLower(name)

	// Split the type into a slice of tokens.
	typeTokens := strings.Split(typeLower, "_")

	// Split the name by:
	// 1. Non-alphanumeric separators (dashes, dots, underscores, etc.)
	// 2. CamelCase boundaries (transition from lowercase to uppercase)
	// First replace camelCase boundaries with a delimiter, then split by non-
	// alphanumeric.
	camelCaseRe := regexp.MustCompile(`([a-z])([A-Z])`)
	nameWithDelim := camelCaseRe.ReplaceAllString(name, "${1}_${2}")

	splitRe := regexp.MustCompile(`[^a-z0-9]+`)
	nameParts := splitRe.Split(strings.ToLower(nameWithDelim), -1)

	// Iterate over each type token and see if it matches any name token.  If so,
	// it's Hungarian.
	for _, tok := range typeTokens {
		if tok == "" {
			continue
		}

		// If the token appears as a whole name part, it's Hungarian, so bail out.
		if slices.Contains(nameParts, tok) {
			return true
		}

		// Also treat any substring occurrence as a match covers cases like
		// resource "aws_s3_bucket" "mybucket", where the name is jammed without
		// separators.
		if strings.Contains(nameLower, tok) {
			// Hungarian - bail out.
			return true
		}
	}

	// Not Hungarian.
	return false
}
