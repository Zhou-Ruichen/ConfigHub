package template

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const defaultSecretsPolicy = "forbidden"

var allowedMergeStrategies = map[string]bool{
	"":                true,
	"replace":         true,
	"managed-section": true,
	"deep-merge":      true,
}

// Template describes one source template, its target, delivery modes, and
// safety policies.
type Template struct {
	ID       string   `yaml:"id" json:"id"`
	Domain   string   `yaml:"domain" json:"domain"`
	Source   string   `yaml:"source" json:"source"`
	Target   string   `yaml:"target" json:"target"`
	Mode     string   `yaml:"mode" json:"mode"`
	Delivery Delivery `yaml:"delivery" json:"delivery"`
	Safety   Safety   `yaml:"safety" json:"safety"`
}

// Delivery declares how rendered output can be consumed.
type Delivery struct {
	Sync   bool `yaml:"sync" json:"sync"`
	Remote bool `yaml:"remote" json:"remote"`
}

// Safety declares write, diff, symlink, secret, merge, and include policies.
type Safety struct {
	Backup          string `yaml:"backup" json:"backup"`
	Diff            string `yaml:"diff" json:"diff"`
	Symlink         string `yaml:"symlink" json:"symlink"`
	Secrets         string `yaml:"secrets" json:"secrets"`
	Merge           string `yaml:"merge" json:"merge"`
	IncludeStrategy string `yaml:"includeStrategy,omitempty" json:"includeStrategy,omitempty"`
}

// Load reads a template definition from YAML and validates the external contract.
func Load(path string) (*Template, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read template %q: %w", path, err)
	}
	return Parse(data)
}

// Parse decodes a template from YAML bytes, applies defaults, and validates it.
func Parse(data []byte) (*Template, error) {
	var tmpl Template
	if err := yaml.Unmarshal(data, &tmpl); err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	ApplyDefaults(&tmpl)
	if err := Validate(&tmpl); err != nil {
		return nil, err
	}
	return &tmpl, nil
}

// ApplyDefaults fills template defaults shared by YAML templates and manifests.
func ApplyDefaults(tmpl *Template) {
	if tmpl.Safety.Secrets == "" {
		tmpl.Safety.Secrets = defaultSecretsPolicy
	}
}

// Validate checks the safety policy values known in Slice 1.
func Validate(tmpl *Template) error {
	if tmpl.ID == "" {
		return fmt.Errorf("validate template: id is required")
	}
	if !allowedMergeStrategies[tmpl.Safety.Merge] {
		return fmt.Errorf("validate template %q: unsupported merge value %q", tmpl.ID, tmpl.Safety.Merge)
	}
	return nil
}
