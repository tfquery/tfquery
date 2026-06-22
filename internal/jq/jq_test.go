// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0
// no-cloc

package jq

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"

	"github.com/tfquery/tfquery/internal/attrs"
)

func TestFilterDatasetJQ(t *testing.T) {
	testData := `[
		{"id":"1","name":"alpha","count":1,"nested":{"inner":"x"}},
		{"id":"2","name":"beta","count":2,"nested":{}},
		{"id":"3","name":"gamma","count":3}
	]`

	attrList := attrs.AttrList{
		{Key: "id", OutputKey: "id", Include: true},
		{Key: "name", OutputKey: "name", Include: true},
		{Key: "nested.inner", OutputKey: "nested.inner", Include: true},
	}

	tests := []struct {
		name    string
		query   string
		wantIDs []string
		wantErr bool
	}{
		{
			name:    "simple equals",
			query:   `.name == "beta"`,
			wantIDs: []string{"2"},
		},
		{
			name:    "and operator",
			query:   `.name == "beta" and .id == "2"`,
			wantIDs: []string{"2"},
		},
		{
			name:    "or operator",
			query:   `.id == "1" or .id == "3"`,
			wantIDs: []string{"1", "3"},
		},
		{
			name:    "flat dotted output key exists",
			query:   `.["nested.inner"] != null`,
			wantIDs: []string{"1"},
		},
		{
			name:    "invalid jq query",
			query:   `.name =`,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FilterDatasetJQ(gjson.Parse(testData), attrList, tt.query)
			if tt.wantErr {
				require.Error(t, err)
				return
			}

			require.NoError(t, err)
			gotIDs := make([]string, 0, len(got))
			for _, row := range got {
				gotIDs = append(gotIDs, row["id"].(string))
			}
			assert.Equal(t, tt.wantIDs, gotIDs)
		})
	}
}

func TestFilterDatasetJQ_UsesOutputKeys(t *testing.T) {
	testData := `[
		{"id":"1","attributes":{"name":"staranto"}},
		{"id":"2","attributes":{"name":"tfquery"}}
	]`

	attrList := attrs.AttrList{
		{Key: "id", OutputKey: "id", Include: true},
		{Key: "attributes.name", OutputKey: "name", Include: true},
	}

	got, err := FilterDatasetJQ(
		gjson.Parse(testData),
		attrList,
		`.name | contains("tar")`,
	)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, "1", got[0]["id"])
	assert.Equal(t, "staranto", got[0]["name"])
}
