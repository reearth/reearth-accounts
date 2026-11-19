package maintenance

import (
	"crypto/sha256"
	"crypto/subtle"
)

// KeyHeader carries the maintenance key on POST /api/maintenance/run.
const KeyHeader = "X-Accounts-Maintenance-Key"

// Key guards the maintenance route. The zero value turns the route off.
type Key struct{ sum *[sha256.Size]byte }

func NewKey(s string) Key {
	if s == "" {
		return Key{}
	}
	sum := sha256.Sum256([]byte(s))
	return Key{sum: &sum}
}

func (k Key) Enabled() bool { return k.sum != nil }

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
