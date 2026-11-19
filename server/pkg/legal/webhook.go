package legal

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

const (
	SignatureHeader = "X-Policy-Signature"
	MaxWebhookBody  = 1 << 20
	MaxFutureSkew   = 5 * time.Minute
	WebhookEvent    = "policy_version_updated"

	signaturePrefix = "sha256="
)

var (
	ErrNotConfigured   = rerror.NewE(i18n.T("policies not configured"))
	ErrBadSignature    = errors.New("legal: bad webhook signature")
	ErrPayloadTooLarge = errors.New("legal: webhook payload larger than 1 MiB")
)

// PayloadError rejects the whole payload: nothing from it is saved.
type PayloadError struct{ Field, Reason string }

func (e *PayloadError) Error() string {
	return fmt.Sprintf("invalid policy webhook payload: %s: %s", e.Field, e.Reason)
}

// Secret is the webhook HMAC key. The zero value turns the feature off.
type Secret struct{ b []byte }

func NewSecret(s string) Secret { return Secret{b: []byte(s)} }

func (s Secret) Enabled() bool { return len(s.b) > 0 }

func (s Secret) String() string { return "***" }

// ParseSignature checks that header is "sha256=" followed by a hex
// HMAC-SHA256, so that a missing or "none" signature is refused before the
// body is read.
func ParseSignature(header string) ([]byte, error) {
	hexSum, ok := strings.CutPrefix(header, signaturePrefix)
	if !ok {
		return nil, ErrBadSignature
	}
	sum, err := hex.DecodeString(hexSum)
	if err != nil || len(sum) != sha256.Size {
		return nil, ErrBadSignature
	}
	return sum, nil
}

// Verify checks header against the HMAC-SHA256 of body keyed with s,
// comparing in constant time.
func (s Secret) Verify(body []byte, header string) error {
	got, err := ParseSignature(header)
	if err != nil {
		return err
	}
	mac := hmac.New(sha256.New, s.b)
	mac.Write(body)
	if !hmac.Equal(got, mac.Sum(nil)) {
		return ErrBadSignature
	}
	return nil
}

// Sign returns the header value Verify accepts for body.
func (s Secret) Sign(body []byte) string {
	mac := hmac.New(sha256.New, s.b)
	mac.Write(body)
	return signaturePrefix + hex.EncodeToString(mac.Sum(nil))
}

// ReadWebhookBody reads at most MaxWebhookBody bytes from r.
func ReadWebhookBody(r io.Reader) ([]byte, error) {
	body, err := io.ReadAll(io.LimitReader(r, MaxWebhookBody+1))
	if err != nil {
		return nil, err
	}
	if len(body) > MaxWebhookBody {
		return nil, ErrPayloadTooLarge
	}
	return body, nil
}

// Entry is one type's entry in versions.json.
type Entry struct {
	Ref
	EffectiveAt time.Time
	Required    bool
	Title       LocalizedText
	URL         LocalizedText
}

// Manifest is a parsed webhook payload. Entries are sorted by type.
type Manifest struct {
	Timestamp time.Time
	Entries   []Entry
}

type wirePayload struct {
	Event     *string                    `json:"event"`
	Timestamp *string                    `json:"timestamp"`
	Versions  map[string]json.RawMessage `json:"versions"`
}

type wireEntry struct {
	Version       *string           `json:"version"`
	EffectiveDate *string           `json:"effectiveDate"`
	Required      *bool             `json:"required"`
	Title         map[string]string `json:"title"`
	URL           map[string]string `json:"url"`
}

// ParseManifest validates every field before returning, so a payload with
// any missing or malformed required field is rejected whole. Times are
// truncated to milliseconds, the precision every backend stores.
func ParseManifest(body []byte, now time.Time) (Manifest, error) {
	if !utf8.Valid(body) {
		return Manifest{}, &PayloadError{Field: "body", Reason: "not valid UTF-8"}
	}
	var p wirePayload
	if err := json.Unmarshal(body, &p); err != nil {
		return Manifest{}, &PayloadError{Field: "body", Reason: err.Error()}
	}
	if p.Event == nil {
		return Manifest{}, &PayloadError{Field: "event", Reason: "missing"}
	}
	if *p.Event != WebhookEvent {
		return Manifest{}, &PayloadError{Field: "event", Reason: fmt.Sprintf("must be %q", WebhookEvent)}
	}
	ts, err := parseTime("timestamp", p.Timestamp)
	if err != nil {
		return Manifest{}, err
	}
	if ts.After(now.Add(MaxFutureSkew)) {
		return Manifest{}, &PayloadError{Field: "timestamp", Reason: "more than 5 minutes in the future"}
	}
	if p.Versions == nil {
		return Manifest{}, &PayloadError{Field: "versions", Reason: "missing"}
	}

	entries := make([]Entry, 0, len(p.Versions))
	for key, raw := range p.Versions {
		e, err := parseEntry(key, raw)
		if err != nil {
			return Manifest{}, err
		}
		entries = append(entries, e)
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].Type < entries[j].Type })
	return Manifest{Timestamp: ts, Entries: entries}, nil
}

func parseEntry(key string, raw json.RawMessage) (Entry, error) {
	field := "versions." + key
	t, err := ParseType(key)
	if err != nil {
		return Entry{}, &PayloadError{Field: field, Reason: err.Error()}
	}
	var w wireEntry
	if err := json.Unmarshal(raw, &w); err != nil {
		return Entry{}, &PayloadError{Field: field, Reason: err.Error()}
	}
	if w.Version == nil {
		return Entry{}, &PayloadError{Field: field + ".version", Reason: "missing"}
	}
	v, err := ParseVersion(*w.Version)
	if err != nil {
		return Entry{}, &PayloadError{Field: field + ".version", Reason: err.Error()}
	}
	eff, err := parseTime(field+".effectiveDate", w.EffectiveDate)
	if err != nil {
		return Entry{}, err
	}
	if w.Required == nil {
		return Entry{}, &PayloadError{Field: field + ".required", Reason: "missing"}
	}
	if w.Title == nil {
		return Entry{}, &PayloadError{Field: field + ".title", Reason: "missing"}
	}
	if hasNUL(w.Title) {
		return Entry{}, &PayloadError{Field: field + ".title", Reason: "contains NUL"}
	}
	if hasNUL(w.URL) {
		return Entry{}, &PayloadError{Field: field + ".url", Reason: "contains NUL"}
	}
	return Entry{
		Ref:         Ref{Type: t, Version: v},
		EffectiveAt: eff,
		Required:    *w.Required,
		Title:       w.Title,
		URL:         w.URL,
	}, nil
}

// hasNUL reports a NUL in a key or value, which no store accepts in text.
func hasNUL(m map[string]string) bool {
	for k, v := range m {
		if strings.ContainsRune(k, 0) || strings.ContainsRune(v, 0) {
			return true
		}
	}
	return false
}

func parseTime(field string, s *string) (time.Time, error) {
	if s == nil {
		return time.Time{}, &PayloadError{Field: field, Reason: "missing"}
	}
	t, err := time.Parse(time.RFC3339, *s)
	if err != nil {
		return time.Time{}, &PayloadError{Field: field, Reason: "not RFC 3339"}
	}
	return t.UTC().Truncate(time.Millisecond), nil
}
