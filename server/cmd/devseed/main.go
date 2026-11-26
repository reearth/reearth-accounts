// Command devseed writes dev-only keys and accounts config for service callers
// that get their tokens from the local mock issuer in docker-compose.dev.yml
// (ADR-0002 decision 11.8). It never prints a private key.
package main

import (
	"bufio"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	serverModule = "github.com/reearth/reearth-accounts/server"
	ignoredDir   = ".dev"
	envFileName  = "accounts.env"
)

type client struct {
	id    string
	actAs []string
}

type options struct {
	dir      string
	host     string
	port     int
	issuerID string
	audience string
	clients  []client
	rotate   bool
}

func (o options) issuerURL() string {
	return fmt.Sprintf("http://%s/%s", net.JoinHostPort(o.host, fmt.Sprint(o.port)), o.issuerID)
}

func (o options) tokenURL() string { return o.issuerURL() + "/token" }

func (o options) keyPath(c client) string { return filepath.Join(o.dir, c.id+".key") }

// The ID becomes a file name and a registry name, so its charset is fixed.
var clientIDPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,62}$`)

type clientFlags []client

func (f *clientFlags) String() string { return "" }

func (f *clientFlags) Set(v string) error {
	id, kinds, ok := strings.Cut(v, "=")
	if !ok || !clientIDPattern.MatchString(id) {
		return fmt.Errorf("want <id>=<act_as>[,<act_as>] with an id matching %s, got %q", clientIDPattern, v)
	}
	actAs := []string{}
	if kinds != "" {
		actAs = strings.Split(kinds, ",")
	}
	*f = append(*f, client{id: id, actAs: actAs})
	return nil
}

func main() {
	if err := run(os.Args[1:], os.Stdout); err != nil {
		fmt.Fprintln(os.Stderr, "devseed:", err)
		os.Exit(1)
	}
}

func run(args []string, out io.Writer) error {
	fs := flag.NewFlagSet("devseed", flag.ContinueOnError)
	dir := fs.String("dir", filepath.Join(ignoredDir, "service-callers"), "output directory, under the server module's "+ignoredDir+" directory")
	o := options{}
	fs.StringVar(&o.host, "host", "mock-oauth2", "issuer host: the mock's compose service name")
	fs.IntVar(&o.port, "port", 8091, "issuer port, the same inside and outside compose")
	fs.StringVar(&o.issuerID, "issuer-id", "accounts-local", "issuer ID in server/dev/mock-oauth2/config.json")
	fs.StringVar(&o.audience, "audience", "reearth-accounts-local", "accounts audience")
	fs.BoolVar(&o.rotate, "rotate", false, "replace existing keys")
	var clients clientFlags
	fs.Var(&clients, "client", "a client as <id>=<act_as>[,<act_as>], repeatable (default cms=integration)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	o.clients = clients
	if len(o.clients) == 0 {
		o.clients = []client{{id: "cms", actAs: []string{"integration"}}}
	}

	root, err := serverRoot()
	if err != nil {
		return err
	}
	if o.dir, err = guardDir(root, *dir); err != nil {
		return err
	}
	return seed(o, out)
}

// serverRoot finds the server module's directory from the working directory.
func serverRoot() (string, error) {
	d, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if isServerModule(filepath.Join(d, "go.mod")) {
			return d, nil
		}
		parent := filepath.Dir(d)
		if parent == d {
			return "", errors.New("run from inside the server module")
		}
		d = parent
	}
}

func isServerModule(goMod string) bool {
	f, err := os.Open(goMod)
	if err != nil {
		return false
	}
	defer func() { _ = f.Close() }()
	s := bufio.NewScanner(f)
	for s.Scan() {
		if line := strings.TrimSpace(s.Text()); strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module ")) == serverModule
		}
	}
	return false
}

// guardDir returns dir as an absolute path, or an error when it is not inside
// root's ignored directory, symlinks resolved.
func guardDir(root, dir string) (string, error) {
	if !filepath.IsAbs(dir) {
		dir = filepath.Join(root, dir)
	}
	dir = filepath.Clean(dir)
	base := filepath.Join(root, ignoredDir)
	if !within(base, dir) {
		return "", fmt.Errorf("%s is outside %s, which git ignores", dir, base)
	}
	realBase, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", err
	}
	realBase = filepath.Join(realBase, ignoredDir)
	for p := dir; within(base, p); p = filepath.Dir(p) {
		real, err := filepath.EvalSymlinks(p)
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return "", err
		}
		rel, _ := filepath.Rel(base, p)
		if real != filepath.Join(realBase, rel) {
			return "", fmt.Errorf("%s resolves outside %s", p, base)
		}
		break
	}
	return dir, nil
}

func within(base, p string) bool {
	rel, err := filepath.Rel(base, p)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}

func seed(o options, out io.Writer) error {
	if err := os.MkdirAll(o.dir, 0o700); err != nil {
		return err
	}
	if err := os.Chmod(o.dir, 0o700); err != nil {
		return err
	}
	for _, c := range o.clients {
		if err := ensureKey(o.keyPath(c), o.rotate); err != nil {
			return fmt.Errorf("client %s: %w", c.id, err)
		}
	}
	env, err := envFragment(o)
	if err != nil {
		return err
	}
	envPath := filepath.Join(o.dir, envFileName)
	if err := writeFileAtomic(envPath, []byte(env)); err != nil {
		return err
	}
	printSettings(out, o, envPath)
	return nil
}

func ensureKey(path string, rotate bool) error {
	fi, err := os.Lstat(path)
	switch {
	case err == nil && !fi.Mode().IsRegular():
		return fmt.Errorf("%s exists and is not a regular file", path)
	case err == nil && !rotate:
		return os.Chmod(path, 0o600)
	case err != nil && !errors.Is(err, os.ErrNotExist):
		return err
	}
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	der, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		return err
	}
	return writeFileAtomic(path, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: der}))
}

func writeFileAtomic(path string, data []byte) error {
	f, err := os.CreateTemp(filepath.Dir(path), "."+filepath.Base(path)+".*")
	if err != nil {
		return err
	}
	tmp := f.Name()
	defer func() { _ = os.Remove(tmp) }()
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return err
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func envFragment(o options) (string, error) {
	auth, err := json.Marshal([]struct {
		ISS string   `json:"iss"`
		AUD []string `json:"aud"`
	}{{ISS: o.issuerURL(), AUD: []string{o.audience}}})
	if err != nil {
		return "", err
	}
	type entry struct {
		Issuer   string   `json:"issuer"`
		ClientID string   `json:"client_id"`
		Name     string   `json:"name"`
		ActAs    []string `json:"act_as"`
	}
	entries := make([]entry, 0, len(o.clients))
	for _, c := range o.clients {
		entries = append(entries, entry{Issuer: o.issuerURL(), ClientID: c.id, Name: c.id, ActAs: c.actAs})
	}
	callers, err := json.Marshal(entries)
	if err != nil {
		return "", err
	}
	// Single quotes keep the JSON literal for docker compose, sh, and godotenv.
	for _, v := range [][]byte{auth, callers} {
		if strings.ContainsRune(string(v), '\'') {
			return "", errors.New("a value contains a single quote")
		}
	}
	return fmt.Sprintf(`# Written by make dev-seed for the local mock issuer. Dev-only: never use in a deployed environment.
REEARTH_ACCOUNTS_LOCAL_ISSUER=true
REEARTH_AUTH='%s'
REEARTH_ACCOUNTS_SERVICE_CALLERS='%s'
`, auth, callers), nil
}

func printSettings(out io.Writer, o options, envPath string) {
	_, _ = fmt.Fprintf(out, "accounts env file: %s\n\n", envPath)
	for _, c := range o.clients {
		_, _ = fmt.Fprintf(out, "client %s (act_as: %s)\n", c.id, strings.Join(c.actAs, ", "))
		_, _ = fmt.Fprintf(out, "  token URL: %s\n", o.tokenURL())
		_, _ = fmt.Fprintf(out, "  client ID: %s\n", c.id)
		_, _ = fmt.Fprintf(out, "  audience:  %s\n", o.audience)
		_, _ = fmt.Fprintf(out, "  key file:  %s\n\n", o.keyPath(c))
	}
	_, _ = fmt.Fprintf(out, "A caller that runs outside compose needs this line in its hosts file:\n  127.0.0.1 %s\n", o.host)
}
