package sign

import "errors"

var (
	ErrSignatureAlgorithm = errors.New("unsupported signature algorithm")
	ErrSignatureInvalid   = errors.New("signature verification failed")
)
