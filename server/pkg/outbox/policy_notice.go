package outbox

import (
	"fmt"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
)

// SubjectNotice is the subject field that names a policy notice, which the
// stores count records by.
const SubjectNotice = "notice"

// PolicyNoticeKey is one key per user and version, so a user gets one notice
// of a version whichever notice queued it.
func PolicyNoticeKey(r legal.Ref, u id.UserID) Key {
	return Key(fmt.Sprintf("policy:%s:%s:%s", r.Type, r.Version, u))
}

type PolicyNoticeSubject struct {
	Bump   legal.Bump
	Notice id.PolicyNoticeID
	Ref    legal.Ref
}

func ParsePolicyNoticeSubject(s Subject) (PolicyNoticeSubject, error) {
	n, err := id.PolicyNoticeIDFrom(s[SubjectNotice])
	if err != nil {
		return PolicyNoticeSubject{}, fmt.Errorf("policy notice subject: %w", err)
	}
	t, err := legal.ParseType(s["type"])
	if err != nil {
		return PolicyNoticeSubject{}, fmt.Errorf("policy notice subject: %w", err)
	}
	v, err := legal.ParseVersion(s["version"])
	if err != nil {
		return PolicyNoticeSubject{}, fmt.Errorf("policy notice subject: %w", err)
	}
	b := legal.Bump(s["bump"])
	if b != legal.BumpMajor && b != legal.BumpMinor {
		return PolicyNoticeSubject{}, fmt.Errorf("policy notice subject: bump %q", b)
	}
	return PolicyNoticeSubject{Bump: b, Notice: n, Ref: legal.Ref{Type: t, Version: v}}, nil
}

func (p PolicyNoticeSubject) Subject() Subject {
	return Subject{
		"bump":        string(p.Bump),
		SubjectNotice: p.Notice.String(),
		"type":        string(p.Ref.Type),
		"version":     p.Ref.Version.String(),
	}
}
