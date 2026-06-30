package di

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation"
	"github.com/reearth/reearthx/log"
)

// Server wraps the Echo instance with start/graceful-shutdown behavior.
type Server struct {
	echo    *echo.Echo
	address string
}

// NewAppEchoServer is a Wire provider that builds the configured Echo server.
func NewAppEchoServer(
	cfg *Config,
	handler *presentation.Handler,
	appMiddlewares *presentation.AppMiddlewares,
) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Validator = newValidator()

	e.Use(
		middleware.Recover(),
		middleware.RequestID(),
	)
	if origins := allowedOrigins(cfg); len(origins) > 0 {
		e.Use(middleware.CORSWithConfig(middleware.CORSConfig{AllowOrigins: origins}))
	}
	e.Use(appMiddlewares.Middlewares()...)

	e.HTTPErrorHandler = presentation.CustomHTTPErrorHandler

	presentation.RegisterRoutes(e, handler)
	if !cfg.IsProduction() {
		presentation.RegisterSwaggerRoutes(e)
	}

	port := cfg.Port
	if port == "" {
		port = "8091"
	}
	host := cfg.Host
	if host == "" {
		host = "0.0.0.0"
	}

	return &Server{echo: e, address: host + ":" + port}
}

// Start runs the server until SIGINT/SIGTERM, then shuts down gracefully with a
// 10-second timeout.
func (s *Server) Start() {
	go func() {
		if err := s.echo.Start(s.address); err != nil && err != http.ErrServerClosed {
			log.Fatalf("admin server: %v", err)
		}
	}()
	log.Infof("admin server started at http://%s", s.address)

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := s.echo.Shutdown(ctx); err != nil {
		log.Errorf("admin server shutdown: %v", err)
	}
	log.Infof("admin server stopped")
}

func allowedOrigins(cfg *Config) []string {
	origins := append([]string{}, cfg.Origins...)
	if cfg.IsDevelopment() {
		origins = append(origins, "http://localhost:3000")
	}
	return origins
}
