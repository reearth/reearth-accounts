package app

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/pprof"
	"strings"
	"time"

	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/golang-jwt/jwt"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interactor"
	"github.com/reearth/reearthx/appx"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
)

func initEcho(ctx context.Context, cfg *ServerConfig) *echo.Echo {
	if cfg.Config == nil {
		log.Fatalc(ctx, "ServerConfig.Config is nil")
	}

	e := echo.New()
	e.Debug = cfg.Debug
	e.HideBanner = true
	e.HidePort = true

	logger := log.NewEcho()
	e.Logger = logger
	e.Use(logger.AccessLogger())

	origins := allowedOrigins(cfg)
	if len(origins) > 0 {
		e.Use(
			middleware.CORSWithConfig(middleware.CORSConfig{
				AllowOrigins: origins,
			}),
		)
	}

	if e.Debug {
		// enable pprof
		pprofGroup := e.Group("/debug/pprof")
		pprofGroup.Any("/cmdline", echo.WrapHandler(http.HandlerFunc(pprof.Cmdline)))
		pprofGroup.Any("/profile", echo.WrapHandler(http.HandlerFunc(pprof.Profile)))
		pprofGroup.Any("/symbol", echo.WrapHandler(http.HandlerFunc(pprof.Symbol)))
		pprofGroup.Any("/trace", echo.WrapHandler(http.HandlerFunc(pprof.Trace)))
		pprofGroup.Any("/*", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	}

	e.HTTPErrorHandler = func(err error, c echo.Context) {
		if c.Response().Committed {
			return
		}

		code, msg := errorMessage(err, func(f string, args ...interface{}) {
			c.Echo().Logger.Errorf(f, args...)
		})
		if err := c.JSON(code, map[string]string{
			"error": msg,
		}); err != nil {
			e.DefaultHTTPErrorHandler(err, c)
		}
	}

	// GraphQL Playground without auth
	if cfg.Debug || cfg.Config.Dev {
		e.GET("/graphql", echo.WrapHandler(
			playground.Handler("reearth-cloud", "/api/graphql"),
		))
		log.Printf("gql: GraphQL Playground is available")
	}

	usecaseMiddleware := UsecaseMiddleware(
		cfg.Repos,
		cfg.Gateways,
		nil,
		cfg.CerbosAdapter,
		interactor.ContainerConfig{
			SignupSecret:    cfg.Config.SignupSecret,
			AuthSrvUIDomain: cfg.Config.HostWeb,
		})

	// API
	api := e.Group("/api")
	jwt, err := appx.AuthMiddleware(cfg.Config.Auths(), adapter.AuthInfoKey, true)
	if err != nil {
		log.Panicc(ctx, err)
	}

	// TODO: Remove after debugging JWT issues
	e.GET("/debug/jwt", jwtDebugHandler)
	log.Printf("debug: JWT debug endpoint is available at /debug/jwt")
	api.Use(jwtDebugMiddleware)

	api.POST(
		"/graphql", GraphqlAPI(cfg.Config, cfg.Config.Dev),
		middleware.CORSWithConfig(middleware.CORSConfig{AllowOrigins: origins}),
		echo.WrapMiddleware(jwt),
		echo.WrapMiddleware(authMiddleware(cfg)),
		cacheControl,
		usecaseMiddleware,
	)

	return e
}

func jwtDebugHandler(c echo.Context) error {
	token := c.Request().Header.Get("Authorization")

	result := map[string]interface{}{
		"server_time": map[string]interface{}{
			"utc":      time.Now().UTC().Format(time.RFC3339),
			"unix":     time.Now().Unix(),
			"jst":      time.Now().In(time.FixedZone("JST", 9*60*60)).Format(time.RFC3339),
			"location": time.Now().Location().String(),
		},
	}

	if token != "" {
		token = strings.TrimPrefix(token, "Bearer ")
		parser := jwt.Parser{}
		t, _, err := parser.ParseUnverified(token, jwt.MapClaims{})

		if err == nil {
			result["token_raw"] = t.Claims

			if claims, ok := t.Claims.(jwt.MapClaims); ok {
				decoded := map[string]interface{}{
					"iss": claims["iss"],
					"sub": claims["sub"],
					"aud": claims["aud"],
				}

				if iat, ok := claims["iat"].(float64); ok {
					iatTime := time.Unix(int64(iat), 0)
					decoded["iat"] = map[string]interface{}{
						"unix": int64(iat),
						"utc":  iatTime.UTC().Format(time.RFC3339),
						"jst":  iatTime.In(time.FixedZone("JST", 9*60*60)).Format(time.RFC3339),
					}
				}

				if exp, ok := claims["exp"].(float64); ok {
					expTime := time.Unix(int64(exp), 0)
					decoded["exp"] = map[string]interface{}{
						"unix": int64(exp),
						"utc":  expTime.UTC().Format(time.RFC3339),
						"jst":  expTime.In(time.FixedZone("JST", 9*60*60)).Format(time.RFC3339),
					}

					if time.Now().Unix() > int64(exp) {
						decoded["status"] = "EXPIRED"
					} else {
						decoded["status"] = "VALID"
						decoded["expires_in_seconds"] = int64(exp) - time.Now().Unix()
					}
				}

				result["token_decoded"] = decoded
			}
		} else {
			result["parse_error"] = err.Error()
		}
	} else {
		result["message"] = "No Authorization header found"
	}

	return c.JSON(200, result)
}

func jwtDebugMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		authHeader := c.Request().Header.Get("Authorization")

		if authHeader != "" {
			log.Infof("=== JWT Debug Start ===")
			log.Infof("Request time (UTC): %v", time.Now().UTC())
			log.Infof("Request time (Unix): %v", time.Now().Unix())
			log.Infof("Request path: %v", c.Request().URL.Path)

			token := strings.TrimPrefix(authHeader, "Bearer ")
			parser := jwt.Parser{}
			t, _, err := parser.ParseUnverified(token, jwt.MapClaims{})

			if err == nil && t != nil {
				if claims, ok := t.Claims.(jwt.MapClaims); ok {
					if iat, ok := claims["iat"].(float64); ok {
						log.Infof("Token iat: %v (issued at: %v)", int64(iat), time.Unix(int64(iat), 0))
					}
					if exp, ok := claims["exp"].(float64); ok {
						log.Infof("Token exp: %v (expires at: %v)", int64(exp), time.Unix(int64(exp), 0))
						log.Infof("Token expires in: %v seconds", int64(exp)-time.Now().Unix())
					}
				}
			}
			log.Infof("=== JWT Debug End ===")
		}

		err := next(c)
		if err != nil && strings.Contains(fmt.Sprintf("%v", err), "expired") {
			log.Errorf("JWT validation failed: %v", err)
		}

		return err
	}
}

func allowedOrigins(cfg *ServerConfig) []string {
	if cfg == nil {
		return nil
	}
	origins := append([]string{}, cfg.Config.Origins...)
	if cfg.Debug {
		origins = append(origins, "http://localhost:3000")
	}
	return origins
}

func errorMessage(err error, log func(string, ...interface{})) (int, string) {
	code := http.StatusBadRequest
	msg := err.Error()

	if err2, ok := err.(*echo.HTTPError); ok {
		code = err2.Code
		if msg2, ok := err2.Message.(string); ok {
			msg = msg2
		} else if msg2, ok := err2.Message.(error); ok {
			msg = msg2.Error()
		} else {
			msg = "error"
		}
		if err2.Internal != nil {
			log("echo internal err: %+v", err2)
		}
	} else if errors.Is(err, rerror.ErrNotFound) {
		code = http.StatusNotFound
		msg = "not found"
	} else {
		if ierr := rerror.UnwrapErrInternal(err); ierr != nil {
			code = http.StatusInternalServerError
			msg = "internal server error"
		}
	}

	return code, msg
}

func cacheControl(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "private")
		return next(c)
	}
}
