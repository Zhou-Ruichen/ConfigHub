package pull

import "errors"

var (
	ErrManifestUnparseable = errors.New("manifest unparseable")
	ErrSchemaUnsupported   = errors.New("unsupported schema version")
	ErrSignatureAlgorithm  = errors.New("unsupported signature algorithm")
	ErrPinnedKeyMismatch   = errors.New("pinned public key mismatch")
	ErrSignatureInvalid    = errors.New("signature verification failed")
	ErrProfileMismatch     = errors.New("manifest profile id mismatch")
	ErrHTTPAuth            = errors.New("hub authentication failed")
	ErrHTTPNotFound        = errors.New("hub resource not found")
)
