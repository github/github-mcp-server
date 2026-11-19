package skills

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
)

// DigestPrefix is the algorithm prefix of every digest this extension defines.
const DigestPrefix = "sha256:"

// digestHexRe matches the 64 lowercase hexadecimal characters that follow the
// algorithm prefix. Uppercase is deliberately rejected: the format is fixed so
// that digest strings compare byte-for-byte without normalization.
var digestHexRe = regexp.MustCompile(`^[0-9a-f]{64}$`)

// Digest computes the digest of a file's raw bytes in the extension's format,
// sha256:{hex} with 64 lowercase hexadecimal characters.
func Digest(content []byte) string {
	sum := sha256.Sum256(content)
	return DigestPrefix + hex.EncodeToString(sum[:])
}

// ValidateDigest checks that a digest string is well formed.
func ValidateDigest(digest string) error {
	h, ok := strings.CutPrefix(digest, DigestPrefix)
	if !ok {
		return fmt.Errorf("digest %q must begin with %q", digest, DigestPrefix)
	}
	if !digestHexRe.MatchString(h) {
		return fmt.Errorf("digest %q must be 64 lowercase hexadecimal characters", digest)
	}
	return nil
}

// Verify checks retrieved content against the size and digest an entry
// published for it.
//
// The size check is not an optimization: a read whose byte length differs from
// the entry's size is a verification failure equivalent to a digest mismatch,
// so it is reported as one whether or not the digest is then computed.
//
// A digest match proves only that listing and content are consistent. It is not
// a security boundary: both are supplied by the same server, and any
// intermediary can rewrite them together.
func Verify(content []byte, want EntryResource) error {
	if int64(len(content)) != want.Size {
		return fmt.Errorf("resource %q: size mismatch, read %d bytes but entry declares %d", want.URI, len(content), want.Size)
	}
	if got := Digest(content); got != want.Digest {
		return fmt.Errorf("resource %q: digest mismatch, computed %s but entry declares %s", want.URI, got, want.Digest)
	}
	return nil
}
