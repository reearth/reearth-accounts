package dregexp

import "regexp"

var EmailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

func IsEmailFormat(s string) bool {
	return EmailRegex.MatchString(s)
}
