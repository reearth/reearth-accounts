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

func (n Notice) before(o Notice) bool {
	if !n.CreatedAt.Equal(o.CreatedAt) {
		return n.CreatedAt.Before(o.CreatedAt)
	}
	return n.ID.String() < o.ID.String()
}

// Overlap is what an earlier notice of the same version for the other
// audience leaves n to do. The outbox key stops a second mail only while the
// first record is kept, so both audiences are compared at fan-out instead.
// Only an earlier notice that is open or ended complete counts, because a
// withdrawn or covered one does not reach its whole audience.
type Overlap int

const (
	OverlapNone Overlap = iota
	// OverlapCovered is an owners notice after an all notice: every owner is
	// already in the earlier audience.
	OverlapCovered
	// OverlapSkipOwners is an all notice after an owners notice.
	OverlapSkipOwners
)

func OverlapOf(n Notice, notices []Notice) Overlap {
	for _, o := range notices {
		if o.Ref != n.Ref || o.Audience == n.Audience || !o.before(n) {
			continue
		}
		if o.Fanout.End != FanoutOpen && o.Fanout.End != FanoutComplete {
			continue
		}
		if n.Audience == AudienceOwners {
			return OverlapCovered
		}
		return OverlapSkipOwners
	}
	return OverlapNone
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
	End    FanoutEnd
	Lease  *Lease
}

func (f Fanout) Done() bool { return f.End != FanoutOpen }

type FanoutEnd string

const (
	FanoutOpen     FanoutEnd = ""
	FanoutComplete FanoutEnd = "complete"
	// FanoutCovered is a notice that an earlier notice of the other audience
	// covered. A repeated request reopens it, and the fan-out checks the
	// cover again, because the earlier notice may have been withdrawn since.
	FanoutCovered FanoutEnd = "covered"
	// FanoutWithdrawn is a notice whose version stopped being upcoming. A
	// repeated request reopens it once the version is upcoming again.
	FanoutWithdrawn FanoutEnd = "withdrawn"
)

// CreateOutcome is what NoticeRepo.Create did with a notice.
type CreateOutcome string

const (
	NoticeCreated  CreateOutcome = "created"
	NoticeExisting CreateOutcome = "existing"
	NoticeReopened CreateOutcome = "reopened"
)

type Lease struct {
	Token string
	Until time.Time
}

// NoticeRepo stores policy_notices. Each method is one conditional write.
type NoticeRepo interface {
	// AdvanceFanout stores cursor and end, and renews the lease until
	// lease.Until, while lease.Token holds the notice. An ended notice drops
	// its lease. It returns false when the token no longer holds.
	AdvanceFanout(ctx context.Context, n id.PolicyNoticeID, lease Lease, cursor string, end FanoutEnd) (bool, error)
	// ClaimFanout leases the oldest notice that is not done and whose lease
	// is absent or expired at now, and returns it. It returns nil when none
	// is due. Two callers never hold the same notice.
	ClaimFanout(ctx context.Context, now time.Time, lease Lease) (*Notice, error)
	// Create stores n unless a notice with its version and audience exists,
	// and returns the stored notice. A stored notice that ended withdrawn or
	// covered restarts with an empty cursor, because the caller checked that
	// its version is upcoming.
	Create(ctx context.Context, n Notice) (Notice, CreateOutcome, error)
	// FindByRef returns the notices of r, oldest first.
	FindByRef(ctx context.Context, r Ref) ([]Notice, error)
	// ReleaseFanout drops the lease while token holds it.
	ReleaseFanout(ctx context.Context, n id.PolicyNoticeID, token string) error
	// Withdraw ends n withdrawn and drops its lease when its fan-out is open
	// or complete, so that Create reopens it. A covered or withdrawn notice
	// stays as it is.
	Withdraw(ctx context.Context, n id.PolicyNoticeID) error
}
