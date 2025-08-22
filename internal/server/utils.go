package server

import "strings"

// splitAndTrimCSV splits a comma-separated string and trims whitespace from each element.
// Empty results are omitted.
func splitAndTrimCSV(s string) []string {
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		v := strings.TrimSpace(p)
		if v != "" {
			out = append(out, v)
		}
	}
	return out
}
