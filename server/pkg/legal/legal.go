// Package legal holds legal policy versions (terms of service, privacy
// policy). Workspace plan policies live in pkg/policy.
package legal

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

var (
	typePattern    = regexp.MustCompile(`^[a-z0-9_-]{1,64}$`)
	versionPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+\.[0-9]+$`)
)

// Type is a policy type key. It never contains ':' because mail keys embed it.
type Type string

func ParseType(s string) (Type, error) {
	if !typePattern.MatchString(s) {
		return "", fmt.Errorf("type %q does not match %s", s, typePattern)
	}
	return Type(s), nil
}

type Version struct{ Major, Minor, Patch uint64 }

func ParseVersion(s string) (Version, error) {
	if !versionPattern.MatchString(s) {
		return Version{}, fmt.Errorf("version %q does not match %s", s, versionPattern)
	}
	parts := strings.Split(s, ".")
	var n [3]uint64
	for i, p := range parts {
		v, err := strconv.ParseUint(p, 10, 64)
		if err != nil {
			return Version{}, fmt.Errorf("version %q: %w", s, err)
		}
		n[i] = v
	}
	return Version{Major: n[0], Minor: n[1], Patch: n[2]}, nil
}

func MustParseVersion(s string) Version {
	v, err := ParseVersion(s)
	if err != nil {
		panic(err)
	}
	return v
}

func (v Version) String() string {
	return fmt.Sprintf("%d.%d.%d", v.Major, v.Minor, v.Patch)
}

func (v Version) Compare(o Version) int {
	for _, d := range [][2]uint64{{v.Major, o.Major}, {v.Minor, o.Minor}, {v.Patch, o.Patch}} {
		switch {
		case d[0] < d[1]:
			return -1
		case d[0] > d[1]:
			return 1
		}
	}
	return 0
}

// Ref names one version of one type.
type Ref struct {
	Type    Type
	Version Version
}

// LocalizedText maps a locale to a value, as in {"en": ..., "ja": ...}.
type LocalizedText map[string]string
