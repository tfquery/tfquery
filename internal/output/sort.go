// Copyright (c) 2026 Steve Taranto <staranto@gmail.com>.
// SPDX-License-Identifier: Apache-2.0

package output

import (
	"sort"
	"strings"
)

// THINK Issue 5
func SortDataset(resultSet []map[string]any, spec string) {
	fields := strings.Split(spec, ",")

	sort.SliceStable(resultSet, func(one, two int) bool {
		for _, field := range fields {
			ascending := true
			if after, ok := strings.CutPrefix(field, "-"); ok {
				field = after
				ascending = false
			}

			caseSensitive := false
			if after, ok := strings.CutPrefix(field, "!"); ok {
				field = after
				caseSensitive = true
			}

			oneValue := resultSet[one][field]
			twoValue := resultSet[two][field]

			// Convert to integers if possible
			oneInt, oneOk := oneValue.(float64)
			twoInt, twoOk := twoValue.(float64)

			if oneOk && twoOk {
				if int(oneInt) != int(twoInt) {
					if ascending {
						return int(oneInt) < int(twoInt)
					}
					return int(oneInt) > int(twoInt)
				}
				continue
			}

			// Fall back to string comparison which can also handle bools.
			oneStr := InterfaceToString(oneValue)
			twoStr := InterfaceToString(twoValue)

			compareOneStr := oneStr
			compareTwoStr := twoStr
			if !caseSensitive {
				compareOneStr = strings.ToLower(oneStr)
				compareTwoStr = strings.ToLower(twoStr)
			}

			if compareOneStr != compareTwoStr {
				if ascending {
					return compareOneStr < compareTwoStr
				}
				return compareOneStr > compareTwoStr
			}
		}
		return false
	})
}
