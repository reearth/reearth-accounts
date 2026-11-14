package user_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/jarcoal/httpmock"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/stretchr/testify/require"
)

func TestUserRepo_IDLookupsPreserveAlias(t *testing.T) {
	const uid = "01j9x0yy00000000000000000a"
	for _, batch := range []bool{false, true} {
		for _, email := range []string{"", "other@example.com"} {
			name := "single"
			if batch {
				name = "batch"
			}
			t.Run(name+"/"+email, func(t *testing.T) {
				transport := httpmock.NewMockTransport()
				client := gqlclient.NewClient("https://accounts.example.com", 30, transport)
				usr := map[string]any{
					"id": uid, "name": "Display Name", "alias": "user-alias", "email": email,
					"workspace": "01j9x0yy00000000000000001a", "auths": []string{}, "metadata": map[string]any{},
				}
				data := map[string]any{"user": usr}
				if batch {
					data = map[string]any{"findUsersByIDs": []any{usr}}
				}
				body, err := json.Marshal(map[string]any{"data": data})
				require.NoError(t, err)
				transport.RegisterResponder("POST", "https://accounts.example.com/api/graphql",
					httpmock.NewStringResponder(http.StatusOK, string(body)))

				var got *user.User
				if batch {
					users, findErr := client.UserRepo.FindByIDs(context.Background(), []string{uid})
					require.NoError(t, findErr)
					require.Len(t, users, 1)
					got = users[0]
				} else {
					got, err = client.UserRepo.FindByID(context.Background(), uid)
					require.NoError(t, err)
				}
				require.NotNil(t, got)
				require.Equal(t, uid, got.ID().String())
				require.Equal(t, "Display Name", got.Name())
				require.Equal(t, "user-alias", got.Alias())
				require.Equal(t, email, got.Email())
				require.Equal(t, 1, transport.GetTotalCallCount())
			})
		}
	}
}
