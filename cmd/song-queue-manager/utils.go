package main

import (
	"fmt"
	"strings"
)

func buildAttribution(t Track) string {
	var parts []string
	parts = append(parts, fmt.Sprintf("%s - %s", t.Artist, t.Track))

	if t.SourceMedia != "" {
		parts = append(parts, fmt.Sprintf("Source media: (%s)", t.SourceMedia))
	}
	if t.Album != "" {
		parts = append(parts, fmt.Sprintf("Album: [%s]", t.Album))
	}
	if t.OriginalComposer != "" {
		parts = append(parts, fmt.Sprintf("Composed by: %s", t.OriginalComposer))
	}

	return strings.Join(parts, "\n")
}
