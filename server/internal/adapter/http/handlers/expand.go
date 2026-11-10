package handlers

import (
	"fmt"
	"net/http"
	"slices"
	"strings"

	"github.com/labstack/echo/v4"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
)

// Expansion is an opt-in embed requested with ?expand=.
type Expansion string

const (
	ExpandMemberUsers Expansion = "members.user"
	ExpandMyWorkspace Expansion = "my_workspace"
)

type expansions map[Expansion]struct{}

func (e expansions) has(x Expansion) bool {
	_, ok := e[x]
	return ok
}

const expansionsKey = "rest.expansions"

// Expand parses ?expand= before the handler runs, so an unknown value fails a
// mutation before it changes anything.
func Expand(allowed ...Expansion) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			set, err := parseExpansions(c.QueryParams()["expand"], allowed)
			if err != nil {
				return err
			}
			c.Set(expansionsKey, set)
			return next(c)
		}
	}
}

func parseExpansions(raw []string, allowed []Expansion) (expansions, error) {
	set := expansions{}
	for _, r := range raw {
		for _, v := range strings.Split(r, ",") {
			x := Expansion(strings.TrimSpace(v))
			if !slices.Contains(allowed, x) {
				return nil, unknownExpansion(x, allowed)
			}
			set[x] = struct{}{}
		}
	}
	return set, nil
}

func unknownExpansion(x Expansion, allowed []Expansion) error {
	names := make([]string, len(allowed))
	for i, a := range allowed {
		names[i] = string(a)
	}
	return &httpinternal.ErrorResponse{
		Status:      http.StatusBadRequest,
		Message:     "validation failed",
		Description: fmt.Sprintf("unknown expand value %q", x),
		FieldErrors: []httpinternal.FieldErrorResponse{{
			Field:   "expand",
			Message: "must be one of: " + strings.Join(names, ", "),
		}},
	}
}

func requestedExpansions(c echo.Context) expansions {
	set, _ := c.Get(expansionsKey).(expansions)
	return set
}
