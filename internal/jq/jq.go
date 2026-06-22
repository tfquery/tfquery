// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package jq

import (
	"fmt"

	"github.com/itchyny/gojq"
	"github.com/tidwall/gjson"

	"github.com/tfquery/tfquery/internal/attrs"
	"github.com/tfquery/tfquery/internal/driller"
)

// FilterDatasetJQ applies a jq expression to each candidate row and returns
// only rows where the expression evaluates to a truthy value.
func FilterDatasetJQ(candidates gjson.Result,
	attrs attrs.AttrList,
	querySpec string,
) ([]map[string]interface{}, error) {
	//nolint:prealloc // We don't know resulting len before applying the query.
	var filteredResults []map[string]interface{}

	query, err := gojq.Parse(querySpec)
	if err != nil {
		return nil, fmt.Errorf("parse jq query: %w", err)
	}

	for _, candidate := range candidates.Array() {
		result := make(map[string]interface{})
		for i := range attrs {
			attr := attrs[i]
			value := driller.Driller(candidate.Raw, attr.Key)
			result[attr.OutputKey] = value.Value()
		}

		matched, err := rowMatchesJQ(result, query)
		if err != nil {
			return nil, fmt.Errorf("evaluate jq query: %w", err)
		}

		if !matched {
			continue
		}

		filteredResults = append(filteredResults, result)
	}

	return filteredResults, nil
}

// rowMatchesJQ evaluates a compiled jq query against a projected row map and
// returns whether any emitted value is truthy.
func rowMatchesJQ(row map[string]interface{}, query *gojq.Query) (bool, error) {
	iter := query.Run(row)

	for {
		value, ok := iter.Next()
		if !ok {
			break
		}

		if err, isErr := value.(error); isErr {
			return false, err
		}

		if isTruthyJQValue(value) {
			return true, nil
		}
	}

	return false, nil
}

// isTruthyJQValue applies jq truthiness rules where only false and null are
// falsey values. nil's are always falsey and non-bool values are always truthy.s
func isTruthyJQValue(v interface{}) bool {
	if v == nil {
		return false
	}

	if b, ok := v.(bool); ok {
		return b
	}

	return true
}
