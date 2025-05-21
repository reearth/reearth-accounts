//go:generate go run github.com/Khan/genqlient
package proxy

import (
	"net/http"

	"github.com/Khan/genqlient/graphql"
)

type HTTPClient = graphql.Doer

type httpClient struct {
	c     graphql.Doer
	token string
}

func NewHTTPClient(c graphql.Doer, token string) *httpClient {
	return &httpClient{c: c, token: token}
}

func (c *httpClient) Do(req *http.Request) (*http.Response, error) {
	h := req.Header
	if h.Get("Authorization") == "" {
		h.Set("Authozation", "Bearer "+c.token)
	}
	if c == nil || c.c == nil {
		return http.DefaultClient.Do(req)
	}
	return c.c.Do(req)
}
