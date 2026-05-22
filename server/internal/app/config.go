package app

import (
	"encoding/json"
	"fmt"
	"net"
	"os"
	"strings"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
	"github.com/reearth/reearthx/log"
)

const configPrefix = "reearth"

type Config struct {
	Port    string `default:"8090" envconfig:"PORT"`
	Dev     bool
	DB      string   `default:"mongodb://localhost" envconfig:"REEARTH_ACCOUNTS_DB"`
	DBName  string   `default:"reearth-account" envconfig:"REEARTH_ACCOUNTS_DB_NAME"`
	Origins []string `envconfig:"REEARTH_ACCOUNTS_ORIGINS"`
	Host    string   `default:"0.0.0.0" envconfig:"HOST"`

	GCPProject string `envconfig:"GOOGLE_CLOUD_PROJECT"`
	Cert       CertConfig
	Policy     PolicyConfig

	// mock
	Mock_Auth bool `envconfig:"REEARTH_MOCK_AUTH" pp:",omitempty"`

	// auth
	Auth     AuthConfigs `pp:",omitempty"`
	Auth_ISS string      `pp:",omitempty"`
	Auth_AUD string      `pp:",omitempty"`
	Auth_ALG *string     `pp:",omitempty"`
	Auth_TTL *int        `pp:",omitempty"`
	Auth0    Auth0Config `pp:",omitempty"`

	CIP          CIPConfig `pp:",omitempty"`
	AuthProvider string    `default:"auth0" envconfig:"REEARTH_ACCOUNTS_AUTH_PROVIDER" pp:",omitempty"`

	GraphQL GraphQLConfig

	SignupSecret   string `envconfig:"REEARTH_ACCOUNTS_SIGNUP_SECRET"`
	HostWeb        string
	Reearth_API    string
	Reearth_Web    string
	Reearth_GCS    string
	Published_Host string

	// cerbos
	CerbosUseSSL bool   `envconfig:"REEARTH_ACCOUNTS_CERBOS_USE_SSL" default:"true"`
	CerbosHost   string `envconfig:"CERBOS_HOST"`

	// Storage
	StorageIsLocal          bool   `envconfig:"REEARTH_ACCOUNTS_STORAGE_IS_LOCAL"`
	StorageBucketName       string `envconfig:"REEARTH_ACCOUNTS_STORAGE_BUCKET_NAME" default:"reearth"`
	StorageEmulatorEnabled  bool   `envconfig:"REEARTH_ACCOUNTS_STORAGE_EMULATOR_ENABLED"`
	StorageEmulatorEndpoint string `envconfig:"REEARTH_ACCOUNTS_STORAGE_EMULATOR_ENDPOINT"`
}

type AuthConfig struct {
	ISS      string
	AUD      []string
	ALG      *string
	TTL      *int
	ClientID *string
}

type Auth0Config struct {
	Domain       string
	Audience     string
	ClientID     string
	ClientSecret string
	WebClientID  string
}

type CIPConfig struct {
	// ProjectID is the CIP / Firebase project id. When set, a CIP JWT provider
	// (ISS=https://securetoken.google.com/<ProjectID>, AUD=[<ProjectID>]) is
	// appended to Config.Auths(), and the Firebase Admin SDK targets this project.
	ProjectID string `envconfig:"REEARTH_ACCOUNTS_CIP_PROJECT_ID"`
	// TenantID is an optional GCIP multi-tenant id. Issuer/audience are unchanged;
	// it only scopes Admin SDK management calls to the tenant.
	TenantID string `envconfig:"REEARTH_ACCOUNTS_CIP_TENANT_ID"`
	// APIKey is the public client API key advertised via authConfig (not a secret).
	APIKey string `envconfig:"REEARTH_ACCOUNTS_CIP_API_KEY"`
	// AuthDomain is the public client auth domain advertised via authConfig.
	AuthDomain string `envconfig:"REEARTH_ACCOUNTS_CIP_AUTH_DOMAIN"`
}

// AuthConfig builds the JWT validation parameters for CIP-issued ID tokens.
// Returns nil when CIP is unconfigured, so Config.Auths() stays unchanged.
func (c CIPConfig) AuthConfig() *AuthConfig {
	if c.ProjectID == "" {
		return nil
	}
	return &AuthConfig{
		ISS: "https://securetoken.google.com/" + c.ProjectID,
		AUD: []string{c.ProjectID},
		// ALG omitted: appx defaults to RS256, which is what CIP uses.
	}
}

type CertConfig struct {
	IP                net.IP
	PubSubTopicIssue  string
	PubSubTopicRevoke string
}

func (c Config) Auths() (res []appx.JWTProvider) {
	if ac := c.Auth0.AuthConfig(); ac != nil {
		a := appx.JWTProvider{
			ISS: ac.ISS,
			AUD: ac.AUD,
			ALG: ac.ALG,
			TTL: ac.TTL,
		}
		res = append(res, a)
	}
	if c.Auth_ISS != "" {
		var aud []string
		if len(c.Auth_AUD) > 0 {
			aud = []string{c.Auth_AUD}
		}
		a := appx.JWTProvider{
			ISS: c.Auth_ISS,
			AUD: aud,
			ALG: c.Auth_ALG,
			TTL: c.Auth_TTL,
		}
		res = append(res, a)
	}
	if ac := c.CIP.AuthConfig(); ac != nil {
		res = append(res, appx.JWTProvider{
			ISS: ac.ISS,
			AUD: ac.AUD,
			ALG: ac.ALG,
			TTL: ac.TTL,
		})
	}
	return append(res, c.Auth...)
}

type AuthConfigs []appx.JWTProvider

// Decode is a custom decoder for AuthConfigs
func (ipd *AuthConfigs) Decode(value string) error {
	var providers []appx.JWTProvider
	if err := json.Unmarshal([]byte(value), &providers); err != nil {
		return fmt.Errorf("invalid identity providers json: %w", err)
	}

	*ipd = providers
	return nil
}

func (a AuthConfig) JWTProvider() appx.JWTProvider {
	return appx.JWTProvider{
		ISS: a.ISS,
		AUD: a.AUD,
		ALG: a.ALG,
		TTL: a.TTL,
	}
}

func (c Auth0Config) AuthConfig() *AuthConfig {
	domain := c.Domain
	if c.Domain == "" {
		return nil
	}
	if !strings.HasPrefix(domain, "https://") && !strings.HasPrefix(domain, "http://") {
		domain = "https://" + domain
	}
	if !strings.HasSuffix(domain, "/") {
		domain = domain + "/"
	}
	aud := []string{}
	if c.Audience != "" {
		aud = append(aud, c.Audience)
	}
	return &AuthConfig{
		ISS: domain,
		AUD: aud,
	}
}

type PolicyConfig struct {
	Default *workspace.PolicyID
}

func ReadConfig(debug bool) (*Config, error) {
	// load .env unless explicitly skipped (e.g. docker-compose with env_file)
	if os.Getenv("SKIP_DOTENV") == "" {
		if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
			return nil, err
		} else if err == nil {
			log.Infof("config: .env loaded")
		}
	}

	var c Config
	if err := envconfig.Process(configPrefix, &c); err != nil {
		return nil, err
	}

	if debug {
		c.Dev = true
	}
	return &c, nil
}

type GraphQLConfig struct {
	ComplexityLimit int `default:"6000"`
}

func (c Config) Print() string {
	s := fmt.Sprintf("%+v", c)
	for _, secret := range []string{c.DB} {
		if secret == "" {
			continue
		}
		s = strings.ReplaceAll(s, secret, "***")
	}
	return s
}

// Auth0ConfigProvider interface implementation
func (c Config) GetAuth0Domain() string {
	return c.Auth0.Domain
}

func (c Config) GetAuth0Audience() string {
	return c.Auth0.Audience
}

func (c Config) GetAuth0WebClientID() string {
	return c.Auth0.WebClientID
}
