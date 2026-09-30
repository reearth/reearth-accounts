package auth0

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// mfaClient mocks the Auth0 endpoints touched by GetMFAStatus/EnableMFA/DisableMFA
// and counts how many times the enrollments list endpoint is hit, so tests can
// assert that the per-sub cache actually bounds Management API traffic.
func mfaClient(enrolledSub string, enrollmentCalls *int32) *http.Client {
	return &http.Client{
		Transport: RoundTripFunc(func(req *http.Request) *http.Response {
			p := req.URL.Path

			if req.Method == http.MethodPost && p == "/oauth/token" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body: res(map[string]interface{}{
						"access_token": token,
						"expires_in":   expiresIn,
					}),
					Header: make(http.Header),
				}
			}

			if req.Method == http.MethodGet && strings.HasSuffix(p, "/enrollments") {
				atomic.AddInt32(enrollmentCalls, 1)
				sub := strings.TrimSuffix(strings.TrimPrefix(p, "/api/v2/users/"), "/enrollments")
				var enrollments []map[string]string
				if sub == enrolledSub {
					enrollments = []map[string]string{{"id": "enr-1", "status": "confirmed"}}
				}
				return &http.Response{StatusCode: http.StatusOK, Body: res(enrollments), Header: make(http.Header)}
			}

			if req.Method == http.MethodDelete && strings.HasPrefix(p, "/api/v2/guardian/enrollments/") {
				return &http.Response{StatusCode: http.StatusOK, Body: res(map[string]interface{}{}), Header: make(http.Header)}
			}

			if req.Method == http.MethodPatch && strings.HasPrefix(p, "/api/v2/users/") {
				return &http.Response{StatusCode: http.StatusOK, Body: res(map[string]interface{}{}), Header: make(http.Header)}
			}

			if req.Method == http.MethodPost && p == "/api/v2/guardian/enrollments/ticket" {
				return &http.Response{
					StatusCode: http.StatusOK,
					Body:       res(map[string]interface{}{"ticket_url": "https://example.com/ticket"}),
					Header:     make(http.Header),
				}
			}

			return &http.Response{StatusCode: http.StatusNotFound, Body: res(map[string]interface{}{"message": "not found"}), Header: make(http.Header)}
		}),
	}
}

func TestAuth0_GetMFAStatus_CachesWithinTTL(t *testing.T) {
	var calls int32
	sub := "mfa-enrolled-sub"

	a := New(domain, clientID, clientSecret, 0)
	a.client = mfaClient(sub, &calls)
	a.current = func() time.Time { return current }
	a.disableLogging = true

	status, err := a.GetMFAStatus(context.Background(), sub)
	assert.NoError(t, err)
	assert.True(t, status.Enrolled)
	assert.EqualValues(t, 1, atomic.LoadInt32(&calls))

	status, err = a.GetMFAStatus(context.Background(), sub)
	assert.NoError(t, err)
	assert.True(t, status.Enrolled)
	assert.EqualValues(t, 1, atomic.LoadInt32(&calls), "second call within TTL should be served from cache")
}

func TestAuth0_GetMFAStatus_RefetchesAfterTTLExpires(t *testing.T) {
	var calls int32
	sub := "mfa-not-enrolled-sub"

	a := New(domain, clientID, clientSecret, 0)
	a.client = mfaClient(sub, &calls)
	now := current
	a.current = func() time.Time { return now }
	a.disableLogging = true

	_, err := a.GetMFAStatus(context.Background(), sub)
	assert.NoError(t, err)
	assert.EqualValues(t, 1, atomic.LoadInt32(&calls))

	now = now.Add(mfaStatusCacheTTL + time.Second)
	_, err = a.GetMFAStatus(context.Background(), sub)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, atomic.LoadInt32(&calls), "cache entry should have expired")
}

func TestAuth0_DisableMFA_UpdatesCacheWithoutRefetch(t *testing.T) {
	var calls int32
	sub := "mfa-enrolled-sub"

	a := New(domain, clientID, clientSecret, 0)
	a.client = mfaClient(sub, &calls)
	a.current = func() time.Time { return current }
	a.disableLogging = true

	status, err := a.GetMFAStatus(context.Background(), sub)
	assert.NoError(t, err)
	assert.True(t, status.Enrolled)
	assert.EqualValues(t, 1, atomic.LoadInt32(&calls))

	assert.NoError(t, a.DisableMFA(context.Background(), sub))

	status, err = a.GetMFAStatus(context.Background(), sub)
	assert.NoError(t, err)
	assert.False(t, status.Enrolled, "disabling mfa should update the cached status")
	assert.EqualValues(t, 2, atomic.LoadInt32(&calls), "status after disable should come from cache, not another enrollments call")
}

func TestAuth0_EnableMFA_InvalidatesCache(t *testing.T) {
	var calls int32
	sub := "mfa-not-enrolled-sub"

	a := New(domain, clientID, clientSecret, 0)
	a.client = mfaClient("some-other-enrolled-sub", &calls)
	a.current = func() time.Time { return current }
	a.disableLogging = true

	status, err := a.GetMFAStatus(context.Background(), sub)
	assert.NoError(t, err)
	assert.False(t, status.Enrolled)
	assert.EqualValues(t, 1, atomic.LoadInt32(&calls))

	_, err = a.EnableMFA(context.Background(), sub)
	assert.NoError(t, err)

	_, err = a.GetMFAStatus(context.Background(), sub)
	assert.NoError(t, err)
	assert.EqualValues(t, 2, atomic.LoadInt32(&calls), "enabling mfa should invalidate the cache and force a refetch")
}
