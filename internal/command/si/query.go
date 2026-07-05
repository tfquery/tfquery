// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package si

import (
	"fmt"
	"strconv"
	"strings"
)

// ParsedQuery represents a parsed Terraform-style query
type ParsedQuery struct {
	Module    []string // Module path components, e.g., ["module", "sample"]
	Mode      string   // "managed", "data", empty for managed resources
	Type      string   // Resource type, e.g., "aws_instance"
	Name      string   // Resource name, e.g., "web"
	Index     any      // Instance index (int, string, or nil for all)
	Attribute string   // Attribute name, e.g., "arn", "id"
}

// processQuery routes queries to appropriate handlers based on syntax
func ProcessQuery(stateData map[string]any, query string) {
	// Check for function evaluation mode
	if after, ok := strings.CutPrefix(query, "/"); ok {
		// Force function mode with leading /
		expression := after
		result := evaluateFunction(expression, stateData)
		fmt.Println(result)
		return
	}

	// Check for balanced parentheses (likely function)
	if hasBalancedParens(query) {
		// Assume function mode
		result := evaluateFunction(query, stateData)
		fmt.Println(result)
		return
	}

	// Normal query mode
	jsonMode := strings.HasPrefix(query, ".")
	if jsonMode {
		query = strings.TrimPrefix(query, ".")
	}

	// Handle special queries
	if result := handleSpecialQueries(stateData, query); result != nil {
		if jsonMode {
			printJSON(result)
		} else {
			fmt.Println(result)
		}
		return
	}

	// Parse the query
	parsed, err := ParseQuery(query)
	if err != nil {
		fmt.Printf("Error: %s\n", err)
		return
	}

	// Find matching resources
	matches := FindMatchingResources(stateData, parsed)

	// Handle attribute extraction if specified
	if parsed.Attribute != "" {
		if jsonMode {
			// Output JSON for attribute values
			for _, match := range matches {
				attrValue := ExtractAttribute(match, parsed)
				if attrValue != nil {
					printJSON(attrValue)
				}
			}
		} else {
			// Output attribute values as strings
			for _, match := range matches {
				attrValue := ExtractAttribute(match, parsed)
				if attrValue != nil {
					fmt.Println(formatAttributeValue(attrValue))
				}
			}
		}
	} else {
		// Normal resource output (no attribute specified)
		if jsonMode {
			// Output JSON for all matches
			for _, match := range matches {
				printJSON(match)
			}
		} else {
			// Output list of resource addresses
			addresses := generateResourceAddresses(matches)
			for _, addr := range addresses {
				fmt.Println(addr)
			}
		}
	}
}

// hasBalancedParens checks if a string has balanced parentheses
func hasBalancedParens(s string) bool {
	openCount := 0
	closeCount := 0

	for _, char := range s {
		switch char {
		case '(':
			openCount++
		case ')':
			closeCount++
		}
	}

	// Must have at least one pair of parens and they must be balanced
	return openCount > 0 && openCount == closeCount
}

// handleSpecialQueries handles built-in special queries
func handleSpecialQueries(stateData map[string]any, query string) any {
	switch query {
	case "terraform_version":
		if val, ok := stateData["terraform_version"]; ok {
			return val
		}
		return "not found"
	case "version":
		if val, ok := stateData["version"]; ok {
			return val
		}
		return "not found"
	}

	// Handle outputs queries like "outputs.bucket_name"
	if after, ok := strings.CutPrefix(query, "outputs."); ok {
		outputName := after
		if outputs, ok := stateData["outputs"].(map[string]any); ok {
			if output, ok := outputs[outputName].(map[string]any); ok {
				return output["value"]
			}
		}
		return fmt.Sprintf("output '%s' not found", outputName)
	}

	return nil
}

// parseQuery parses a Terraform-style query string into structured components
func ParseQuery(query string) (*ParsedQuery, error) {
	parsed := &ParsedQuery{
		Mode: "managed", // Default to managed resources
	}

	// Split the query correctly, respecting quoted strings
	parts := smartSplit(query, ".")
	pos := 0

	// Parse module path: collect all "module.NAME" pairs
	for pos < len(parts) && parts[pos] == "module" {
		if pos+1 >= len(parts) {
			// "module" at end with no name - invalid
			return nil, fmt.Errorf("invalid module syntax: 'module' must be followed by module name")
		}
		pos++ // skip "module"
		moduleName := parts[pos]
		parsed.Module = append(parsed.Module, moduleName)
		pos++ // move to next part
	}

	// Check for data mode
	if pos < len(parts) && parts[pos] == "data" {
		parsed.Mode = "data"
		pos++
	}

	// Get resource type (optional)
	if pos < len(parts) {
		typeAndIndex := parts[pos]
		// Check for index notation
		if idx := strings.Index(typeAndIndex, "["); idx != -1 {
			parsed.Type = typeAndIndex[:idx]
			indexStr := typeAndIndex[idx+1 : len(typeAndIndex)-1]
			parsed.Index = parseIndex(indexStr)
		} else {
			parsed.Type = typeAndIndex
		}
		pos++
	}

	// Get resource name (optional)
	if pos < len(parts) {
		nameAndIndex := parts[pos]
		// Check for index notation
		if idx := strings.Index(nameAndIndex, "["); idx != -1 {
			parsed.Name = nameAndIndex[:idx]
			indexStr := nameAndIndex[idx+1 : len(nameAndIndex)-1]
			parsed.Index = parseIndex(indexStr)
		} else {
			parsed.Name = nameAndIndex
		}
		pos++
	}

	// Get attribute (optional)
	if pos < len(parts) {
		parsed.Attribute = parts[pos]
		pos++
	}

	// Ensure we've consumed all parts
	if pos < len(parts) {
		return nil, fmt.Errorf("unexpected extra parts in query: %v", parts[pos:])
	}

	return parsed, nil
}

// smartSplit splits a string by delimiter but respects quoted strings
func smartSplit(s, delimiter string) []string {
	var parts []string
	var current strings.Builder
	inQuotes := false
	i := 0

	for i < len(s) {
		switch {
		case s[i] == '"':
			inQuotes = !inQuotes
			current.WriteByte(s[i])
			i++
		case !inQuotes && i+len(delimiter) <= len(s) && s[i:i+len(delimiter)] == delimiter:
			parts = append(parts, current.String())
			current.Reset()
			i += len(delimiter)
		default:
			current.WriteByte(s[i])
			i++
		}
	}

	if current.Len() > 0 {
		parts = append(parts, current.String())
	}

	return parts
}

// parseIndex parses an index string into appropriate type
func parseIndex(indexStr string) any {
	// Try to parse as integer
	if i, err := strconv.Atoi(indexStr); err == nil {
		return i
	}

	// Try to parse as quoted string
	if strings.HasPrefix(indexStr, `"`) && strings.HasSuffix(indexStr, `"`) {
		return indexStr[1 : len(indexStr)-1]
	}

	// Return as string
	return indexStr
}
