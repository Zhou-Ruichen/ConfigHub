package apply

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
)

func MarkerName(templateID string) string {
	return strings.ReplaceAll(templateID, "/", "-")
}

func ValidateMarkerSupported(targetPath string) error {
	ext := strings.ToLower(filepath.Ext(targetPath))
	base := strings.ToLower(filepath.Base(targetPath))
	if ext == ".json" {
		return fmt.Errorf("%w: json target %q cannot use managed-section in Slice 3", ErrPathPolicy, targetPath)
	}
	switch ext {
	case ".sh", ".zsh", ".bash", ".toml", ".conf":
		return nil
	}
	switch base {
	case ".zshrc", ".bashrc", ".bash_profile", ".profile", ".gitconfig", ".tmux.conf", ".vimrc", "config", "gitconfig":
		return nil
	}
	return fmt.Errorf("%w: marker style not registered for target %q", ErrPathPolicy, targetPath)
}

func MergeManagedSection(existing []byte, markerName string, payload []byte) ([]byte, error) {
	open := []byte("# >>> confighub:" + markerName + " >>>")
	close := []byte("# <<< confighub:" + markerName + " <<<")
	lineEnding := detectLineEnding(existing)
	pairs, err := findMarkerPairs(existing, open, close)
	if err != nil {
		return nil, err
	}
	if len(pairs) > 1 {
		return nil, fmt.Errorf("multiple marker blocks found for %q", markerName)
	}
	payload = bytes.TrimRight(payload, "\r\n")
	if len(pairs) == 0 {
		out := append([]byte{}, existing...)
		if len(out) > 0 && !bytes.HasSuffix(out, []byte("\n")) && !bytes.HasSuffix(out, []byte("\r\n")) {
			out = append(out, lineEnding...)
		}
		if len(out) > 0 {
			out = append(out, lineEnding...)
		}
		out = append(out, open...)
		out = append(out, lineEnding...)
		out = append(out, payload...)
		out = append(out, lineEnding...)
		out = append(out, close...)
		out = append(out, lineEnding...)
		return out, nil
	}
	pair := pairs[0]
	out := append([]byte{}, existing[:pair.contentStart]...)
	out = append(out, payload...)
	out = append(out, lineEnding...)
	out = append(out, existing[pair.contentEnd:]...)
	return out, nil
}

func RemoveManagedSection(existing []byte, markerName string) ([]byte, error) {
	open := []byte("# >>> confighub:" + markerName + " >>>")
	close := []byte("# <<< confighub:" + markerName + " <<<")
	lineEnding := detectLineEnding(existing)
	pairs, err := findMarkerLinePairs(existing, open, close)
	if err != nil {
		return nil, err
	}
	if len(pairs) == 0 {
		return append([]byte{}, existing...), nil
	}
	if len(pairs) > 1 {
		return nil, fmt.Errorf("multiple marker blocks found for %q", markerName)
	}
	pair := pairs[0]
	out := append([]byte{}, existing[:pair.contentStart]...)
	out = append(out, existing[pair.contentEnd:]...)
	for len(out) > 0 && bytes.HasSuffix(out, lineEnding) {
		trimmed := out[:len(out)-len(lineEnding)]
		if !bytes.HasSuffix(trimmed, lineEnding) {
			break
		}
		out = trimmed
	}
	return out, nil
}

func MarkerBlockPresent(existing []byte, markerName string) (bool, error) {
	open := []byte("# >>> confighub:" + markerName + " >>>")
	close := []byte("# <<< confighub:" + markerName + " <<<")
	pairs, err := findMarkerPairs(existing, open, close)
	if err != nil {
		return false, err
	}
	if len(pairs) > 1 {
		return false, fmt.Errorf("multiple marker blocks found for %q", markerName)
	}
	return len(pairs) == 1, nil
}

type markerPair struct {
	contentStart int
	contentEnd   int
}

func findMarkerPairs(data, open, close []byte) ([]markerPair, error) {
	pairs := []markerPair{}
	pos := 0
	for {
		openStartRel := bytes.Index(data[pos:], open)
		if openStartRel < 0 {
			break
		}
		openStart := pos + openStartRel
		openLineEnd := lineEndIndex(data, openStart)
		contentStart := openLineEnd
		if contentStart < len(data) && data[contentStart] == '\r' {
			contentStart++
		}
		if contentStart < len(data) && data[contentStart] == '\n' {
			contentStart++
		}
		closeStartRel := bytes.Index(data[contentStart:], close)
		if closeStartRel < 0 {
			return nil, fmt.Errorf("missing close marker for %q", string(open))
		}
		closeStart := contentStart + closeStartRel
		contentEnd := closeStart
		pairs = append(pairs, markerPair{contentStart: contentStart, contentEnd: contentEnd})
		pos = closeStart + len(close)
	}
	return pairs, nil
}

func findMarkerLinePairs(data, open, close []byte) ([]markerPair, error) {
	pairs := []markerPair{}
	pos := 0
	for {
		openStartRel := bytes.Index(data[pos:], open)
		if openStartRel < 0 {
			break
		}
		openStart := pos + openStartRel
		lineStart := openStart
		for lineStart > 0 && data[lineStart-1] != '\n' && data[lineStart-1] != '\r' {
			lineStart--
		}
		contentStart := lineStart
		openLineEnd := lineEndIndex(data, openStart)
		afterOpen := openLineEnd
		if afterOpen < len(data) && data[afterOpen] == '\r' {
			afterOpen++
		}
		if afterOpen < len(data) && data[afterOpen] == '\n' {
			afterOpen++
		}
		closeStartRel := bytes.Index(data[afterOpen:], close)
		if closeStartRel < 0 {
			return nil, fmt.Errorf("missing close marker for %q", string(open))
		}
		closeStart := afterOpen + closeStartRel
		closeLineEnd := lineEndIndex(data, closeStart)
		contentEnd := closeLineEnd
		if contentEnd < len(data) && data[contentEnd] == '\r' {
			contentEnd++
		}
		if contentEnd < len(data) && data[contentEnd] == '\n' {
			contentEnd++
		}
		pairs = append(pairs, markerPair{contentStart: contentStart, contentEnd: contentEnd})
		pos = contentEnd
	}
	return pairs, nil
}

func lineEndIndex(data []byte, start int) int {
	for i := start; i < len(data); i++ {
		if data[i] == '\n' || data[i] == '\r' {
			return i
		}
	}
	return len(data)
}

func detectLineEnding(data []byte) []byte {
	for i := 0; i < len(data); i++ {
		if data[i] == '\n' {
			if i > 0 && data[i-1] == '\r' {
				return []byte("\r\n")
			}
			return []byte("\n")
		}
	}
	return []byte("\n")
}
