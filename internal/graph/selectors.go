// SPDX-License-Identifier: Apache-2.0

package graph

import "strings"

// ParseSelectors parses a "key1=val1, key2=val2" string into a map.
func ParseSelectors(raw string) map[string]string {
	result := make(map[string]string)
	if raw == "" {
		return result
	}
	for _, pair := range strings.Split(raw, ",") {
		pair = strings.TrimSpace(pair)
		idx := strings.IndexByte(pair, '=')
		if idx <= 0 {
			continue
		}
		result[strings.TrimSpace(pair[:idx])] = strings.TrimSpace(pair[idx+1:])
	}
	return result
}

// MatchLabels returns true when all selector entries are satisfied by labels.
func MatchLabels(labels, selector map[string]string) bool {
	for k, v := range selector {
		if labels[k] != v {
			return false
		}
	}
	return true
}
