package metadata

import (
	"fmt"
	"strings"
	"time"
)

// FormatAge returns a human-readable age string for a given duration.
func FormatAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	if days < 7 {
		return fmt.Sprintf("%dd", days)
	}
	return fmt.Sprintf("%dw", days/7)
}

// FormatLabels formats a label map as a comma-separated "key=value" string.
func FormatLabels(labels map[string]string) string {
	if len(labels) == 0 {
		return ""
	}
	parts := make([]string, 0, len(labels))
	for k, v := range labels {
		parts = append(parts, k+"="+v)
	}
	return strings.Join(parts, ", ")
}

// FormatPorts formats a slice of port strings into a single display string.
func FormatPorts(ports []string) string {
	return strings.Join(ports, ", ")
}

// FormatAccessModes formats PVC access modes into a short representation.
func FormatAccessModes(modes []string) string {
	short := make([]string, 0, len(modes))
	for _, m := range modes {
		switch m {
		case "ReadWriteOnce":
			short = append(short, "RWO")
		case "ReadOnlyMany":
			short = append(short, "ROX")
		case "ReadWriteMany":
			short = append(short, "RWX")
		case "ReadWriteOncePod":
			short = append(short, "RWOP")
		default:
			short = append(short, m)
		}
	}
	return strings.Join(short, ",")
}
