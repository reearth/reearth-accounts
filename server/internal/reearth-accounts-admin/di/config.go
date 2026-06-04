package di

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	infraCerbos "github.com/reearth/reearth-accounts/server/internal/infrastructure/cerbos"
	mongorepo "github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/appx"
	"github.com/reearth/reearthx/log"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// Config holds the admin-api configuration. It is intentionally scoped to what
// the admin context needs (DB, auth, cerbos) and is converted into per-layer
// configs by the provide* functions below before injection.
type Config struct {
	Port string `default:"8091" envconfig:"REEARTH_ACCOUNTS_ADMIN_PORT"`
	Host string `default:"0.0.0.0" envconfig:"REEARTH_ACCOUNTS_ADMIN_HOST"`
	Env  string `envconfig:"REEARTH_ACCOUNTS_ADMIN_ENV"`

	DB      string   `default:"mongodb://localhost" envconfig:"REEARTH_ACCOUNTS_DB"`
	DBName  string   `default:"reearth-account" envconfig:"REEARTH_ACCOUNTS_DB_NAME"`
	Origins []string `envconfig:"REEARTH_ACCOUNTS_ADMIN_ORIGINS"`

	// auth (Auth0 JWT)
	Auth0    Auth0Config
	Auth_ISS string
	Auth_AUD string
	Auth_ALG *string
	Auth_TTL *int
	Auth     AuthConfigs

	// cerbos
	CerbosHost   string `envconfig:"CERBOS_HOST"`
	CerbosUseSSL bool   `default:"true" envconfig:"REEARTH_ACCOUNTS_CERBOS_USE_SSL"`
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

func (c *Config) IsProduction() bool  { return c.Env == "Production" }
func (c *Config) IsDevelopment() bool { return c.Env == "Development" || c.Env == "" }

func (c *Config) Print() {
	masked := *c
	if masked.DB != "" {
		masked.DB = "***"
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

// provideJWTProviders converts the config into the JWT provider list consumed
// by the auth middleware.
func provideJWTProviders(cfg *Config) []appx.JWTProvider {
	return cfg.Auths()
}

// provideMongoClient connects to MongoDB and returns the client plus a cleanup
// function that disconnects it (used by Wire for graceful teardown).
func provideMongoClient(cfg *Config) (*mongo.Client, func(), error) {
	ctx := context.Background()
	client, err := mongo.Connect(
		ctx,
		options.Client().ApplyURI(cfg.DB).SetConnectTimeout(time.Second*10))
	if err != nil {
		return nil, nil, fmt.Errorf("mongo connect: %w", err)
	}
	cleanup := func() {
		_ = client.Disconnect(context.Background())
	}
	return client, cleanup, nil
}

// provideRepoContainer builds the shared repository container over MongoDB.
func provideRepoContainer(client *mongo.Client, cfg *Config) (*repo.Container, error) {
	repos, err := mongorepo.New(context.Background(), client.Database(cfg.DBName), false, false, []user.Repo{})
	if err != nil {
		return nil, fmt.Errorf("init mongo repos: %w", err)
	}
	return repos, nil
}

// provideCerbosGateway builds the Cerbos gateway. When CERBOS_HOST is unset it
// returns nil; the permission checker treats a nil gateway as "allow" for local
// development.
func provideCerbosGateway(cfg *Config) (gateway.CerbosGateway, error) {
	if cfg.CerbosHost == "" {
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
	return infraCerbos.NewCerbosAdapter(client), nil
}
