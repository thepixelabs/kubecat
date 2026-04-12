// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"encoding/json"
	"fmt"
)

// ComputeFieldDifferences compares two raw JSON resource blobs and returns a
// list of field-level changes.  managedFields are filtered before returning.
func ComputeFieldDifferences(oldRaw, newRaw []byte) ([]FieldChange, error) {
	var oldObj, newObj map[string]interface{}
	if err := json.Unmarshal(oldRaw, &oldObj); err != nil {
		return nil, fmt.Errorf("diff: unmarshal old: %w", err)
	}
	if err := json.Unmarshal(newRaw, &newObj); err != nil {
		return nil, fmt.Errorf("diff: unmarshal new: %w", err)
	}

	var changes []FieldChange
	walkDiff("", oldObj, newObj, &changes)

	changes = FilterManagedFields(changes)
	for i := range changes {
		changes[i].Severity = AssessSeverity(changes[i].Path)
	}

	return changes, nil
}

// walkDiff recursively compares two generic maps and appends FieldChange
// entries to changes.
func walkDiff(prefix string, old, new map[string]interface{}, changes *[]FieldChange) {
	seen := make(map[string]struct{})

	for k, oldVal := range old {
		seen[k] = struct{}{}
		path := joinPath(prefix, k)
		newVal, exists := new[k]

		if !exists {
			*changes = append(*changes, FieldChange{
				Path:     path,
				OldValue: oldVal,
				NewValue: nil,
			})
			continue
		}

		oldMap, oldIsMap := oldVal.(map[string]interface{})
		newMap, newIsMap := newVal.(map[string]interface{})

		if oldIsMap && newIsMap {
			walkDiff(path, oldMap, newMap, changes)
			continue
		}

		if !equalValues(oldVal, newVal) {
			*changes = append(*changes, FieldChange{
				Path:     path,
				OldValue: oldVal,
				NewValue: newVal,
			})
		}
	}

	// Fields added in new.
	for k, newVal := range new {
		if _, ok := seen[k]; ok {
			continue
		}
		path := joinPath(prefix, k)
		*changes = append(*changes, FieldChange{
			Path:     path,
			OldValue: nil,
			NewValue: newVal,
		})
	}
}

func joinPath(prefix, key string) string {
	if prefix == "" {
		return key
	}
	return prefix + "." + key
}

func equalValues(a, b interface{}) bool {
	aj, _ := json.Marshal(a)
	bj, _ := json.Marshal(b)
	return string(aj) == string(bj)
}
