package sign

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/ruichen/config-hub/internal/bundle"
)

func CanonicalManifestBytes(m *bundle.Manifest) ([]byte, error) {
	if m == nil {
		return nil, fmt.Errorf("canonical manifest: nil manifest")
	}
	data, err := json.Marshal(m)
	if err != nil {
		return nil, fmt.Errorf("canonical manifest clone: %w", err)
	}
	var clone bundle.Manifest
	dec := json.NewDecoder(bytes.NewReader(data))
	if err := dec.Decode(&clone); err != nil {
		return nil, fmt.Errorf("canonical manifest decode clone: %w", err)
	}
	clone.Signature = nil
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(&clone); err != nil {
		return nil, fmt.Errorf("canonical manifest: %w", err)
	}
	return []byte(strings.TrimSuffix(buf.String(), "\n")), nil
}
