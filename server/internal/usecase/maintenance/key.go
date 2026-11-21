package maintenance

import (
	"crypto/sha256"
	"crypto/subtle"
)

const KeyHeader = "X-Accounts-Maintenance-Key"

const MinKeyLen = 32

type Key struct{ sum *[sha256.Size]byte }

func NewKey(s string) Key {
	if s == "" {
		return Key{}
	}
	sum := sha256.Sum256([]byte(s))
	return Key{sum: &sum}
}

// Matches compares digests, so the time taken reveals neither the key nor
// its length.
func (k Key) Matches(header string) bool {
	if k.sum == nil {
		return false
	}
	got := sha256.Sum256([]byte(header))
	return subtle.ConstantTimeCompare(got[:], k.sum[:]) == 1
}

func (Key) String() string { return "***" }
