package legal

import (
	"net/netip"
	"slices"
	"strings"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

var (
	ErrInvalidVersion = rerror.NewE(i18n.T("version must be MAJOR.MINOR.PATCH"))
	ErrNotAcceptable  = rerror.NewE(i18n.T("policy version is neither current nor upcoming"))
	ErrUnknownType    = rerror.NewE(i18n.T("unknown policy type"))
)

// Acceptance is one record of policy_acceptances: the evidence of consent and
// the consent state at once. (UserID, Type, Version) identifies it.
type Acceptance struct {
	Ref
	AcceptedAt time.Time
	Client     Client
	UserID     id.UserID
}

type Client struct {
	// IP is the connection address, or the client address that a trusted
	// proxy reported. The zero value means unknown.
	IP        netip.Addr
	Origin    string
	UserAgent string
}

type Overview struct {
	// Acceptances holds the Held acceptance of each type, sorted by type.
	Acceptances []Acceptance
	Pending     []VersionRecord
	Upcoming    []VersionRecord
}

type RefInput struct {
	Type    string
	Version string
}

func BuildOverview(c Catalog, as []Acceptance, now time.Time) Overview {
	ov := EmptyOverview()
	types := make([]Type, 0, len(as))
	for _, a := range as {
		types = append(types, a.Type)
	}
	slices.Sort(types)
	for _, t := range slices.Compact(types) {
		if h := Held(c, as, t); h != nil {
			ov.Acceptances = append(ov.Acceptances, *h)
		}
	}
	ov.Pending = Pending(c, as, now)
	ov.Upcoming = Upcoming(c, now)
	return ov
}

func EmptyOverview() Overview {
	return Overview{Acceptances: []Acceptance{}, Pending: []VersionRecord{}, Upcoming: []VersionRecord{}}
}

// Held returns the user's acceptance of t: the highest version among the
// records that are not withdrawn, or nil.
func Held(c Catalog, as []Acceptance, t Type) *Acceptance {
	var held *Acceptance
	for i, a := range as {
		if a.Type != t || c.Withdrawn(a.Ref) {
			continue
		}
		if held == nil || a.Version.Compare(held.Version) > 0 {
			held = &as[i]
		}
	}
	return held
}

// ParseRefs reports a malformed type as ErrUnknownType, since no published
// type can have that form.
func ParseRefs(in []RefInput) ([]Ref, error) {
	res := make([]Ref, len(in))
	for i, r := range in {
		t, err := ParseType(r.Type)
		if err != nil {
			return nil, ErrUnknownType
		}
		v, err := ParseVersion(r.Version)
		if err != nil {
			return nil, ErrInvalidVersion
		}
		res[i] = Ref{Type: t, Version: v}
	}
	return res, nil
}

// Pending lists, sorted by type, the current versions the user must accept:
// the current version is required, and the user holds no version of the type
// with a major number equal to or above it. A minor or patch change is never
// pending, and accepting the upcoming version counts for the current one.
func Pending(c Catalog, as []Acceptance, now time.Time) []VersionRecord {
	res := []VersionRecord{}
	for _, t := range c.Types() {
		cur := c.Current(t, now)
		if cur == nil || !cur.Required {
			continue
		}
		if h := Held(c, as, t); h == nil || h.Version.Major < cur.Version.Major {
			res = append(res, *cur)
		}
	}
	return res
}

// Pick returns the value for lang, then for its base language, then "en",
// then the value of the smallest key. It returns "" for an empty text.
func (t LocalizedText) Pick(lang string) string {
	if v, ok := t[lang]; ok {
		return v
	}
	if base, _, ok := strings.Cut(lang, "-"); ok {
		if v, ok := t[base]; ok {
			return v
		}
	}
	if v, ok := t["en"]; ok {
		return v
	}
	keys := make([]string, 0, len(t))
	for k := range t {
		keys = append(keys, k)
	}
	if len(keys) == 0 {
		return ""
	}
	return t[slices.Min(keys)]
}

// PlanAccept returns the refs to record when the user holding as accepts req.
// A ref at or below the version the user holds records nothing. It rejects
// the whole request when one ref names an unknown type (ErrUnknownType) or
// another version that is neither current nor upcoming (ErrNotAcceptable).
func PlanAccept(c Catalog, as []Acceptance, req []Ref, now time.Time) ([]Ref, error) {
	types := c.Types()
	res := []Ref{}
	for _, r := range req {
		if _, ok := slices.BinarySearch(types, r.Type); !ok {
			return nil, ErrUnknownType
		}
		if h := Held(c, as, r.Type); h != nil && h.Version.Compare(r.Version) >= 0 {
			continue
		}
		if !names(c.Current(r.Type, now), r.Version) && !names(c.Upcoming(r.Type, now), r.Version) {
			return nil, ErrNotAcceptable
		}
		res = append(res, r)
	}
	slices.SortFunc(res, func(a, b Ref) int {
		if a.Type != b.Type {
			return strings.Compare(string(a.Type), string(b.Type))
		}
		return a.Version.Compare(b.Version)
	})
	return slices.Compact(res), nil
}

// Upcoming lists the upcoming version of each type, sorted by type.
func Upcoming(c Catalog, now time.Time) []VersionRecord {
	res := []VersionRecord{}
	for _, t := range c.Types() {
		if u := c.Upcoming(t, now); u != nil {
			res = append(res, *u)
		}
	}
	return res
}

func names(rec *VersionRecord, v Version) bool {
	return rec != nil && rec.Version == v
}
