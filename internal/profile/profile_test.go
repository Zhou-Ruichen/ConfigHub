package profile

import (
	"reflect"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestProfileParse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    *Profile
		wantErr bool
	}{
		{
			name: "valid profile round trip",
			input: `id: macbook
owner: ruichen
role: workstation
os: macos
domains:
  ai: true
  dotfiles: true
allowedTemplates:
  - ai/claude
  - dotfiles/git
  - dotfiles/git-include
vars:
  example: rendered-by-confighub
`,
			want: &Profile{
				ID:               "macbook",
				Owner:            "ruichen",
				Role:             "workstation",
				OS:               "macos",
				Domains:          map[string]bool{"ai": true, "dotfiles": true},
				AllowedTemplates: []string{"ai/claude", "dotfiles/git", "dotfiles/git-include"},
				Vars:             map[string]any{"example": "rendered-by-confighub"},
			},
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

func TestLoadFixtureProfile(t *testing.T) {
	got, err := Load("../../examples/profiles/macbook.yaml")
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if got.ID != "macbook" || !got.Domains["ai"] || !got.Domains["dotfiles"] {
		t.Fatalf("Load() = %#v", got)
	}
}
