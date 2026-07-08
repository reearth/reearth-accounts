package user

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/pkg/id"
)

// GetUser godoc
//
//	@Summary		Get a user
//	@Description	Returns the detail of a single user by ID.
//	@Tags			users
//	@Produce		json
//	@Param			id	path		string	true	"User ID"
//	@Success		200	{object}	UserDetailResponse
//	@Failure		400	{object}	internal.ErrorResponse	"invalid id"
//	@Failure		401	{object}	internal.ErrorResponse	"unauthorized"
//	@Failure		403	{object}	internal.ErrorResponse	"not approved"
//	@Failure		404	{object}	internal.ErrorResponse	"not found"
//	@Router			/users/{id} [get]
func (h *Handler) GetUser(c echo.Context) error {
	uid, err := id.UserIDFrom(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid id")
	}

	u, err := h.getUC.Execute(c.Request().Context(), uid)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, newUserDetailResponse(u))
}
