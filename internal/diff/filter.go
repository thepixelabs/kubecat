package diff

import "strings"

// managedFieldsPrefix is the JSON path prefix for fields we want to suppress.
var suppressedPrefixes = []string{
	"metadata.managedFields",
	"metadata.resourceVersion",
	"metadata.generation",
	"metadata.uid",
	"metadata.creationTimestamp",
	"status.",
}

// FilterManagedFields removes noise fields from a diff result.
// It discards fields that Kubernetes manages internally (managedFields,
// resourceVersion, status, etc.) so the diff surfaces only intent-level
// changes that the operator actually cares about.
func FilterManagedFields(changes []FieldChange) []FieldChange {
	out := changes[:0]
	for _, c := range changes {
		if isSuppressed(c.Path) {
			continue
		}
		out = append(out, c)
	}
	return out
}

func isSuppressed(path string) bool {
	for _, prefix := range suppressedPrefixes {
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}
