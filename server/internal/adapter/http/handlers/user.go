package handlers

import (
	"net/http"
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/http/httpmodel"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearthx/appx"
	"github.com/reearth/reearthx/rerror"
	"github.com/samber/lo"
)

type UserHandler struct{}

func NewUserHandler() *UserHandler { return &UserHandler{} }

// Me godoc
// @Tags User
// @Summary Get current user
// @Security BearerAuth
// @Produce json
// @Success 200 {object} httpmodel.MeResponse
// @Failure 401 {object} internal.ErrorResponse
// @Router /api/users/me [get]
func (h *UserHandler) Me(c echo.Context) error {
	u, err := httpinternal.RequireUser(c)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewMeResponse(u))
}

// UpdateMe godoc
// @Tags User
// @Summary Update current user
// @Security BearerAuth
// @Accept json
// @Produce json
// @Param body body httpmodel.UpdateMeRequest true "fields to update"
// @Success 200 {object} httpmodel.MeResponse
// @Failure 400 {object} internal.ErrorResponse
// @Failure 401 {object} internal.ErrorResponse
// @Router /api/users/me [patch]
func (h *UserHandler) UpdateMe(c echo.Context) error {
	ctx := c.Request().Context()
	req := &httpmodel.UpdateMeRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	u, err := httpinternal.Usecases(c).User.UpdateMe(ctx, req.ToInteractorInput(), httpinternal.Operator(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewMeResponse(u))
}

// DeleteMe godoc
// @Tags User
// @Summary Delete current user
// @Security BearerAuth
// @Produce json
// @Success 204
// @Failure 401 {object} internal.ErrorResponse
// @Router /api/users/me [delete]
func (h *UserHandler) DeleteMe(c echo.Context) error {
	ctx := c.Request().Context()
	u, err := httpinternal.RequireUser(c)
	if err != nil {
		return err
	}
	if err := httpinternal.Usecases(c).User.DeleteMe(ctx, u.ID(), httpinternal.Operator(c)); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// RemoveMyAuth godoc
// @Tags User
// @Summary Remove an auth provider from the current user
// @Security BearerAuth
// @Param sub path string true "auth provider/sub"
// @Produce json
// @Success 200 {object} httpmodel.MeResponse
// @Router /api/users/me/auths/{sub} [delete]
func (h *UserHandler) RemoveMyAuth(c echo.Context) error {
	ctx := c.Request().Context()
	sub := c.Param("sub")
	u, err := httpinternal.Usecases(c).User.RemoveMyAuth(ctx, sub, httpinternal.Operator(c))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewMeResponse(u))
}

// Get godoc
// @Tags User
// @Summary Get a user by ID
// @Security BearerAuth
// @Param id path string true "user ID"
// @Produce json
// @Success 200 {object} httpmodel.UserResponse
// @Failure 404 {object} internal.ErrorResponse
// @Router /api/users/{id} [get]
func (h *UserHandler) Get(c echo.Context) error {
	ctx := c.Request().Context()
	uid, err := id.UserIDFrom(c.Param("id"))
	if err != nil {
		return httpinternal.NewError(http.StatusBadRequest, "invalid user id", nil)
	}
	res, err := httpinternal.Usecases(c).User.FetchByID(ctx, id.UserIDList{uid})
	if err != nil {
		return err
	}
	if len(res) == 0 {
		return rerror.ErrNotFound
	}
	return c.JSON(http.StatusOK, httpmodel.NewUserResponse(res[0]))
}

// List godoc
// @Tags User
// @Summary List users by IDs (optionally paginated with alias filter)
// @Description Default form returns an array of users. When page, page_size, or alias is supplied, a paginated object is returned instead: {"items": [user...], "pagination": {"page", "page_size", "total"}}.
// @Security BearerAuth
// @Param ids query string false "comma-separated user IDs"
// @Param alias query string false "alias filter (requires pagination)"
// @Param page query int false "page (default 1)"
// @Param page_size query int false "page size (default 50, max 100)"
// @Produce json
// @Success 200 {array} httpmodel.UserResponse "array form; the paginated form returns an object wrapper (see description)"
// @Router /api/users [get]
func (h *UserHandler) List(c echo.Context) error {
	ctx := c.Request().Context()
	uc := httpinternal.Usecases(c).User

	idsParam := c.QueryParam("ids")
	var ids id.UserIDList
	if idsParam != "" {
		parsed, err := httpmodel.ParseUserIDs(strings.Split(idsParam, ","))
		if err != nil {
			return httpinternal.NewError(http.StatusBadRequest, "invalid user id in ids", nil)
		}
		ids = parsed
	}

	// Paginated form when page/page_size/alias supplied.
	if c.QueryParam("page") != "" || c.QueryParam("page_size") != "" || c.QueryParam("alias") != "" {
		var pp httpinternal.PageParams
		if err := c.Bind(&pp); err != nil {
			return err
		}
		page, size := pp.Normalized()
		var alias *string
		if a := c.QueryParam("alias"); a != "" {
			alias = &a
		}
		res, err := uc.FetchByIDsWithPagination(ctx, ids, alias, interfaces.FetchByIDsWithPaginationParam{Page: int64(page), Size: int64(size)})
		if err != nil {
			return err
		}
		return c.JSON(http.StatusOK, httpinternal.NewPageResult(httpmodel.NewUserResponses(res.Users), page, size, res.TotalCount))
	}

	res, err := uc.FetchByID(ctx, ids)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewUserResponses(res))
}

// Search godoc
// @Tags User
// @Summary Search users by keyword
// @Security BearerAuth
// @Param keyword query string true "search keyword"
// @Produce json
// @Success 200 {array} httpmodel.UserResponse
// @Router /api/users/search [get]
func (h *UserHandler) Search(c echo.Context) error {
	ctx := c.Request().Context()
	keyword := c.QueryParam("keyword")
	res, err := httpinternal.Usecases(c).User.SearchUser(ctx, keyword)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewUserResponses(res))
}

// FindByAlias godoc
// @Tags User
// @Summary Find a user by alias
// @Security BearerAuth
// @Param alias query string true "alias"
// @Produce json
// @Success 200 {object} httpmodel.UserResponse
// @Failure 404 {object} internal.ErrorResponse
// @Router /api/users/by-alias [get]
func (h *UserHandler) FindByAlias(c echo.Context) error {
	ctx := c.Request().Context()
	res, err := httpinternal.Usecases(c).User.FetchByAlias(ctx, c.QueryParam("alias"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewUserResponse(res))
}

// FindByNameOrEmail godoc
// @Tags User
// @Summary Find a user by name or email
// @Security BearerAuth
// @Param q query string true "name or email"
// @Produce json
// @Success 200 {object} httpmodel.SimpleUserResponse
// @Failure 404 {object} internal.ErrorResponse
// @Router /api/users/by-name-or-email [get]
func (h *UserHandler) FindByNameOrEmail(c echo.Context) error {
	ctx := c.Request().Context()
	res, err := httpinternal.Usecases(c).User.FetchByNameOrEmail(ctx, c.QueryParam("q"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewSimpleUserResponse(res))
}

// FindByNameOrAlias godoc
// @Tags User
// @Summary Find users by name or alias
// @Security BearerAuth
// @Param q query string true "name or alias"
// @Produce json
// @Success 200 {array} httpmodel.UserResponse
// @Router /api/users/by-name-or-alias [get]
func (h *UserHandler) FindByNameOrAlias(c echo.Context) error {
	ctx := c.Request().Context()
	res, err := httpinternal.Usecases(c).User.FetchByNameOrAlias(ctx, c.QueryParam("q"))
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewUserResponses(res))
}

// Signup godoc
// @Tags User
// @Summary Sign up a new user
// @Accept json
// @Produce json
// @Param body body httpmodel.SignupRequest true "signup fields"
// @Success 200 {object} httpmodel.UserResponse
// @Failure 400 {object} internal.ErrorResponse
// @Failure 409 {object} internal.ErrorResponse
// @Router /api/users/signup [post]
func (h *UserHandler) Signup(c echo.Context) error {
	ctx := c.Request().Context()
	req := &httpmodel.SignupRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	uid, err := parseUserIDRef(req.ID)
	if err != nil {
		return httpinternal.NewError(http.StatusBadRequest, "invalid id", nil)
	}
	wid, err := parseWorkspaceIDRef(req.WorkspaceID)
	if err != nil {
		return httpinternal.NewError(http.StatusBadRequest, "invalid workspace_id", nil)
	}
	param := interfaces.SignupParam{
		Email:       req.Email,
		Name:        req.Name,
		Password:    req.Password,
		Secret:      req.Secret,
		Lang:        httpmodel.ParseLang(req.Lang),
		Theme:       httpmodel.ParseTheme(req.Theme),
		UserID:      uid,
		WorkspaceID: wid,
		MockAuth:    lo.FromPtr(req.MockAuth),
	}
	u, err := httpinternal.Usecases(c).User.Signup(ctx, param)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewUserResponse(u))
}

// SignupOIDC godoc
// @Tags User
// @Summary Sign up via OIDC
// @Accept json
// @Produce json
// @Param body body httpmodel.SignupOIDCRequest true "OIDC signup fields"
// @Success 200 {object} httpmodel.UserResponse
// @Router /api/users/signup-oidc [post]
func (h *UserHandler) SignupOIDC(c echo.Context) error {
	ctx := c.Request().Context()
	req := &httpmodel.SignupOIDCRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	ai := adapterAuthInfo(c)
	accessToken, iss := "", ""
	if ai != nil {
		accessToken, iss = ai.Token, ai.Iss
	}
	uid, err := parseUserIDRef(req.ID)
	if err != nil {
		return httpinternal.NewError(http.StatusBadRequest, "invalid id", nil)
	}
	wid, err := parseWorkspaceIDRef(req.WorkspaceID)
	if err != nil {
		return httpinternal.NewError(http.StatusBadRequest, "invalid workspace_id", nil)
	}
	param := interfaces.SignupOIDCParam{
		Sub:         lo.FromPtr(req.Sub),
		AccessToken: accessToken,
		Issuer:      iss,
		Email:       lo.FromPtr(req.Email),
		Name:        lo.FromPtr(req.Name),
		Secret:      req.Secret,
		User: interfaces.SignupUserParam{
			Lang:        httpmodel.ParseLang(req.Lang),
			UserID:      uid,
			WorkspaceID: wid,
		},
	}
	u, err := httpinternal.Usecases(c).User.SignupOIDC(ctx, param)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewUserResponse(u))
}

// CreateVerification godoc
// @Tags User
// @Summary Create an email verification
// @Accept json
// @Produce json
// @Param body body httpmodel.CreateVerificationRequest true "email"
// @Success 200 {object} httpmodel.MessageResponse
// @Router /api/users/verifications [post]
func (h *UserHandler) CreateVerification(c echo.Context) error {
	ctx := c.Request().Context()
	req := &httpmodel.CreateVerificationRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	if err := httpinternal.Usecases(c).User.CreateVerification(ctx, req.Email); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.MessageResponse{Success: true})
}

// VerifyUser godoc
// @Tags User
// @Summary Verify a user with a code
// @Accept json
// @Produce json
// @Param body body httpmodel.VerifyUserRequest true "code"
// @Success 200 {object} httpmodel.UserResponse
// @Router /api/users/verify [post]
func (h *UserHandler) VerifyUser(c echo.Context) error {
	ctx := c.Request().Context()
	req := &httpmodel.VerifyUserRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	u, err := httpinternal.Usecases(c).User.VerifyUser(ctx, req.Code)
	if err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.NewUserResponse(u))
}

// StartPasswordReset godoc
// @Tags User
// @Summary Start a password reset
// @Accept json
// @Produce json
// @Param body body httpmodel.StartPasswordResetRequest true "email"
// @Success 200 {object} httpmodel.MessageResponse
// @Router /api/users/password-reset/start [post]
func (h *UserHandler) StartPasswordReset(c echo.Context) error {
	ctx := c.Request().Context()
	req := &httpmodel.StartPasswordResetRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	if err := httpinternal.Usecases(c).User.StartPasswordReset(ctx, req.Email); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.MessageResponse{Success: true})
}

// PasswordReset godoc
// @Tags User
// @Summary Complete a password reset
// @Accept json
// @Produce json
// @Param body body httpmodel.PasswordResetRequest true "password and token"
// @Success 200 {object} httpmodel.MessageResponse
// @Router /api/users/password-reset [post]
func (h *UserHandler) PasswordReset(c echo.Context) error {
	ctx := c.Request().Context()
	req := &httpmodel.PasswordResetRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	if err := httpinternal.Usecases(c).User.PasswordReset(ctx, req.Password, req.Token); err != nil {
		return err
	}
	return c.JSON(http.StatusOK, httpmodel.MessageResponse{Success: true})
}

// FindOrCreate godoc
// @Tags User
// @Summary Find or create a user (M2M or JWT)
// @Accept json
// @Produce json
// @Param body body httpmodel.FindOrCreateRequest true "sub, iss, token"
// @Success 204 "No Content (stub mirroring the GraphQL findOrCreate resolver)"
// @Failure 400 {object} internal.ErrorResponse
// @Router /api/users/find-or-create [post]
func (h *UserHandler) FindOrCreate(c echo.Context) error {
	// The GraphQL findOrCreate resolver is currently a stub (returns nil), and the
	// User.FindOrCreate interactor — although implemented on the concrete interactor —
	// is NOT exposed on the interfaces.User interface that the container holds, so it
	// cannot be invoked here without modifying that shared interface (out of scope for
	// an additive REST layer). Mirror the GraphQL stub: validate the body then return
	// 204 until the interactor method is promoted to the interface.
	req := &httpmodel.FindOrCreateRequest{}
	if err := httpinternal.BindValidate(c, req); err != nil {
		return err
	}
	return c.NoContent(http.StatusNoContent)
}

// parseUserIDRef parses an optional user ID reference: (nil, nil) when omitted/empty,
// (id, nil) when valid, and (nil, err) when a value is provided but malformed so the
// caller can return 400 rather than silently ignoring bad input.
func parseUserIDRef(s *string) (*id.UserID, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	uid, err := id.UserIDFrom(*s)
	if err != nil {
		return nil, err
	}
	return &uid, nil
}

// parseWorkspaceIDRef parses an optional workspace ID reference; see parseUserIDRef.
func parseWorkspaceIDRef(s *string) (*id.WorkspaceID, error) {
	if s == nil || *s == "" {
		return nil, nil
	}
	wid, err := id.WorkspaceIDFrom(*s)
	if err != nil {
		return nil, err
	}
	return &wid, nil
}

func adapterAuthInfo(c echo.Context) *appx.AuthInfo {
	return adapter.GetAuthInfo(c.Request().Context())
}
