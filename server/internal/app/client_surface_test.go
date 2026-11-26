package app

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/adapter/gql"
	adapterhttp "github.com/reearth/reearth-accounts/server/internal/adapter/http"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vektah/gqlparser/v2/ast"
)

var updateGolden = flag.Bool("update", false, "rewrite testdata golden files")

func TestClientSurface(t *testing.T) {
	var lines []string
	for k, a := range adapterhttp.ClientRoutes() {
		assert.False(t, a.Admits(workspace.SubjectUser), "%s must not admit a client acting for a user yet", k)
		lines = append(lines, fmt.Sprintf("rest    %-60s %s", k, a))
	}
	for f, a := range clientFields {
		assert.False(t, a.Admits(workspace.SubjectUser), "%s %s must not admit a client acting for a user yet", f.op, f.name)
		lines = append(lines, fmt.Sprintf("graphql %-60s %s", string(f.op)+" "+f.name, a))
	}
	sort.Strings(lines)
	got := strings.Join(lines, "\n") + "\n"

	path := filepath.Join("testdata", "client_surface.golden")
	if *updateGolden {
		require.NoError(t, os.MkdirAll("testdata", 0o755))
		require.NoError(t, os.WriteFile(path, []byte(got), 0o644))
	}
	want, err := os.ReadFile(path)
	require.NoError(t, err)
	assert.Equal(t, string(want), got, "the client surface changed; rerun with -update and review the diff")
}

type clientSurface struct {
	e        *echo.Echo
	cfg      *ServerConfig
	kinds    map[string]workspace.SubjectKind
	tokenFor func(kind string) map[string]string
}

func newClientSurface(t *testing.T) clientSurface {
	t.Helper()
	idp := newTestIdP(t)
	f := newServiceFixture(t)
	cfg := serviceConfig(t, f.repos, []*testIdP{idp}, callersJSON(idp.issuer(), "svc-flow", "integration"))
	cfg.Gateways = &gateway.Container{}
	tok := idp.sign(t, clientClaims("svc-flow"))
	return clientSurface{
		e:     initEcho(context.Background(), cfg),
		cfg:   cfg,
		kinds: map[string]workspace.SubjectKind{"anonymous": workspace.SubjectNone, "integration": workspace.SubjectIntegration},
		tokenFor: func(kind string) map[string]string {
			h := map[string]string{"Authorization": "Bearer " + tok}
			if kind == "integration" {
				h[ActAsHeader] = f.actAs()
			}
			return h
		},
	}
}

func (s clientSurface) do(method, path, body string, h map[string]string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	for k, v := range h {
		req.Header.Set(k, v)
	}
	rr := httptest.NewRecorder()
	s.e.ServeHTTP(rr, req)
	return rr
}

func concretePath(p string) string {
	parts := strings.Split(p, "/")
	for i, seg := range parts {
		if strings.HasPrefix(seg, ":") || seg == "*" {
			parts[i] = id.NewWorkspaceID().String()
		}
	}
	return strings.Join(parts, "/")
}

func TestClientGate_RESTDeniesEveryUnlistedRoute(t *testing.T) {
	s := newClientSurface(t)
	table := adapterhttp.ClientRoutes()
	paths := map[string]struct{}{}
	for _, r := range s.e.Routes() {
		if r.Method != echo.RouteNotFound && strings.HasPrefix(r.Path, "/api/") && r.Path != "/api/graphql" {
			paths[r.Path] = struct{}{}
		}
	}
	methods := []string{http.MethodGet, http.MethodHead, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions}
	checked := 0
	for p := range paths {
		for _, m := range methods {
			path := concretePath(p)
			c := s.e.NewContext(httptest.NewRequest(m, path, nil), httptest.NewRecorder())
			s.e.Router().Find(m, path, c)
			key := m + " " + c.Path()
			for kind, k := range s.kinds {
				if table[key].Admits(k) {
					continue
				}
				checked++
				rr := s.do(m, path, "{}", s.tokenFor(kind))
				assert.Equal(t, http.StatusForbidden, rr.Code, "%s as %s", key, kind)
				if m != http.MethodHead {
					assert.Contains(t, rr.Body.String(), "not open to client tokens", "%s as %s", key, kind)
				}
			}
		}
	}
	assert.Greater(t, checked, 300)
}

func TestClientGate_GraphQLDeniesEveryUnlistedField(t *testing.T) {
	s := newClientSurface(t)
	schema := gql.NewExecutableSchema(gql.Config{}).Schema()
	checked := 0
	for op, root := range map[ast.Operation]*ast.Definition{ast.Query: schema.Query, ast.Mutation: schema.Mutation} {
		for _, f := range root.Fields {
			for kind, k := range s.kinds {
				if clientFields[rootField{op: op, name: f.Name}].Admits(k) {
					continue
				}
				checked++
				doc := fieldDocument(schema, op, f)
				body, _ := json.Marshal(map[string]string{"query": doc})
				rr := s.do(http.MethodPost, "/api/graphql", string(body), s.tokenFor(kind))
				require.Equal(t, http.StatusForbidden, rr.Code, "%s as %s: %s", doc, kind, rr.Body.String())
				var res struct {
					Data   map[string]any `json:"data"`
					Errors []struct {
						Extensions map[string]any `json:"extensions"`
					} `json:"errors"`
				}
				require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &res))
				assert.Nil(t, res.Data, doc)
				require.Len(t, res.Errors, 1, doc)
				assert.Equal(t, errForbidden, res.Errors[0].Extensions["code"], doc)
			}
		}
	}
	assert.Greater(t, checked, 50)
}

// fieldDocument selects one root field with a value for every required argument.
func fieldDocument(s *ast.Schema, op ast.Operation, f *ast.FieldDefinition) string {
	var args []string
	for _, a := range f.Arguments {
		if a.Type.NonNull {
			args = append(args, a.Name+": "+literal(s, a.Type))
		}
	}
	sel := f.Name
	if len(args) > 0 {
		sel += "(" + strings.Join(args, ", ") + ")"
	}
	if d := s.Types[f.Type.Name()]; d != nil && d.Kind != ast.Scalar && d.Kind != ast.Enum {
		sel += " { __typename }"
	}
	return string(op) + " { " + sel + " }"
}

func literal(s *ast.Schema, t *ast.Type) string {
	if t.Elem != nil {
		return "[" + literal(s, t.Elem) + "]"
	}
	d := s.Types[t.NamedType]
	switch d.Kind {
	case ast.Enum:
		return d.EnumValues[0].Name
	case ast.InputObject:
		var fields []string
		for _, f := range d.Fields {
			if f.Type.NonNull {
				fields = append(fields, f.Name+": "+literal(s, f.Type))
			}
		}
		return "{" + strings.Join(fields, ", ") + "}"
	}
	switch d.Name {
	case "Int":
		return "1"
	case "Float":
		return "1.5"
	case "Boolean":
		return "true"
	default:
		return `"x"`
	}
}

func TestClientGate_RouteOutsideTheGatedGroupGetsNoOperator(t *testing.T) {
	s := newClientSurface(t)
	jwtm, err := appx.AuthMiddleware(s.cfg.Config.Auths(), adapter.AuthInfoKey, true)
	require.NoError(t, err)
	reached := false
	handler := func(c echo.Context) error {
		reached = true
		assert.Nil(t, adapter.Operator(c.Request().Context()))
		assert.Nil(t, adapter.User(c.Request().Context()))
		return c.NoContent(http.StatusNoContent)
	}
	resolve := restAuthResolver(s.cfg)
	s.e.GET("/api/outside-required", handler, echo.WrapMiddleware(jwtm), adapterhttp.RequiredAuth(resolve))
	s.e.GET("/api/outside-optional", handler, echo.WrapMiddleware(jwtm), adapterhttp.OptionalAuth(resolve))

	for kind := range s.kinds {
		rr := s.do(http.MethodGet, "/api/outside-required", "", s.tokenFor(kind))
		assert.Equal(t, http.StatusUnauthorized, rr.Code, kind)
		assert.False(t, reached, kind)

		rr = s.do(http.MethodGet, "/api/outside-optional", "", s.tokenFor(kind))
		assert.Equal(t, http.StatusNoContent, rr.Code, kind)
		assert.True(t, reached, kind)
		reached = false
	}
}
