package apply

import (
	"errors"
	"io"
)

var (
	ErrPathPolicy = errors.New("target path policy rejected")
)

const (
	ActionWrote                = "wrote"
	ActionUnchanged            = "unchanged"
	ActionManagedSectionUpdate = "managed-section-update"
	ActionIncludedOnce         = "included-once"
	ActionRemoved              = "removed"
	ActionRemovedNoop          = "removed-noop"
)

// Options configures an apply operation. HomeDir is injectable for tests and is
// used for ~ expansion; callers should pass os.UserHomeDir() in the CLI.
type Options struct {
	BundleDir string
	RootDir   string
	StateDir  string
	HomeDir   string
	ProfileID string
	DryRun    bool
	Yes       bool
	JSON      bool
	Out       io.Writer
}

// Result describes the actions computed and optionally executed for a bundle.
type Result struct {
	ProfileID     string       `json:"profileId"`
	BundleVersion string       `json:"bundleVersion"`
	BackupDir     string       `json:"backupDir,omitempty"`
	DryRun        bool         `json:"dryRun"`
	Files         []FileAction `json:"files"`
	RemovedFiles  []FileAction `json:"removedFiles"`
	Diff          string       `json:"diff,omitempty"`
}

// FileAction is safe to persist in state/apply.log. It never contains rendered
// bytes or backup file contents.
type FileAction struct {
	TemplateID       string  `json:"templateId,omitempty"`
	TargetPath       string  `json:"targetPath"`
	Checksum         string  `json:"checksum,omitempty"`
	Action           string  `json:"action"`
	PreviousChecksum *string `json:"previousChecksum"`
	BackupRelPath    string  `json:"backupRelPath,omitempty"`
}
