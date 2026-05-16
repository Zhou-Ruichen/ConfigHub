package profile

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Profile describes the operator, machine, role, domains, and templates selected
// for a ConfigHub render.
type Profile struct {
	ID               string          `yaml:"id" json:"id"`
	Owner            string          `yaml:"owner" json:"owner"`
	Role             string          `yaml:"role" json:"role"`
	OS               string          `yaml:"os" json:"os"`
	Domains          map[string]bool `yaml:"domains" json:"domains"`
	AllowedTemplates []string        `yaml:"allowedTemplates" json:"allowedTemplates"`
}

// Load reads a profile from a YAML file and validates the external contract.
func Load(path string) (*Profile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read profile %q: %w", path, err)
	}
	return Parse(data)
}

// Parse decodes a profile from YAML bytes and rejects malformed or empty input.
func Parse(data []byte) (*Profile, error) {
	var profile Profile
	if err := yaml.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("parse profile: %w", err)
	}
	if profile.ID == "" {
		return nil, fmt.Errorf("validate profile: id is required")
	}
	if len(profile.Domains) == 0 {
		return nil, fmt.Errorf("validate profile %q: at least one domain is required", profile.ID)
	}
	return &profile, nil
}
