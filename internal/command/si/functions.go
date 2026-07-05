// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package si

import (
	"encoding/json"
	"fmt"
	"maps"
	"regexp"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/ext/tryfunc"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/zclconf/go-cty/cty"
	"github.com/zclconf/go-cty/cty/function"
	"github.com/zclconf/go-cty/cty/function/stdlib"
)

// evaluateFunction provides basic Terraform function evaluation
func evaluateFunction(expression string, stateData map[string]any) string {
	// Preprocess terraform addresses in the expression before HCL evaluation
	processedExpression := preprocessTerraformAddresses(expression, stateData)

	// Use HCL to evaluate the expression
	ctx := &hcl.EvalContext{
		Variables: buildVariableMap(stateData),
		Functions: buildFunctionMap(),
	}

	expr, diags := hclsyntax.ParseExpression([]byte(processedExpression), "", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return fmt.Sprintf("Error parsing expression: %s", diags.Error())
	}

	result, diags := expr.Value(ctx)
	if diags.HasErrors() {
		return fmt.Sprintf("Error evaluating expression: %s", diags.Error())
	}

	return formatCtyValue(result)
}

// preprocessTerraformAddresses finds terraform addresses in the expression and
// replaces them with their actual values
func preprocessTerraformAddresses(expression string, stateData map[string]any) string {
	// This regex matches terraform addresses like:
	// module.sample.aws_instance.web[0].arn
	// aws_security_group.example.id
	// data.aws_ami.ubuntu.id
	addressPattern := regexp.MustCompile(`\b(module\.[a-zA-Z0-9_.-]+\.[a-zA-Z0-9_.-]+(?:\[[^\]]+\])?\.[a-zA-Z0-9_.-]+|[a-zA-Z0-9_]+\.[a-zA-Z0-9_.-]+(?:\[[^\]]+\])?\.[a-zA-Z0-9_.-]+|data\.[a-zA-Z0-9_.-]+\.[a-zA-Z0-9_.-]+(?:\[[^\]]+\])?\.[a-zA-Z0-9_.-]+)\b`)

	return addressPattern.ReplaceAllStringFunc(expression, func(address string) string {
		// Parse the terraform address and extract its value
		parsed, err := ParseQuery(address)
		if err != nil {
			return address // Return original if parsing fails
		}

		// Find matching resources
		matches := FindMatchingResources(stateData, parsed)
		if len(matches) == 0 {
			return address // Return original if no matches
		}

		// Extract the attribute value
		attrValue := ExtractAttribute(matches[0], parsed)
		if attrValue == nil {
			return address // Return original if attribute not found
		}

		// Convert to JSON string for HCL evaluation
		jsonBytes, err := json.Marshal(attrValue)
		if err != nil {
			return address // Return original if marshalling fails
		}

		return string(jsonBytes)
	})
}

// buildFunctionMap dynamically builds the function map
// While we can't use full reflection on package exports in Go,
// we can at least make this more maintainable and systematic
func buildFunctionMap() map[string]function.Function {
	funcs := make(map[string]function.Function)

	// Define all stdlib functions in a systematic way
	// This approach makes it easy to add new functions when they become available
	stdlibFunctions := map[string]function.Function{
		// Arithmetic functions
		"abs":    stdlib.AbsoluteFunc,
		"ceil":   stdlib.CeilFunc,
		"floor":  stdlib.FloorFunc,
		"log":    stdlib.LogFunc,
		"max":    stdlib.MaxFunc,
		"min":    stdlib.MinFunc,
		"pow":    stdlib.PowFunc,
		"signum": stdlib.SignumFunc,

		// String functions
		"chomp":      stdlib.ChompFunc,
		"format":     stdlib.FormatFunc,
		"indent":     stdlib.IndentFunc,
		"join":       stdlib.JoinFunc,
		"lower":      stdlib.LowerFunc,
		"replace":    stdlib.ReplaceFunc,
		"split":      stdlib.SplitFunc,
		"substr":     stdlib.SubstrFunc,
		"title":      stdlib.TitleFunc,
		"trim":       stdlib.TrimFunc,
		"trimprefix": stdlib.TrimPrefixFunc,
		"trimspace":  stdlib.TrimSpaceFunc,
		"trimsuffix": stdlib.TrimSuffixFunc,
		"upper":      stdlib.UpperFunc,

		// Collection functions
		"chunklist":       stdlib.ChunklistFunc,
		"coalesce":        stdlib.CoalesceFunc,
		"coalescelist":    stdlib.CoalesceListFunc,
		"compact":         stdlib.CompactFunc,
		"concat":          stdlib.ConcatFunc,
		"contains":        stdlib.ContainsFunc,
		"distinct":        stdlib.DistinctFunc,
		"element":         stdlib.ElementFunc,
		"flatten":         stdlib.FlattenFunc,
		"index":           stdlib.IndexFunc,
		"keys":            stdlib.KeysFunc,
		"length":          stdlib.LengthFunc,
		"lookup":          stdlib.LookupFunc,
		"merge":           stdlib.MergeFunc,
		"reverse":         stdlib.ReverseFunc,
		"reverselist":     stdlib.ReverseListFunc,
		"setintersection": stdlib.SetIntersectionFunc,
		"setproduct":      stdlib.SetProductFunc,
		"setsubtract":     stdlib.SetSubtractFunc,
		"setunion":        stdlib.SetUnionFunc,
		"slice":           stdlib.SliceFunc,
		"sort":            stdlib.SortFunc,
		"values":          stdlib.ValuesFunc,
		"zipmap":          stdlib.ZipmapFunc,

		// Data functions
		"csvdecode":  stdlib.CSVDecodeFunc,
		"jsondecode": stdlib.JSONDecodeFunc,
		"jsonencode": stdlib.JSONEncodeFunc,
		"formatdate": stdlib.FormatDateFunc,
		"formatlist": stdlib.FormatListFunc,
		"parseint":   stdlib.ParseIntFunc,
		"range":      stdlib.RangeFunc,
		"timeadd":    stdlib.TimeAddFunc,

		// Pattern functions
		"regex":    stdlib.RegexFunc,
		"regexall": stdlib.RegexAllFunc,
	}

	// Add all functions to the map
	maps.Copy(funcs, stdlibFunctions)

	// Add HCL extension functions
	funcs["try"] = tryfunc.TryFunc
	funcs["can"] = tryfunc.CanFunc

	// Note: This approach is much more maintainable than the previous hard-coded
	// list. To add new functions from stdlib, just add them to the appropriate
	// category above.
	//
	// For true dynamic discovery, we'd need:
	// 1. The stdlib package to export a Functions() method (it doesn't)
	// 2. Or use unsafe reflection on private package state (not recommended)
	// 3. Or parse the Go source/docs to generate this list (build-time solution)
	//
	// This systematic approach is the best balance of maintainability and safety.

	return funcs
}

// buildVariableMap converts state data to cty values for HCL evaluation
func buildVariableMap(stateData map[string]any) map[string]cty.Value {
	vars := make(map[string]cty.Value)

	// Convert the entire state data
	if stateData != nil {
		vars["state"] = convertToCtyValue(stateData)

		// Also expose top-level keys directly
		for key, value := range stateData {
			vars[key] = convertToCtyValue(value)
		}
	}

	return vars
}

// convertToCtyValue converts Go values to cty values
func convertToCtyValue(val any) cty.Value {
	switch v := val.(type) {
	case nil:
		return cty.NullVal(cty.DynamicPseudoType)
	case bool:
		return cty.BoolVal(v)
	case int:
		return cty.NumberIntVal(int64(v))
	case int64:
		return cty.NumberIntVal(v)
	case float64:
		return cty.NumberFloatVal(v)
	case string:
		return cty.StringVal(v)
	case []any:
		vals := make([]cty.Value, len(v))
		for i, item := range v {
			vals[i] = convertToCtyValue(item)
		}
		return cty.TupleVal(vals)
	case map[string]any:
		vals := make(map[string]cty.Value)
		for key, item := range v {
			vals[key] = convertToCtyValue(item)
		}
		return cty.ObjectVal(vals)
	default:
		// Fallback to string representation
		return cty.StringVal(fmt.Sprintf("%v", v))
	}
}

// formatCtyValue converts a cty value back to a string for display
func formatCtyValue(val cty.Value) string {
	if val.IsNull() {
		return "null"
	}

	switch val.Type() {
	case cty.Bool:
		return fmt.Sprintf("%t", val.True())
	case cty.Number:
		bf := val.AsBigFloat()
		if bf.IsInt() {
			i, _ := bf.Int64()
			return fmt.Sprintf("%d", i)
		}
		f, _ := bf.Float64()
		return fmt.Sprintf("%g", f)
	case cty.String:
		return val.AsString()
	default:
		// For complex types, convert to JSON
		goVal := ctyValueToGo(val)
		if jsonBytes, err := json.Marshal(goVal); err == nil {
			return string(jsonBytes)
		}
		return fmt.Sprintf("%#v", goVal)
	}
}

// ctyValueToGo converts cty values to Go values
func ctyValueToGo(val cty.Value) any {
	if val.IsNull() {
		return nil
	}

	switch {
	case val.Type() == cty.Bool:
		return val.True()
	case val.Type() == cty.Number:
		bf := val.AsBigFloat()
		if bf.IsInt() {
			i, _ := bf.Int64()
			return i
		}
		f, _ := bf.Float64()
		return f
	case val.Type() == cty.String:
		return val.AsString()
	case val.Type().IsTupleType():
		var result []any
		for it := val.ElementIterator(); it.Next(); {
			_, elemVal := it.Element()
			result = append(result, ctyValueToGo(elemVal))
		}
		return result
	case val.Type().IsObjectType():
		result := make(map[string]any)
		for it := val.ElementIterator(); it.Next(); {
			keyVal, elemVal := it.Element()
			key := keyVal.AsString()
			result[key] = ctyValueToGo(elemVal)
		}
		return result
	default:
		return fmt.Sprintf("%#v", val)
	}
}
