package bundle

// Manifest is the machine-readable contract for one immutable ConfigHub bundle.
type Manifest struct {
	SchemaVersion  string             `json:"schemaVersion"`
	BundleVersion  string             `json:"bundleVersion"`
	ProfileID      string             `json:"profileId"`
	CreatedAt      string             `json:"createdAt"`
	SourceRevision string             `json:"sourceRevision"`
	Domains        []string           `json:"domains"`
	Files          []FileEntry        `json:"files"`
	RemovedFiles   []RemovedFileEntry `json:"removedFiles"`
	ChangeSummary  string             `json:"changeSummary"`
	Signature      *Signature         `json:"signature"`
}

// FileEntry describes one rendered file in a bundle.
type FileEntry struct {
	TemplateID string `json:"templateId"`
	Domain     string `json:"domain"`
	BundlePath string `json:"bundlePath"`
	TargetPath string `json:"targetPath"`
	Mode       string `json:"mode"`
	Checksum   string `json:"checksum"`
	Delivery   string `json:"delivery"`
	Safety     Safety `json:"safety"`
}

// Safety declares write, diff, symlink, secret, merge, and include policies for
// a manifest file entry.
type Safety struct {
	Backup          string `json:"backup"`
	Diff            string `json:"diff"`
	Symlink         string `json:"symlink"`
	Secrets         string `json:"secrets"`
	Merge           string `json:"merge"`
	IncludeStrategy string `json:"includeStrategy,omitempty"`
}

// RemovedFileEntry describes a file that should be deleted during apply if the
// local checksum still matches the previous checksum.
type RemovedFileEntry struct {
	TemplateID       string `json:"templateId,omitempty"`
	TargetPath       string `json:"targetPath"`
	Reason           string `json:"reason"`
	PreviousChecksum string `json:"previousChecksum"`
}

// Signature is reserved for bundle signatures required in a later slice.
type Signature struct {
	Algorithm string `json:"algorithm,omitempty"`
	Value     string `json:"value,omitempty"`
}
