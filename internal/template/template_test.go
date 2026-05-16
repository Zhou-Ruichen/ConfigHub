package template

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestTemplateParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Template
		wantErr bool
	}{
		{
			name: "valid template round trip",
			input: `id: dotfiles/git-include
domain: dotfiles
source: dotfiles/git/gitconfig-include.tmpl
target: ~/.gitconfig
mode: "0644"
delivery:
  sync: true
  remote: false
safety:
  backup: required
  diff: required
  symlink: reject
  secrets: forbidden
  merge: managed-section
  includeStrategy: append-once
`,
			want: &Template{
				ID:     "dotfiles/git-include",
				Domain: "dotfiles",
				Source: "dotfiles/git/gitconfig-include.tmpl",
				Target: "~/.gitconfig",
				Mode:   "0644",
				Delivery: Delivery{
					Sync:   true,
					Remote: false,
				},
				Safety: Safety{
					Backup:          "required",
					Diff:            "required",
					Symlink:         "reject",
					Secrets:         "forbidden",
					Merge:           "managed-section",
					IncludeStrategy: "append-once",
				},
			},
		},
		{
			name: "default secrets forbidden",
			input: `id: dotfiles/git
domain: dotfiles
source: dotfiles/git/confighub.gitconfig.tmpl
target: ~/.config/confighub/fragments/dotfiles/git/confighub.gitconfig
mode: "0644"
delivery:
  sync: true
safety:
  backup: required
  diff: required
  symlink: reject
  merge: replace
`,
			want: &Template{
				ID:     "dotfiles/git",
				Domain: "dotfiles",
				Source: "dotfiles/git/confighub.gitconfig.tmpl",
				Target: "~/.config/confighub/fragments/dotfiles/git/confighub.gitconfig",
				Mode:   "0644",
				Delivery: Delivery{
					Sync: true,
				},
				Safety: Safety{
					Backup:  "required",
					Diff:    "required",
					Symlink: "reject",
					Secrets: "forbidden",
					Merge:   "replace",
				},
			},
		},
		{
			name: "unknown merge rejected",
			input: `id: bad/template
safety:
  merge: overwrite-ish
`,
			wantErr: true,
		},
		{
			name:    "invalid yaml rejected",
			input:   "id: [unterminated\n",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Parse([]byte(tt.input))
			if tt.wantErr {
				if err == nil {
					t.Fatalf("Parse() error = nil, want error")
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse() error = %v", err)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("Parse() = %#v, want %#v", got, tt.want)
			}
			encoded, err := yaml.Marshal(got)
			if err != nil {
				t.Fatalf("yaml.Marshal() error = %v", err)
			}
			roundTrip, err := Parse(encoded)
			if err != nil {
				t.Fatalf("Parse(roundTrip) error = %v", err)
			}
			if !reflect.DeepEqual(roundTrip, got) {
				t.Fatalf("round trip = %#v, want %#v", roundTrip, got)
			}
		})
	}
}

func TestLoadFixtureTemplates(t *testing.T) {
	for _, path := range []string{
		"../../examples/templates/ai/claude.yaml",
		"../../examples/templates/dotfiles/git.yaml",
		"../../examples/templates/dotfiles/git-include.yaml",
	} {
		t.Run(path, func(t *testing.T) {
			if _, err := Load(path); err != nil {
				t.Fatalf("Load(%q) error = %v", path, err)
			}
		})
	}
}
