package domain

import (
	"crypto/sha256"
	"fmt"
	"strings"
)

// ShortPathHash returns a 32-character hex prefix of the SHA-256 of the given
// parts, joined by a NUL byte that cannot occur in a path or a git ref. The
// prefix keeps 128 bits so a collision stays out of reach of anyone who can
// influence a part, such as a suggested --git-ref value.
func ShortPathHash(parts ...string) string {
	hasher := sha256.New()
	fmt.Fprintf(hasher, "%s", strings.Join(parts, "\x00"))
	return fmt.Sprintf("%x", hasher.Sum(nil))[:32]
}
