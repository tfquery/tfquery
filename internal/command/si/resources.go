// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package si

import (
	"encoding/json"
	"fmt"
	"maps"
	"strconv"
	"strings"
)

// findMatchingResources finds resources in state data matching the query
func FindMatchingResources(stateData map[string]any, query *ParsedQuery) []map[string]any {
	resources, ok := stateData["resources"].([]any)
	if !ok {
		return nil
	}

	var matches []map[string]any

	for _, resource := range resources {
		res, ok := resource.(map[string]any)
		if !ok {
			continue
		}

		// Check mode
		mode := "managed"
		if resMode, ok := res["mode"].(string); ok {
			mode = resMode
		}
		if query.Mode != mode {
			continue
		}

		// Check module
		if !matchesModule(res, query.Module) {
			continue
		}

		// Check type (if specified)
		if query.Type != "" {
			if resType, ok := res["type"].(string); !ok || resType != query.Type {
				continue
			}
		}

		// Check name (if specified)
		if query.Name != "" {
			if resName, ok := res["name"].(string); !ok || resName != query.Name {
				continue
			}
		}

		// If we have instances, check index
		if instances, ok := res["instances"].([]any); ok {
			if query.Index != nil {
				// Find specific instance
				for _, instance := range instances {
					inst, ok := instance.(map[string]any)
					if !ok {
						continue
					}
					if matchesIndex(inst, query.Index) {
						matches = append(matches, createResourceMatch(res, inst))
					}
				}
			} else {
				// Return all instances
				for _, instance := range instances {
					inst, ok := instance.(map[string]any)
					if !ok {
						continue
					}
					matches = append(matches, createResourceMatch(res, inst))
				}
			}
		}
	}

	return matches
}

// matchesModule checks if a resource belongs to the specified module path
func matchesModule(resource map[string]any, moduleQuery []string) bool {
	if len(moduleQuery) == 0 {
		// No module specified - match resources not in modules
		return resource["module"] == nil
	}

	moduleStr, ok := resource["module"].(string)
	if !ok {
		return false
	}

	// Build expected module string
	expected := "module." + strings.Join(moduleQuery, ".")
	return moduleStr == expected
}

// matchesIndex checks if an instance matches the specified index
func matchesIndex(instance map[string]any, queryIndex any) bool {
	indexKey, ok := instance["index_key"]
	if !ok {
		// No index key means this is the only instance (index 0)
		return queryIndex == 0 || queryIndex == "0"
	}

	switch v := queryIndex.(type) {
	case int:
		if idx, ok := indexKey.(float64); ok {
			return int(idx) == v
		}
		if idx, ok := indexKey.(int); ok {
			return idx == v
		}
		// Also try string conversion
		if idx, ok := indexKey.(string); ok {
			return idx == strconv.Itoa(v)
		}
	case string:
		if idx, ok := indexKey.(string); ok {
			// Direct string comparison - this should handle "10.0.0.0/8" == "10.0.0.0/8"
			return idx == v
		}
		// Also try numeric conversion
		if idx, ok := indexKey.(float64); ok {
			return strconv.Itoa(int(idx)) == v
		}
		if idx, ok := indexKey.(int); ok {
			return strconv.Itoa(idx) == v
		}
	}

	return false
}

// createResourceMatch creates a flattened resource representation
func createResourceMatch(resource map[string]any, instance map[string]any) map[string]any {
	// Create a combined view of resource + instance
	result := make(map[string]any)

	// Copy resource fields
	for k, v := range resource {
		if k != "instances" {
			result[k] = v
		}
	}

	// Copy instance fields
	maps.Copy(result, instance)

	return result
}

// generateResourceAddresses creates Terraform addresses for matched resources
func generateResourceAddresses(matches []map[string]any) []string {
	var addresses []string

	for _, match := range matches {
		addr := buildResourceAddress(match)
		addresses = append(addresses, addr)
	}

	return addresses
}

// buildResourceAddress constructs a Terraform address from resource data
func buildResourceAddress(resource map[string]any) string {
	var parts []string

	// Add module prefix if present
	if module, ok := resource["module"].(string); ok && module != "" {
		parts = append(parts, module)
	}

	// Add mode prefix for data sources
	if mode, ok := resource["mode"].(string); ok && mode == "data" {
		parts = append(parts, "data")
	}

	// Add type
	if resourceType, ok := resource["type"].(string); ok {
		parts = append(parts, resourceType)
	}

	// Add name
	if name, ok := resource["name"].(string); ok {
		namePart := name

		// Add index if present
		if indexKey, ok := resource["index_key"]; ok {
			switch v := indexKey.(type) {
			case float64:
				namePart += fmt.Sprintf("[%d]", int(v))
			case int:
				namePart += fmt.Sprintf("[%d]", v)
			case string:
				namePart += fmt.Sprintf("[%q]", v)
			}
		}

		parts = append(parts, namePart)
	}

	return strings.Join(parts, ".")
}

// extractAttribute extracts the specified attribute from a resource, handling indices
func ExtractAttribute(resource map[string]any, parsed *ParsedQuery) any {
	// Check if this is a flattened resource match (has attributes directly)
	if attributes, ok := resource["attributes"].(map[string]any); ok {
		if attrValue, exists := attributes[parsed.Attribute]; exists {
			return attrValue
		}
		return nil
	}

	// Fall back to original instances array logic for unflattened resources
	instances, ok := resource["instances"].([]any)
	if !ok || len(instances) == 0 {
		return nil
	}

	// If no index specified, get attribute from all instances
	if parsed.Index == nil {
		var results []any
		for _, instance := range instances {
			if instanceMap, ok := instance.(map[string]any); ok {
				if attributes, ok := instanceMap["attributes"].(map[string]any); ok {
					if attrValue, exists := attributes[parsed.Attribute]; exists {
						results = append(results, attrValue)
					}
				}
			}
		}
		if len(results) == 1 {
			return results[0] // Single value, return directly
		}
		return results // Multiple values, return as array
	}

	// Find the specific instance by index
	for _, instance := range instances {
		if instanceMap, ok := instance.(map[string]any); ok {
			if indexKey, exists := instanceMap["index_key"]; exists {
				if indexMatchesValue(indexKey, parsed.Index) {
					if attributes, ok := instanceMap["attributes"].(map[string]any); ok {
						if attrValue, exists := attributes[parsed.Attribute]; exists {
							return attrValue
						}
					}
				}
			} else if parsed.Index == 0 || parsed.Index == "0" {
				// No index_key means this is the first (and possibly only) instance
				if attributes, ok := instanceMap["attributes"].(map[string]any); ok {
					if attrValue, exists := attributes[parsed.Attribute]; exists {
						return attrValue
					}
				}
			}
		}
	}

	return nil
}

// formatAttributeValue formats an attribute value for string output
func formatAttributeValue(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case nil:
		return "null"
	default:
		if jsonBytes, err := json.Marshal(v); err == nil {
			return string(jsonBytes)
		}
		return fmt.Sprintf("%v", v)
	}
}

// indexMatchesValue checks if an index key matches the query index
func indexMatchesValue(indexKey any, queryIndex any) bool {
	switch qv := queryIndex.(type) {
	case int:
		if idx, ok := indexKey.(float64); ok {
			return int(idx) == qv
		}
		if idx, ok := indexKey.(int); ok {
			return idx == qv
		}
		if idx, ok := indexKey.(string); ok {
			return idx == strconv.Itoa(qv)
		}
	case string:
		if idx, ok := indexKey.(string); ok {
			return idx == qv
		}
		if idx, ok := indexKey.(float64); ok {
			return strconv.Itoa(int(idx)) == qv
		}
	}
	return false
}

// printJSON outputs data as formatted JSON
func printJSON(data any) {
	jsonBytes, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Printf("Error formatting JSON: %s\n", err)
		return
	}
	fmt.Println(string(jsonBytes))
}
