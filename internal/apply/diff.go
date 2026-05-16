package apply

import (
	"bytes"
	"fmt"
	"strings"
)

const maxDiffBytes = 1 << 20

func RenderDiff(targetPath string, existing, rendered []byte) string {
	if len(existing) > maxDiffBytes || len(rendered) > maxDiffBytes || isBinary(existing) || isBinary(rendered) {
		if bytes.Equal(existing, rendered) {
			return fmt.Sprintf("%s: no change\n", targetPath)
		}
		return fmt.Sprintf("%s: changed (diff omitted: file is binary or larger than 1 MiB)\n", targetPath)
	}
	if bytes.Equal(existing, rendered) {
		return fmt.Sprintf("%s: no change\n", targetPath)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "--- %s (existing)\n", targetPath)
	fmt.Fprintf(&b, "+++ %s (rendered)\n", targetPath)
	oldLines := splitLines(existing)
	newLines := splitLines(rendered)
	max := len(oldLines)
	if len(newLines) > max {
		max = len(newLines)
	}
	for i := 0; i < max; i++ {
		var old, neu string
		if i < len(oldLines) {
			old = oldLines[i]
		}
		if i < len(newLines) {
			neu = newLines[i]
		}
		switch {
		case i >= len(oldLines):
			fmt.Fprintf(&b, "+%s\n", truncateDiffLine(neu))
		case i >= len(newLines):
			fmt.Fprintf(&b, "-%s\n", truncateDiffLine(old))
		case old == neu:
			fmt.Fprintf(&b, " %s\n", truncateDiffLine(old))
		default:
			fmt.Fprintf(&b, "-%s\n", truncateDiffLine(old))
			fmt.Fprintf(&b, "+%s\n", truncateDiffLine(neu))
		}
	}
	return b.String()
}

func splitLines(data []byte) []string {
	s := strings.TrimSuffix(string(data), "\n")
	if s == "" {
		return []string{}
	}
	return strings.Split(s, "\n")
}

func truncateDiffLine(line string) string {
	const limit = 4096
	if len(line) <= limit {
		return strings.TrimSuffix(line, "\r")
	}
	return line[:limit] + "... [truncated]"
}

func isBinary(data []byte) bool {
	return bytes.IndexByte(data, 0) >= 0
}
