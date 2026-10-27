// Package authn resolves the caller identity of an API request.
package authn

import (
	"crypto/sha256"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"

	"github.com/hilbras/omniconnector/packages/shared-contracts"
)

// ErrUnauthenticated means the request carried no valid credentials
var ErrUnauthenticated = errors.New("unauthenticated")

// Authenticator resolves a request to a caller identity
type Authenticator interface {
	Authenticate(r *http.Request) (*contracts.IdentityContext, error)
}

// DevToken accepts one static bearer token and maps it to a fixed identity.
// It exists only until user authentication lands in Phase 2. With an empty
// token it rejects every request, so an unconfigured server is closed.
type DevToken struct {
	digest      [32]byte
	enabled     bool
	userID      string
	permissions []string
}

// NewDevToken creates a DevToken authenticator
func NewDevToken(token, userID string, permissions []string) *DevToken {
	return &DevToken{
		digest:      sha256.Sum256([]byte(token)),
		enabled:     token != "",
		userID:      userID,
		permissions: append([]string(nil), permissions...),
	}
}

// Authenticate checks the bearer token in constant time
func (d *DevToken) Authenticate(r *http.Request) (*contracts.IdentityContext, error) {
	if !d.enabled {
		return nil, ErrUnauthenticated
	}
	token, ok := bearerToken(r)
	if !ok {
		return nil, ErrUnauthenticated
	}
	// Comparing digests keeps the comparison constant-time regardless of length
	got := sha256.Sum256([]byte(token))
	if subtle.ConstantTimeCompare(got[:], d.digest[:]) != 1 {
		return nil, ErrUnauthenticated
	}
	return &contracts.IdentityContext{
		UserID:      d.userID,
		Permissions: append([]string(nil), d.permissions...),
	}, nil
}

func bearerToken(r *http.Request) (string, bool) {
	h := r.Header.Get("Authorization")
	const prefix = "Bearer "
	if len(h) <= len(prefix) || !strings.EqualFold(h[:len(prefix)], prefix) {
		return "", false
	}
	return strings.TrimSpace(h[len(prefix):]), true
}
