package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/log"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/text/language"
)

// Values only the demo user may see. None of them may reach a log entry.
var logProbeSelfValues = []string{
	"log-probe-self@example.com",
	"log-probe-description",
	"https://log-probe.example.com",
	"auth0|log-probe-subject",
}

const logProbeOtherEmail = "other-rest@example.com"

func seedLogProbe(ctx context.Context, r *repo.Container) error {
	if err := seedDemoUser(ctx, r); err != nil {
		return err
	}
	u, err := r.User.FindByID(ctx, restDemoUID)
	if err != nil {
		return err
	}
	if err := u.UpdateEmail(logProbeSelfValues[0]); err != nil {
		return err
	}
	u.AddAuth(user.Auth{Provider: "auth0", Sub: logProbeSelfValues[3]})
	u.SetMetadata(user.MetadataFrom("", logProbeSelfValues[1], logProbeSelfValues[2], language.Japanese, user.ThemeDark))
	return r.User.Save(ctx, u)
}

type syncBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

func captureLogs(t *testing.T) *syncBuffer {
	t.Helper()
	buf := &syncBuffer{}
	log.SetOutput(buf)
	t.Cleanup(func() { log.SetOutput(os.Stdout) })
	return buf
}

// requestEntries returns the request log lines, one per request.
func requestEntries(out string) []string {
	var res []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "\trequest\t") {
			res = append(res, l)
		}
	}
	return res
}

func TestRequestLog_NoSelfFieldValueReachesTheLog(t *testing.T) {
	e, _ := StartServer(t, &app.Config{Mock_Auth: true, Dev: true}, false, seedLogProbe)
	logs := captureLogs(t)

	b, err := json.Marshal(GraphQLRequest{
		OperationName: "Probe",
		Query: `query Probe($q: String!) {
			me { id email auths metadata { description website } }
			byEmail: userByNameOrEmail(nameOrEmail: $q) { id }
		}`,
		Variables: map[string]any{"q": logProbeOtherEmail},
	})
	require.NoError(t, err)
	gqlBody := e.POST("/api/graphql").WithHeader("Content-Type", "application/json").
		WithHeader("X-Internal-Service", "visualizer").WithBytes(b).
		Expect().Status(http.StatusOK).Body().Raw()

	restBody := e.GET("/api/users/me").Expect().Status(http.StatusOK).Body().Raw()
	e.GET("/api/users/by-name-or-email").WithQuery("q", logProbeOtherEmail).Expect().Status(http.StatusOK)

	for _, v := range logProbeSelfValues[:3] {
		require.Contains(t, gqlBody+restBody, v, "the response must carry the value for this test to mean anything")
	}

	out := logs.String()
	require.Len(t, requestEntries(out), 3, out)
	for _, v := range append(logProbeSelfValues, logProbeOtherEmail) {
		assert.NotContains(t, out, v)
	}
}

func TestRequestLog_GraphQLOperationAndTopLevelFields(t *testing.T) {
	e, _ := StartServer(t, &app.Config{Mock_Auth: true}, false, seedDemoUser)
	logs := captureLogs(t)

	b, err := json.Marshal(GraphQLRequest{
		OperationName: "GetMe",
		Query:         `query GetMe { me { id } ...Ws } fragment Ws on Query { aliased: findByUser(userId: "` + restDemoUID.String() + `") { id } }`,
	})
	require.NoError(t, err)
	e.POST("/api/graphql").WithHeader("Content-Type", "application/json").
		WithHeader("X-Internal-Service", "reearth-cms").WithBytes(b).
		Expect().Status(http.StatusOK)

	entries := requestEntries(logs.String())
	require.Len(t, entries, 1, logs.String())
	entry := entries[0]
	assert.Contains(t, entry, `"route": "/api/graphql"`)
	assert.Contains(t, entry, `"operation": "GetMe"`)
	assert.Contains(t, entry, `"operation_type": "query"`)
	assert.Contains(t, entry, `"fields": ["me", "findByUser"]`)
	assert.Contains(t, entry, `"status": 200`)
	assert.Contains(t, entry, `"latency_ms": `)
	assert.Contains(t, entry, `"viewer_id": "`+restDemoUID.String()+`"`)
	assert.Contains(t, entry, `"caller": "reearth-cms"`)
	assert.Contains(t, entry, `"request_id": "`)
	assert.NotContains(t, entry, "aliased")
}

func TestRequestLog_RESTRouteAndStatus(t *testing.T) {
	e, _ := StartServer(t, &app.Config{Mock_Auth: true}, false, seedDemoUser)
	logs := captureLogs(t)

	e.GET("/api/users/" + restOtherUID.String()).Expect().Status(http.StatusOK)
	e.GET("/api/users/not-an-id").Expect().Status(http.StatusBadRequest)

	entries := requestEntries(logs.String())
	require.Len(t, entries, 2, logs.String())
	for i, status := range []string{`"status": 200`, `"status": 400`} {
		assert.Contains(t, entries[i], `"route": "/api/users/:id"`)
		assert.Contains(t, entries[i], `"method": "GET"`)
		assert.Contains(t, entries[i], status)
		assert.NotContains(t, entries[i], restOtherUID.String(), "the route template is logged, not the path")
	}
}
