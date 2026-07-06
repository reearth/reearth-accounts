package di

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/reearth/reearth-accounts/server/internal/admin/auth/session"
	"github.com/reearth/reearth-accounts/server/internal/admin/gateway/google"
	authhandler "github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/auth"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/authuc"
	mongorepo "github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/appx"
	"github.com/reearth/reearthx/log"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Config holds the admin-api configuration. It shares the DB and Cerbos env
// vars with the main service and adds admin-specific HTTP settings. The
// provide* functions convert it into per-dependency values for injection.
type Config struct {
	Port string `default:"8091" envconfig:"REEARTH_ACCOUNTS_ADMIN_PORT"`
	Host string `default:"0.0.0.0" envconfig:"REEARTH_ACCOUNTS_ADMIN_HOST"`
	Env  string `envconfig:"REEARTH_ACCOUNTS_ADMIN_ENV"`

	DB       string   `default:"mongodb://localhost" envconfig:"REEARTH_ACCOUNTS_DB"`
	DBName   string   `default:"reearth-account" envconfig:"REEARTH_ACCOUNTS_DB_NAME"`
	DBDriver string   `envconfig:"REEARTH_ACCOUNTS_DB_DRIVER"`
	Origins  []string `envconfig:"REEARTH_ACCOUNTS_ADMIN_ORIGINS"`

	// auth (Auth0 JWT) — mirrors the main service's auth env vars.
	Auth0    Auth0Config
	Auth_ISS string
	Auth_AUD string
	Auth_ALG *string
	Auth_TTL *int
	Auth     AuthConfigs

	// cerbos
	CerbosHost   string `envconfig:"CERBOS_HOST"`
	CerbosUseSSL bool   `default:"true" envconfig:"REEARTH_ACCOUNTS_CERBOS_USE_SSL"`

	// admin session auth (Google OAuth + self-issued session JWT)
	GoogleOAuthClientID string        `envconfig:"REEARTH_ACCOUNTS_ADMIN_GOOGLE_OAUTH_CLIENT_ID"`
	SessionSecret       string        `envconfig:"REEARTH_ACCOUNTS_ADMIN_SESSION_SECRET"`
	SessionTTL          time.Duration `default:"12h" envconfig:"REEARTH_ACCOUNTS_ADMIN_SESSION_TTL"`
	BootstrapEmails     []string      `envconfig:"REEARTH_ACCOUNTS_ADMIN_BOOTSTRAP_EMAILS"`
	AllowedEmailDomain  string        `default:"eukarya.io" envconfig:"REEARTH_ACCOUNTS_ADMIN_ALLOWED_EMAIL_DOMAIN"`
}

type Auth0Config struct {
	Domain   string
	Audience string
}

type AuthConfig struct {
	ISS string
	AUD []string
	ALG *string
	TTL *int
}

type AuthConfigs []appx.JWTProvider

// Decode is a custom envconfig decoder for a JSON-encoded list of providers.
func (ipd *AuthConfigs) Decode(value string) error {
	var providers []appx.JWTProvider
	if err := json.Unmarshal([]byte(value), &providers); err != nil {
		return fmt.Errorf("invalid identity providers json: %w", err)
	}
	*ipd = providers
	return nil
}

func (c Auth0Config) authConfig() *AuthConfig {
	domain := c.Domain
	if domain == "" {
		return nil
	}
	if !strings.HasPrefix(domain, "https://") && !strings.HasPrefix(domain, "http://") {
		domain = "https://" + domain
	}
	if !strings.HasSuffix(domain, "/") {
		domain += "/"
	}
	aud := []string{}
	if c.Audience != "" {
		aud = append(aud, c.Audience)
	}
	return &AuthConfig{ISS: domain, AUD: aud}
}

// Auths returns the configured JWT providers used to validate Auth0 tokens.
func (c Config) Auths() (res []appx.JWTProvider) {
	if ac := c.Auth0.authConfig(); ac != nil {
		res = append(res, appx.JWTProvider{ISS: ac.ISS, AUD: ac.AUD, ALG: ac.ALG, TTL: ac.TTL})
	}
	if c.Auth_ISS != "" {
		var aud []string
		if c.Auth_AUD != "" {
			aud = []string{c.Auth_AUD}
		}
		res = append(res, appx.JWTProvider{ISS: c.Auth_ISS, AUD: aud, ALG: c.Auth_ALG, TTL: c.Auth_TTL})
	}
	return append(res, c.Auth...)
}

// Env checks are case-insensitive so a mis-cased REEARTH_ACCOUNTS_ADMIN_ENV
// (e.g. "production") can't accidentally enable non-prod behavior such as
// serving Swagger or allowing CERBOS_HOST to be unset.
func (c *Config) IsProduction() bool  { return strings.EqualFold(c.Env, "Production") }
func (c *Config) IsDevelopment() bool { return c.Env == "" || strings.EqualFold(c.Env, "Development") }

// resolveDBDriver mirrors the main service: honor REEARTH_ACCOUNTS_DB_DRIVER,
// else infer from the DB URI scheme, defaulting to mongo.
func (c *Config) resolveDBDriver() string {
	if c.DBDriver != "" {
		switch strings.ToLower(c.DBDriver) {
		case "postgres", "postgresql":
			return "postgres"
		default:
			return "mongo"
		}
	}
	if strings.HasPrefix(c.DB, "postgres://") || strings.HasPrefix(c.DB, "postgresql://") {
		return "postgres"
	}
	return "mongo"
}

func (c *Config) Print() {
	masked := *c
	if masked.DB != "" {
		masked.DB = "***"
	}
	if masked.SessionSecret != "" {
		masked.SessionSecret = "***"
	}
	log.Infof("admin config: %+v", masked)
}

// LoadConfig is the root Wire provider for configuration.
func LoadConfig() *Config {
	if os.Getenv("SKIP_DOTENV") == "" {
		if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
			log.Warnf("failed to load .env: %v", err)
		} else if err == nil {
			log.Infof("config: .env loaded")
		}
	}

	var cfg Config
	if err := envconfig.Process("reearth", &cfg); err != nil {
		log.Fatalf("Error config setup: %v", err)
	}
	cfg.Print()
	return &cfg
}

// provideGoogleVerifier builds the Google id_token verifier bound to the admin
// OAuth client ID. A missing client ID is a misconfiguration that would reject
// every sign-in, so we fail fast in production (and warn in development).
func provideGoogleVerifier(cfg *Config) (google.Verifier, error) {
	if cfg.GoogleOAuthClientID == "" {
		if cfg.IsProduction() {
			return nil, fmt.Errorf("REEARTH_ACCOUNTS_ADMIN_GOOGLE_OAUTH_CLIENT_ID is required in production")
		}
		log.Warnf("admin Google OAuth client ID not configured; Google sign-in will reject all tokens")
	}
	return google.NewVerifier(cfg.GoogleOAuthClientID), nil
}

// provideSessionManager builds the session-token issuer/parser. An empty secret
// or non-positive TTL would silently break sign-in (500s / immediately-expired
// tokens), so we validate up front: TTL is always required, and the secret is
// required in production (a warning in development).
func provideSessionManager(cfg *Config) (*session.Manager, error) {
	if cfg.SessionTTL <= 0 {
		return nil, fmt.Errorf("REEARTH_ACCOUNTS_ADMIN_SESSION_TTL must be positive")
	}
	if cfg.SessionSecret == "" {
		if cfg.IsProduction() {
			return nil, fmt.Errorf("REEARTH_ACCOUNTS_ADMIN_SESSION_SECRET is required in production")
		}
		log.Warnf("admin session secret not configured; Google sign-in will fail until REEARTH_ACCOUNTS_ADMIN_SESSION_SECRET is set")
	}
	return session.NewManager(cfg.SessionSecret, cfg.SessionTTL), nil
}

// provideGoogleSignInOptions maps config to the sign-in policy.
func provideGoogleSignInOptions(cfg *Config) authuc.GoogleSignInOptions {
	return authuc.GoogleSignInOptions{
		AllowedDomain:   cfg.AllowedEmailDomain,
		BootstrapEmails: cfg.BootstrapEmails,
	}
}

// provideCookieSecure sets the session cookie's Secure attribute (on in prod).
func provideCookieSecure(cfg *Config) authhandler.CookieSecure {
	return authhandler.CookieSecure(cfg.IsProduction())
}

// provideRepoContainer connects to the configured backend (Mongo or Postgres)
// and builds the shared repository container. Schema migrations are owned by
// the main reearth-accounts service; the admin API only connects. The returned
// cleanup closes the underlying connection.
func provideRepoContainer(cfg *Config) (*repo.Container, func(), error) {
	if cfg.resolveDBDriver() == "postgres" {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		pool, err := pgxpool.New(ctx, cfg.DB)
		if err != nil {
			return nil, nil, fmt.Errorf("postgres pool init: %w", err)
		}
		if err := pool.Ping(ctx); err != nil {
			pool.Close()
			return nil, nil, fmt.Errorf("postgres ping: %w", err)
		}
		repos, err := postgres.New(context.Background(), pool, []user.Repo{})
		if err != nil {
			pool.Close()
			return nil, nil, fmt.Errorf("init postgres repos: %w", err)
		}
		return repos, func() { pool.Close() }, nil
	}

	// Bound startup connection like the Postgres branch so a network issue
	// can't hang the process indefinitely.
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	client, err := mongo.Connect(
		ctx,
		options.Client().ApplyURI(cfg.DB).SetConnectTimeout(time.Second*10))
	if err != nil {
		return nil, nil, fmt.Errorf("mongo connect: %w", err)
	}
	repos, err := mongorepo.New(context.Background(), client.Database(cfg.DBName), false, false, []user.Repo{})
	if err != nil {
		_ = client.Disconnect(context.Background())
		return nil, nil, fmt.Errorf("init mongo repos: %w", err)
	}
	cleanup := func() { _ = client.Disconnect(context.Background()) }
	return repos, cleanup, nil
}

// provideCerbosClient builds the Cerbos gRPC client. When CERBOS_HOST is unset
// it returns nil; the authz checker treats a nil client as "allow" for local
// development. In production a missing CERBOS_HOST is a misconfiguration that
// would silently disable all admin authorization, so we fail fast instead.
func provideCerbosClient(cfg *Config) (*cerbos.GRPCClient, error) {
	if cfg.CerbosHost == "" {
		if cfg.IsProduction() {
			return nil, fmt.Errorf("CERBOS_HOST is required in production: admin authorization cannot be disabled")
		}
		log.Warnf("cerbos host not configured; admin permission checks will be skipped")
		return nil, nil
	}
	var opts []cerbos.Opt
	if !cfg.CerbosUseSSL {
		opts = append(opts, cerbos.WithPlaintext(), cerbos.WithTLSInsecure())
	}
	client, err := cerbos.New(cfg.CerbosHost, opts...)
	if err != nil {
		return nil, fmt.Errorf("create cerbos client: %w", err)
	}
	return client, nil
}
