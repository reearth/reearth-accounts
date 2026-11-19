package legal

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var parseNow = time.Date(2026, 11, 20, 3, 0, 0, 0, time.UTC)

const validPayload = `{
  "event": "policy_version_updated",
  "timestamp": "2026-11-20T03:00:00Z",
  "changes": [{"type": "tos", "from": "1.0.0", "to": "2.0.0"}],
  "repository": "example/policies",
  "versions": {
    "tos": {
      "version": "2.0.0",
      "effectiveDate": "2027-01-01T00:00:00.123456Z",
      "required": true,
      "title": {"en": "Terms of Service", "ja": "利用規約"},
      "url": {"en": "https://example.com/tos", "ja": "https://example.com/ja/tos"},
      "sha": "ignored"
    },
    "privacy": {
      "version": "1.4.2",
      "effectiveDate": "2026-01-01T09:00:00+09:00",
      "required": false,
      "title": {"en": "Privacy Policy"}
    }
  }
}`

func TestParseManifest_Valid(t *testing.T) {
	m, err := ParseManifest([]byte(validPayload), parseNow)
	require.NoError(t, err)

	assert.True(t, m.Timestamp.Equal(parseNow))
	require.Len(t, m.Entries, 2)

	privacy, tos := m.Entries[0], m.Entries[1]
	assert.Equal(t, Ref{Type: "privacy", Version: Version{1, 4, 2}}, privacy.Ref)
	assert.True(t, privacy.EffectiveAt.Equal(time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)))
	assert.False(t, privacy.Required)
	assert.Nil(t, privacy.URL, "url is optional")

	assert.Equal(t, Ref{Type: "tos", Version: Version{2, 0, 0}}, tos.Ref)
	assert.Equal(t, time.Date(2027, 1, 1, 0, 0, 0, 123000000, time.UTC), tos.EffectiveAt, "truncated to ms")
	assert.True(t, tos.Required)
	assert.Equal(t, LocalizedText{"en": "Terms of Service", "ja": "利用規約"}, tos.Title)
	assert.Equal(t, "https://example.com/ja/tos", tos.URL["ja"])
}

func TestParseManifest_EmptyVersions(t *testing.T) {
	m, err := ParseManifest([]byte(`{"event":"policy_version_updated","timestamp":"2026-11-20T03:00:00Z","versions":{}}`), parseNow)
	require.NoError(t, err)
	assert.Empty(t, m.Entries)
}

func TestParseManifest_Rejects(t *testing.T) {
	tos := func(fields string) string {
		return `{"event":"policy_version_updated","timestamp":"2026-11-20T03:00:00Z","versions":{"tos":{` + fields + `}}}`
	}
	const ok = `"version":"2.0.0","effectiveDate":"2027-01-01T00:00:00Z","required":true,"title":{"en":"ToS"}`
	tests := []struct {
		name  string
		body  string
		field string
	}{
		{"not json", `{`, "body"},
		{"not an object", `[]`, "body"},
		{"missing event", `{"timestamp":"2026-11-20T03:00:00Z","versions":{}}`, "event"},
		{"wrong event", `{"event":"push","timestamp":"2026-11-20T03:00:00Z","versions":{}}`, "event"},
		{"missing timestamp", `{"event":"policy_version_updated","versions":{}}`, "timestamp"},
		{"timestamp not RFC 3339", `{"event":"policy_version_updated","timestamp":"2026-11-20 03:00:00","versions":{}}`, "timestamp"},
		{"timestamp too far ahead", `{"event":"policy_version_updated","timestamp":"2026-11-20T03:05:01Z","versions":{}}`, "timestamp"},
		{"missing versions", `{"event":"policy_version_updated","timestamp":"2026-11-20T03:00:00Z"}`, "versions"},
		{"null versions", `{"event":"policy_version_updated","timestamp":"2026-11-20T03:00:00Z","versions":null}`, "versions"},
		{"versions not a map", `{"event":"policy_version_updated","timestamp":"2026-11-20T03:00:00Z","versions":[]}`, "body"},
		{"bad type key", `{"event":"policy_version_updated","timestamp":"2026-11-20T03:00:00Z","versions":{"to:s":{` + ok + `}}}`, "versions.to:s"},
		{"upper-case type key", `{"event":"policy_version_updated","timestamp":"2026-11-20T03:00:00Z","versions":{"ToS":{` + ok + `}}}`, "versions.ToS"},
		{"type key too long", `{"event":"policy_version_updated","timestamp":"2026-11-20T03:00:00Z","versions":{"` + strings.Repeat("a", 65) + `":{` + ok + `}}}`, "versions." + strings.Repeat("a", 65)},
		{"entry null", `{"event":"policy_version_updated","timestamp":"2026-11-20T03:00:00Z","versions":{"tos":null}}`, "versions.tos.version"},
		{"missing version", tos(`"effectiveDate":"2027-01-01T00:00:00Z","required":true,"title":{"en":"ToS"}`), "versions.tos.version"},
		{"bad version", tos(`"version":"2.0","effectiveDate":"2027-01-01T00:00:00Z","required":true,"title":{"en":"ToS"}`), "versions.tos.version"},
		{"prerelease version", tos(`"version":"2.0.0-rc1","effectiveDate":"2027-01-01T00:00:00Z","required":true,"title":{"en":"ToS"}`), "versions.tos.version"},
		{"version not a string", tos(`"version":2,"effectiveDate":"2027-01-01T00:00:00Z","required":true,"title":{"en":"ToS"}`), "versions.tos"},
		{"missing effectiveDate", tos(`"version":"2.0.0","required":true,"title":{"en":"ToS"}`), "versions.tos.effectiveDate"},
		{"bad effectiveDate", tos(`"version":"2.0.0","effectiveDate":"2027-01-01","required":true,"title":{"en":"ToS"}`), "versions.tos.effectiveDate"},
		{"missing required", tos(`"version":"2.0.0","effectiveDate":"2027-01-01T00:00:00Z","title":{"en":"ToS"}`), "versions.tos.required"},
		{"required not a bool", tos(`"version":"2.0.0","effectiveDate":"2027-01-01T00:00:00Z","required":"yes","title":{"en":"ToS"}`), "versions.tos"},
		{"missing title", tos(`"version":"2.0.0","effectiveDate":"2027-01-01T00:00:00Z","required":true`), "versions.tos.title"},
		{"title not a map", tos(`"version":"2.0.0","effectiveDate":"2027-01-01T00:00:00Z","required":true,"title":"ToS"`), "versions.tos"},
		{"url not a map", tos(ok + `,"url":"https://example.com"`), "versions.tos"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseManifest([]byte(tt.body), parseNow)
			var pe *PayloadError
			require.True(t, errors.As(err, &pe), "got %v", err)
			assert.Equal(t, tt.field, pe.Field)
		})
	}
}

func TestParseManifest_OneBadTypeRejectsTheWholePayload(t *testing.T) {
	body := `{"event":"policy_version_updated","timestamp":"2026-11-20T03:00:00Z","versions":{
		"privacy":{"version":"1.0.0","effectiveDate":"2026-01-01T00:00:00Z","required":true,"title":{"en":"P"}},
		"tos":{"version":"2.0.0","effectiveDate":"2027-01-01T00:00:00Z","title":{"en":"ToS"}}}}`
	m, err := ParseManifest([]byte(body), parseNow)
	var pe *PayloadError
	require.True(t, errors.As(err, &pe))
	assert.Empty(t, m.Entries)
}

func TestParseManifest_TimestampAtTheSkewLimit(t *testing.T) {
	_, err := ParseManifest([]byte(`{"event":"policy_version_updated","timestamp":"2026-11-20T03:05:00Z","versions":{}}`), parseNow)
	assert.NoError(t, err)
}

func TestSecret_Verify(t *testing.T) {
	s := NewSecret("webhook-secret")
	body := []byte(validPayload)
	good := s.Sign(body)

	assert.NoError(t, s.Verify(body, good))
	assert.NoError(t, s.Verify(body, "sha256="+strings.ToUpper(strings.TrimPrefix(good, "sha256="))))

	for name, header := range map[string]string{
		"missing":       "",
		"none":          "none",
		"no prefix":     strings.TrimPrefix(good, "sha256="),
		"sha1 prefix":   "sha1=" + strings.TrimPrefix(good, "sha256="),
		"not hex":       "sha256=zz",
		"truncated":     good[:len(good)-2],
		"other secret":  NewSecret("other").Sign(body),
		"other body":    s.Sign(append(bytes.Clone(body), ' ')),
		"empty hex":     "sha256=",
		"padded header": " " + good,
	} {
		t.Run(name, func(t *testing.T) {
			assert.ErrorIs(t, s.Verify(body, header), ErrBadSignature)
		})
	}
}

func TestSecret(t *testing.T) {
	assert.False(t, NewSecret("").Enabled())
	assert.True(t, NewSecret("x").Enabled())
	assert.Equal(t, "***", NewSecret("webhook-secret").String())
}

func TestReadWebhookBody(t *testing.T) {
	b, err := ReadWebhookBody(bytes.NewReader(make([]byte, MaxWebhookBody)))
	require.NoError(t, err)
	assert.Len(t, b, MaxWebhookBody)

	_, err = ReadWebhookBody(bytes.NewReader(make([]byte, MaxWebhookBody+1)))
	assert.ErrorIs(t, err, ErrPayloadTooLarge)
}

func TestParseVersion(t *testing.T) {
	v, err := ParseVersion("10.2.33")
	require.NoError(t, err)
	assert.Equal(t, Version{10, 2, 33}, v)
	assert.Equal(t, "10.2.33", v.String())
	assert.Equal(t, -1, MustParseVersion("1.9.9").Compare(MustParseVersion("2.0.0")))
	assert.Equal(t, 1, MustParseVersion("1.10.0").Compare(MustParseVersion("1.9.0")))
	assert.Equal(t, 0, MustParseVersion("1.0.0").Compare(MustParseVersion("1.0.0")))
	for _, bad := range []string{"", "1", "1.0", "v1.0.0", "1.0.0.0", " 1.0.0", "1.0.0\n", "-1.0.0", "99999999999999999999.0.0"} {
		_, err := ParseVersion(bad)
		assert.Error(t, err, bad)
	}
}
