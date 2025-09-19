package migration

import (
	"regexp"
	"strings"
)

func sanitizeAlias(alias string) string {
	alias = regexp.MustCompile(`[^a-zA-Z0-9-]`).ReplaceAllString(alias, "-")
	alias = regexp.MustCompile(`-+`).ReplaceAllString(alias, "-")
	alias = strings.Trim(alias, "-")

	if len(alias) < 5 {
		alias = alias + strings.Repeat("a", 5-len(alias))
	}

	if len(alias) > 30 {
		alias = alias[:30]
		alias = strings.TrimRight(alias, "-")
		if len(alias) < 5 {
			alias = alias + strings.Repeat("a", 5-len(alias))
		}
	}

	return alias
}
