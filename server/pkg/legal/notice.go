package legal

import (
	"context"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

var (
	ErrInvalidAudience = rerror.NewE(i18n.T("audience must be all or owners"))
	ErrNoUpcoming      = rerror.NewE(i18n.T("policy type has no upcoming version"))
	ErrNotRequired     = rerror.NewE(i18n.T("policy type is not required"))
	ErrPatchNotice     = rerror.NewE(i18n.T("a patch version needs no notice"))
)

type Audience string

const (
	// AudienceAll is every active user with an email address, verified or not.
	AudienceAll Audience = "all"
	// AudienceOwners is the owners of non-personal workspaces.
	AudienceOwners Audience = "owners"
)

type Bump string

const (
	BumpMajor Bump = "major"
	BumpMinor Bump = "minor"
	BumpPatch Bump = "patch"
)

// BumpOf names the first component in which v differs from the current
// version. With no current version every change is major.
func BumpOf(current *VersionRecord, v Version) Bump {
	switch {
	case current == nil || current.Version.Major != v.Major:
		return BumpMajor
	case current.Version.Minor != v.Minor:
		return BumpMinor
	default:
		return BumpPatch
	}
}

// CheckNoticeable returns the upcoming version of t that a notice announces,
// and how it changes the current one.
func CheckNoticeable(c Catalog, t Type, now time.Time) (VersionRecord, Bump, error) {
	up := c.Upcoming(t, now)
	if up == nil {
		return VersionRecord{}, "", ErrNoUpcoming
	}
	if !up.Required {
		return VersionRecord{}, "", ErrNotRequired
	}
	b := BumpOf(c.Current(t, now), up.Version)
	if b == BumpPatch {
		return VersionRecord{}, "", ErrPatchNotice
	}
	return *up, b, nil
}

// HoldsAtLeast reports whether as, the records of one user, hold r or a later
// version of its type.
func HoldsAtLeast(c Catalog, as []Acceptance, r Ref) bool {
	h := Held(c, as, r.Type)
	return h != nil && h.Version.Compare(r.Version) >= 0
}

// Notice is one staff request to mail the audience about an upcoming
// version. A notice is unique on its version and audience.
type Notice struct {
	Audience    Audience
	Bump        Bump
	CreatedAt   time.Time
	Fanout      Fanout
	ID          id.PolicyNoticeID
	Ref         Ref
	RequestedBy id.AdminUserID
}

func NewNotice(c Catalog, t Type, a Audience, by id.AdminUserID, now time.Time) (Notice, error) {
	up, b, err := CheckNoticeable(c, t, now)
	if err != nil {
		return Notice{}, err
	}
	return Notice{Audience: a, Bump: b, CreatedAt: now, ID: id.NewPolicyNoticeID(), Ref: up.Ref, RequestedBy: by}, nil
}

func ParseAudience(s string) (Audience, error) {
	switch a := Audience(s); a {
	case AudienceAll, AudienceOwners:
		return a, nil
	}
	return "", ErrInvalidAudience
}

// Fanout is how far the passes have queued the notice's audience. Cursor is
// the last user or workspace ID paged.
type Fanout struct {
	Cursor string
	Done   bool
	Lease  *Lease
}

type Lease struct {
	Token string
	Until time.Time
}

// NoticeRepo stores policy_notices. Each method is one conditional write.
type NoticeRepo interface {
	// AdvanceFanout stores cursor and done, and renews the lease until
	// lease.Until, while lease.Token holds the notice. A done notice drops
	// its lease. It returns false when the token no longer holds.
	AdvanceFanout(ctx context.Context, n id.PolicyNoticeID, lease Lease, cursor string, done bool) (bool, error)
	// ClaimFanout leases the oldest notice that is not done and whose lease
	// is absent or expired at now, and returns it. It returns nil when none
	// is due. Two callers never hold the same notice.
	ClaimFanout(ctx context.Context, now time.Time, lease Lease) (*Notice, error)
	// Create stores n unless a notice with its version and audience exists,
	// and returns the stored notice and whether n was stored.
	Create(ctx context.Context, n Notice) (Notice, bool, error)
	// FindByRef returns the notices of r, oldest first.
	FindByRef(ctx context.Context, r Ref) ([]Notice, error)
	// ReleaseFanout drops the lease while token holds it.
	ReleaseFanout(ctx context.Context, n id.PolicyNoticeID, token string) error
}
