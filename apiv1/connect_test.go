package apiv1_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/henrwal/postatron-cli/apiv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// Profiles and connect sessions go to their routes, with names escaped into
// the path, since a profile may be named rather than given by id.
func TestProfileAndConnectSessionCalls(t *testing.T) {
	type call struct{ method, path, body string }
	var calls []call
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, _ := io.ReadAll(r.Body)
		calls = append(calls, call{r.Method, r.URL.EscapedPath(), string(raw)})
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/profiles":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(apiv1.Profile{ID: "8f2c1a9e", Name: "Acme Ltd", Accounts: []apiv1.Account{}})
		case r.Method == http.MethodPatch && r.URL.Path == "/v1/profiles/Acme Ltd":
			_ = json.NewEncoder(w).Encode(apiv1.Profile{ID: "8f2c1a9e", Name: "Acme Studio", Accounts: []apiv1.Account{}})
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/profiles/8f2c1a9e":
			_ = json.NewEncoder(w).Encode(apiv1.DeleteProfileResponse{ID: "8f2c1a9e", Deleted: true})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/connect/sessions":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(apiv1.ConnectSession{ID: "cs_1", URL: "https://postatron.com/connect/ptc_x", Status: apiv1.ConnectSessionPending})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/connect/sessions/cs_1":
			_ = json.NewEncoder(w).Encode(apiv1.ConnectSession{ID: "cs_1", Status: apiv1.ConnectSessionConnected,
				Account: &apiv1.Account{ID: "ig1", Platform: "instagram"}})
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/profiles/Busy":
			w.WriteHeader(http.StatusConflict)
			_ = json.NewEncoder(w).Encode(apiv1.ErrorBody{Error: apiv1.ErrorDetail{Code: apiv1.CodeConflict, Message: "the Busy profile still has accounts"}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := apiv1.NewClient(server.URL, "ptn_test")
	ctx := context.Background()

	profile, err := client.CreateProfile(ctx, apiv1.CreateProfileRequest{Name: "Acme Ltd"})
	require.NoError(t, err)
	assert.Equal(t, "8f2c1a9e", profile.ID)
	assert.JSONEq(t, `{"name":"Acme Ltd"}`, calls[0].body)

	profile, err = client.UpdateProfile(ctx, "Acme Ltd", apiv1.UpdateProfileRequest{Name: "Acme Studio"})
	require.NoError(t, err)
	assert.Equal(t, "Acme Studio", profile.Name)
	assert.Equal(t, "/v1/profiles/Acme%20Ltd", calls[1].path)

	deleted, err := client.DeleteProfile(ctx, "8f2c1a9e")
	require.NoError(t, err)
	assert.True(t, deleted.Deleted)

	session, err := client.CreateConnectSession(ctx, apiv1.CreateConnectSessionRequest{
		Profile: "Acme Ltd", Platforms: []string{"instagram"}, RedirectURL: "https://app.example.com/connected",
	})
	require.NoError(t, err)
	assert.Equal(t, "https://postatron.com/connect/ptc_x", session.URL)
	assert.JSONEq(t, `{"profile":"Acme Ltd","platforms":["instagram"],"redirect_url":"https://app.example.com/connected"}`, calls[3].body)

	session, err = client.GetConnectSession(ctx, "cs_1")
	require.NoError(t, err)
	assert.Equal(t, apiv1.ConnectSessionConnected, session.Status)
	require.NotNil(t, session.Account)
	assert.Equal(t, "ig1", session.Account.ID)

	_, err = client.DeleteProfile(ctx, "Busy")
	var apiErr *apiv1.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, apiv1.CodeConflict, apiErr.Code)
}
