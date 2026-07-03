// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package filters

import (
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/apex/log"
	"github.com/tidwall/gjson"

	"github.com/tfquery/tfquery/internal/attrs"
	"github.com/tfquery/tfquery/internal/driller"
	"github.com/tfquery/tfquery/internal/hungarian"
)

// filterRegex is the pattern used to parse filter expressions into key,
// operator, and target components. It matches an optional leading underscore
// (indicating server-side filter), followed by a key, and optionally an
// operator (with optional negation) and target. Operators are one of
// = ^ ~ < > @ / or ?, optionally prefixed with '!'. Examples:
// "name" (key only), "name=value" (key + operator + target),
// "name=" (key + operator, no target), "name?" (exists), "name!?"
// (missing), "_tags=prod" (server-side key + operator + target).
var filterRegex = regexp.MustCompile(`^(_)?([^!?=^~<>@/]*)(!?[=^~<>@/?])?(.*)$`)

// Filter is a single parsed --filter expression including the key, operator
// token, optional negation, server-side flag, and value to match against.
type Filter struct {
	// Filter criteria
	Key      string `yaml:"key" json:"Key"`
	Operator string `yaml:"operator" json:"Operator"`
	Value    string `yaml:"value" json:"Value"`

	// Filter modifiers
	Negate     bool `yaml:"negate" json:"Negate"`
	ServerSide bool `yaml:"serverSide" json:"ServerSide"`
}

// BuildFilters parses a filter specification string into a slice of Filter.
// Invalid specs (unsupported operator or malformed expression) are skipped.
func BuildFilters(spec string) []Filter {
	//nolint:prealloc // Don't prealloc because we don't know what len will be and performance is not critical
	var filters []Filter

	// If there are no filters specified, go home early.
	if spec == "" {
		return filters
	}

	// Default delimiter is ",", allow an override for situations where the value
	// contains commas.
	delim := ","
	if d, ok := os.LookupEnv("TFQUERY_FILTER_DELIM"); ok {
		delim = d
	}

	// Split the spec and iterate over each filter spec entry.
	filterSpecs := strings.Split(spec, delim)
	for _, filterSpec := range filterSpecs {
		filterSpec = strings.TrimSpace(filterSpec)
		if filterSpec == "" {
			continue
		}

		parts := filterRegex.FindStringSubmatch(filterSpec)

		// Regex should always match, so check for nil just in case.
		if parts == nil {
			log.Error("invalid filter: " + filterSpec)
			continue
		}

		// parts[1] is the optional leading underscore (for server-side filters)
		// parts[2] is the key
		// parts[3] is the optional operator (may include negation like "!")
		// parts[4] is the optional target

		serverSide := parts[1] == "_"
		key := strings.TrimSpace(parts[2])
		operator := parts[3]
		target := parts[4]

		// If key is empty, skip this filter.
		if key == "" {
			log.Error("invalid filter: empty key in " + filterSpec)
			continue
		}

		// Handle operator negation.
		negate := strings.HasPrefix(operator, "!")
		if negate {
			operator = strings.TrimPrefix(operator, "!")
		}

		// We've got a valid filter, append it to the result set.
		filters = append(filters, Filter{
			Key:        key,
			ServerSide: serverSide,
			Negate:     negate,
			Operator:   operator,
			Value:      target,
		})
	}

	return filters
}

// FilterDataset returns a result set filtered per the provided spec. It is the
// public entry point used by SliceDiceSpit.  To be clear, this is the result
// filtering. Any server-side filtering was returned by the API.
func FilterDataset(candidates gjson.Result, attrs attrs.AttrList, spec string) []map[string]interface{} {
	//nolint:prealloc // Don't prealloc because we don't know what len will be.
	var filteredResults []map[string]interface{}

	// Build a slice of filters from the spec once so we can discard invalid
	// entries and avoid reparsing for each candidate row.
	filters := BuildFilters(spec)

	// Iterate over the candidate dataset, checking each against the filters.
	for _, candidate := range candidates.Array() {
		if !applyFilters(candidate, attrs, filters) {
			continue
		}

		// If the filter check was successful, add each attribute from the candidate
		// to the filtered result set.
		result := make(map[string]interface{})
		for i := range attrs {
			attr := attrs[i]
			// Intentionally defer Transform to SliceDiceSpit output phase.
			// This function is responsible for filtering only. Transformations
			// are applied downstream during output formatting.
			value := driller.Driller(candidate.Raw, attr.Key)
			result[attr.OutputKey] = value.Value()
		}
		filteredResults = append(filteredResults, result)
	}

	return filteredResults
}

// applyFilters returns true if the candidate row matches all of the
// provided filters. Server-side TF API filter keys (prefixed with _) are
// ignored here.
func applyFilters(candidate gjson.Result, attrs attrs.AttrList,
	filters []Filter,
) bool {
	// No filters, so go home early.
	if len(filters) == 0 {
		return true
	}

	// Iterate over the filters, checking each against the candidate.
	for _, filter := range filters {
		var key string

		// Skip server-side filters as they were applied by the API and we're not
		// interested in them here.
		if filter.ServerSide {
			continue
		}

		// Handle the special case of the hungarian filter. This filter checks if
		// the resource name follows Hungarian notation (i.e., contains tokens
		// from the resource type).
		if filter.Key == "hungarian" {
			// Get the resource type and name from the candidate.
			hungarian := isHungarian(candidate, filter)
			return hungarian == hungarianPass
		}

		// Find the attribute that matches the filter key.
		for _, attr := range attrs {
			if attr.OutputKey == filter.Key {
				key = attr.Key
				break
			}
		}

		// If an attribute matching the filter key was not found, log the condition
		// and skip this filter (continue processing other filters). This allows
		// invalid filters to be reported without rejecting the entire row.
		if key == "" {
			msg := fmt.Sprintf("filter key not found in output: %s. (ignored)", filter.Key)
			log.Error(msg)
			continue
		}

		// Get the value from the candidate for the key.
		value := driller.Driller(candidate.Raw, key).Value()

		// For unary existence checks, there is no value, so we don't evaluate any.
		if filter.Operator == "?" {
			exists := value != nil && value != ""
			// If a value exists but the filter is negated (e.g., "key!?"), then this
			// filter should fail.
			if exists == filter.Negate {
				return false
			}
			// Otherwise, it passes and we keep on going.
			continue
		}

		// All non-unary operators require a value to compare.
		if value == nil {
			return false
		}

		// Check the value against the filter. If it fails the check, fail early as
		// there's no need to continue checking the remaining filters.
		if !checkAllOperators(value, filter) {
			return false
		}
	}

	return true
}

// checkAllOperators evaluates the filter against the provided value using the
// appropriate operator check func.
func checkAllOperators(value interface{}, filter Filter) bool {
	// Try string comparison first.
	if v, ok := value.(string); ok {
		return checkStringOperand(v, filter)
	}

	// Try numeric comparison next. It's ok if this is not numeric and fails.
	if num, ok := toFloat64(value); ok {
		return checkNumericOperand(num, filter)
	}

	// Finally, if the operator is '@', we can check for membership in slices
	// or maps.
	if filter.Operator == "@" {
		return checkContainsOperand(value, filter)
	}

	// The '=' operator would have already check in the checkXXXOperand() calls
	// above. If we can't handle the type, treat '=' as a no-op filter. This
	// preserves the behavior that unsupported types do not exclude rows when the
	// user is requesting an equality match.
	// THINK Do we really want to do this? Perhaps we should just fail if '='
	// operator is used with an unsupported operand type?
	if filter.Operator == "=" {
		return true
	}

	log.Error(fmt.Sprintf("unsupported type for filtering: %T", value))
	return false
}

// hungarianCheckType represents the Hungarian filter match outcome.
type hungarianCheckType int

const (
	hungarianPass hungarianCheckType = iota
	hungarianFail
)

// checkContainsOperand evaluates a membership style filter (operator '@')
// against slice or map values.
func checkContainsOperand(value interface{}, filter Filter) bool {
	switch val := value.(type) {
	case []any:
		for _, item := range val {
			if item == filter.Value {
				return !filter.Negate
			}
		}
		return filter.Negate
	case map[string]any:
		_, found := val[filter.Value]
		if filter.Negate {
			return !found
		}
		return found
	default:
		log.Error(fmt.Sprintf("unsupported type for contains filtering: %T", value))
		return false
	}
}

// checkNumericOperand compares a numeric value against the filter value using
// numeric semantics. Supported operators: =, >, < and the negated form via
// filter.Negate (e.g., != is represented as Negate + "=").
func checkNumericOperand(value float64, filter Filter) bool {
	// Parse the value as a float64
	tgt, err := strconv.ParseFloat(strings.TrimSpace(filter.Value), 64)
	if err != nil {
		log.Error("invalid numeric value: " + filter.Value)
		return false
	}

	switch filter.Operator {
	case "=":
		return (value == tgt) == !filter.Negate
	case ">":
		return (value > tgt) == !filter.Negate
	case "<":
		return (value < tgt) == !filter.Negate
	default:
		log.Error("unsupported numeric operator: " + filter.Operator)
		return false
	}
}

// checkStringOperand evaluates a string comparison style filter against the
// provided value using the operator semantics.
func checkStringOperand(value string, filter Filter) bool {
	switch filter.Operator {
	case "=":
		return value == filter.Value == !filter.Negate
	case "~":
		return strings.EqualFold(value, filter.Value) == !filter.Negate
	case "^":
		return strings.HasPrefix(value, filter.Value) == !filter.Negate
	case ">":
		return value > filter.Value == !filter.Negate
	case "<":
		return value < filter.Value == !filter.Negate
	case "@":
		return strings.Contains(value, filter.Value) == !filter.Negate
	case "/":
		matched, err := regexp.MatchString(filter.Value, value)
		if err != nil {
			log.Error("invalid regex: " + filter.Value)
			return false
		}
		return matched == !filter.Negate
	default:
		log.Error("unsupported filtering operator: " + filter.Operator)
		return false
	}
}

// isHungarian() checks to see if the current candidate passes or fails the
// test.  There are two components of this after ensuring both fields are
// present and can be converted to string.  First, a determination to whether
// we're looking for Hungarian notation (filter.Value is "" or "true") or not
// (filter.Value is "false").  Second, we need to apply negation if specified.
func isHungarian(candidate gjson.Result, filter Filter) hungarianCheckType {
	typeVal := driller.Driller(candidate.Raw, "type").Value()
	nameVal := driller.Driller(candidate.Raw, "name").Value()

	// Both type and name must be present.
	if typeVal == nil || nameVal == nil {
		return hungarianPass
	}

	// Convert to strings.
	typeStr, typeOK := typeVal.(string)
	nameStr, nameOK := nameVal.(string)
	if !typeOK || !nameOK {
		return hungarianPass
	}

	// Determine if the resource is Hungarian notation.
	found := hungarian.IsHungarian(typeStr, nameStr)

	// Determine the result based on the filter value and negation.
	// If filter.Value is empty or "true", keep Hungarian resources.
	// If filter.Value is "false", keep non-Hungarian resources.
	mode := filter.Value == "" || filter.Value == "true"

	switch {
	case mode && !found:
		return hungarianFail
	case !mode && found:
		return hungarianFail
	}

	return hungarianPass
}

// toFloat64 attempts to normalize various numeric types to float64.
// Returns (0, false) if v is not a recognized numeric type.
func toFloat64(v interface{}) (float64, bool) {
	switch n := v.(type) {
	case float64:
		return n, true
	case float32:
		return float64(n), true
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	default:
		return 0, false
	}
}
