package sign

import (
	"crypto/ed25519"
	"encoding/base64"
	"fmt"

	"github.com/ruichen/config-hub/internal/bundle"
)

func SignManifest(m *bundle.Manifest, priv ed25519.PrivateKey) (*bundle.Signature, error) {
	if len(priv) != ed25519.PrivateKeySize {
		return nil, fmt.Errorf("sign manifest: private key length %d", len(priv))
	}
	payload, err := CanonicalManifestBytes(m)
	if err != nil {
		return nil, err
	}
	sig := ed25519.Sign(priv, payload)
	return &bundle.Signature{Algorithm: AlgorithmEd25519, Value: base64.StdEncoding.EncodeToString(sig)}, nil
}

func VerifyManifest(m *bundle.Manifest, sig *bundle.Signature, pub ed25519.PublicKey) error {
	if sig == nil || sig.Algorithm != AlgorithmEd25519 {
		return ErrSignatureAlgorithm
	}
	if len(pub) != ed25519.PublicKeySize {
		return fmt.Errorf("verify manifest: public key length %d", len(pub))
	}
	decoded, err := base64.StdEncoding.DecodeString(sig.Value)
	if err != nil || len(decoded) != ed25519.SignatureSize {
		return ErrSignatureInvalid
	}
	payload, err := CanonicalManifestBytes(m)
	if err != nil {
		return err
	}
	if !ed25519.Verify(pub, payload, decoded) {
		return ErrSignatureInvalid
	}
	return nil
}
