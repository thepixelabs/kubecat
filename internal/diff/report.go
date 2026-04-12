// SPDX-License-Identifier: Apache-2.0

package diff

import (
	"fmt"
	"strings"
)

// criticalPaths are field path prefixes that indicate a high-impact change.
var criticalPaths = []string{
	"spec.replicas",
	"spec.containers",
	"spec.template",
	"spec.selector",
}

// warningPaths are field path prefixes indicating a medium-impact change.
var warningPaths = []string{
	"spec.",
	"metadata.labels",
	"metadata.annotations",
}

// AssessSeverity classifies the severity of a change based on the field path.
func AssessSeverity(path string) ChangeSeverity {
	for _, p := range criticalPaths {
		if strings.HasPrefix(path, p) {
			return SeverityCritical
		}
	}
	for _, p := range warningPaths {
		if strings.HasPrefix(path, p) {
			return SeverityWarning
		}
	}
	return SeverityInfo
}

// GenerateMarkdownReport renders a DiffResult as a human-readable Markdown
// summary suitable for embedding in AI responses or the UI.
func GenerateMarkdownReport(result DiffResult) string {
	if len(result.Changes) == 0 {
		return fmt.Sprintf("No changes detected for **%s** `%s/%s`.\n",
			result.Kind, result.Namespace, result.Name)
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("## Changes for %s `%s/%s`\n\n", result.Kind, result.Namespace, result.Name))
	sb.WriteString(fmt.Sprintf("**%d** field(s) changed:\n\n", len(result.Changes)))
	sb.WriteString("| Severity | Field | Old Value | New Value |\n")
	sb.WriteString("|----------|-------|-----------|----------|\n")

	for _, c := range result.Changes {
		severity := severityEmoji(c.Severity)
		sb.WriteString(fmt.Sprintf("| %s %s | `%s` | `%v` | `%v` |\n",
			severity, string(c.Severity),
			c.Path,
			formatValue(c.OldValue),
			formatValue(c.NewValue),
		))
	}

	return sb.String()
}

func severityEmoji(s ChangeSeverity) string {
	switch s {
	case SeverityCritical:
		return "!"
	case SeverityWarning:
		return "~"
	default:
		return " "
	}
}

func formatValue(v interface{}) string {
	if v == nil {
		return "<removed>"
	}
	s := fmt.Sprintf("%v", v)
	if len(s) > 60 {
		return s[:57] + "..."
	}
	return s
}
