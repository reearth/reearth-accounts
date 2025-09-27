package app

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/pprof"
	"time"

	"github.com/99designs/gqlgen/graphql/playground"
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/reearth/reearth-accounts/internal/adapter"
	"github.com/reearth/reearth-accounts/internal/usecase/interactor"
	"github.com/reearth/reearthx/appx"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
)

type responseBodyWriter struct {
	http.ResponseWriter
	body *bytes.Buffer
}

func (w *responseBodyWriter) Write(b []byte) (int, error) {
	w.body.Write(b)
	return w.ResponseWriter.Write(b)
}

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
	e.Use(accessLogger())

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

	api.POST(
		"/graphql", GraphqlAPI(cfg.Config.GraphQL, cfg.Config.Dev),
		middleware.CORSWithConfig(middleware.CORSConfig{AllowOrigins: origins}),
		echo.WrapMiddleware(jwt),
		echo.WrapMiddleware(authMiddleware(cfg)),
		cacheControl,
		usecaseMiddleware,
	)

	return e
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

func accessLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			res := c.Response()
			start := time.Now()

			reqid := log.GetReqestID(res, req)

			// Capture request body
			var requestBody string
			if req.Body != nil {
				bodyBytes, err := io.ReadAll(req.Body)
				if err == nil {
					requestBody = string(bodyBytes)
					// Restore the body for downstream handlers
					req.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				}
			}

			// Capture response body using a custom response writer
			resBody := &bytes.Buffer{}
			writer := &responseBodyWriter{
				ResponseWriter: c.Response().Writer,
				body:           resBody,
			}
			c.Response().Writer = writer

			args := []any{
				"time_unix", start.Unix(),
				"remote_ip", c.RealIP(),
				"host", req.Host,
				"uri", req.RequestURI,
				"method", req.Method,
				"path", req.URL.Path,
				"protocol", req.Proto,
				"referer", req.Referer(),
				"user_agent", req.UserAgent(),
				"bytes_in", req.ContentLength,
				"request_id", reqid,
				"route", c.Path(),
				"request_body", requestBody,
				"response_body", "",
			}

			logger := log.GetLoggerFromContextOrDefault(c.Request().Context())
			logger = logger.WithCaller(false)

			// incoming log
			logger.Infow(
				fmt.Sprintf("<-- %s %s", req.Method, req.URL.Path),
				args...,
			)

			if err := next(c); err != nil {
				c.Error(err)
			}

			res = c.Response()
			stop := time.Now()
			latency := stop.Sub(start)
			latencyHuman := latency.String()

			// Get response body from the custom writer
			responseBody := resBody.String()

			// Update args with captured response body
			for i, arg := range args {
				if str, ok := arg.(string); ok && str == "response_body" && i+1 < len(args) {
					args[i+1] = responseBody
					break
				}
			}

			args = append(args,
				"status", res.Status,
				"bytes_out", res.Size,
				"letency", latency.Microseconds(),
				"latency_human", latencyHuman,
			)

			// outcoming log
			logger.Infow(
				fmt.Sprintf("--> %s %d %s %s", req.Method, res.Status, req.URL.Path, latencyHuman),
				args...,
			)
			return nil
		}
	}
}
