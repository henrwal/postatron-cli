package apiv1_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/henrwal/postatron-cli/apiv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestClientRoundTrips(t *testing.T) {
	var gotAuth, gotPath, gotQuery, gotMethod string
	var gotBody apiv1.CreatePostRequest

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotQuery = r.URL.RawQuery
		gotMethod = r.Method
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodPost && r.URL.Path == "/v1/posts":
			_ = json.NewDecoder(r.Body).Decode(&gotBody)
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(apiv1.Post{ID: "p1", Status: "scheduled"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/posts":
			_ = json.NewEncoder(w).Encode(apiv1.PostList{Data: []apiv1.Post{{ID: "p1"}}, NextCursor: "c2"})
		case r.Method == http.MethodGet && r.URL.Path == "/v1/posts/p1":
			_ = json.NewEncoder(w).Encode(apiv1.Post{ID: "p1"})
		case r.Method == http.MethodDelete && r.URL.Path == "/v1/posts/p1":
			_ = json.NewEncoder(w).Encode(apiv1.DeletePostResponse{ID: "p1", Cancelled: true, Status: "cancelled"})
		case r.URL.Path == "/v1/accounts":
			_ = json.NewEncoder(w).Encode(apiv1.AccountList{Data: []apiv1.Account{{ID: "a1", Platform: "x"}}})
		case r.URL.Path == "/v1/usage":
			_ = json.NewEncoder(w).Encode(apiv1.UsageReport{Plan: apiv1.UsagePlan{Code: "STARTER"}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := apiv1.NewClient(server.URL+"/", "ptn_test")
	ctx := context.Background()

	when := time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC)
	post, err := client.CreatePost(ctx, apiv1.CreatePostRequest{Content: "hi", Platforms: []string{"x"}, ScheduledAt: &when})
	require.NoError(t, err)
	assert.Equal(t, "p1", post.ID)
	assert.Equal(t, "Bearer ptn_test", gotAuth)
	assert.Equal(t, "hi", gotBody.Content)
	assert.Equal(t, []string{"x"}, gotBody.Platforms)

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	list, err := client.ListPosts(ctx, apiv1.ListPostsQuery{Status: "scheduled", Platform: "x", From: &from, Limit: 10, Cursor: "c1"})
	require.NoError(t, err)
	assert.Equal(t, "c2", list.NextCursor)
	assert.Equal(t, "cursor=c1&from=2026-09-01T00%3A00%3A00Z&limit=10&platform=x&status=scheduled", gotQuery)

	got, err := client.GetPost(ctx, "p1")
	require.NoError(t, err)
	assert.Equal(t, "p1", got.ID)
	assert.Equal(t, "/v1/posts/p1", gotPath)

	deleted, err := client.DeletePost(ctx, "p1")
	require.NoError(t, err)
	assert.True(t, deleted.Cancelled)
	assert.Equal(t, http.MethodDelete, gotMethod)

	accounts, err := client.ListAccounts(ctx)
	require.NoError(t, err)
	assert.Len(t, accounts.Data, 1)

	usage, err := client.GetUsage(ctx)
	require.NoError(t, err)
	assert.Equal(t, "STARTER", usage.Plan.Code)
}

func TestClientDecodesErrors(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "37")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(apiv1.ErrorBody{Error: apiv1.ErrorDetail{Code: apiv1.CodeRateLimited, Message: "slow down", Details: map[string]any{"limit": 60}}})
	}))
	defer server.Close()

	client := apiv1.NewClient(server.URL, "ptn_test")
	_, err := client.GetUsage(context.Background())
	require.Error(t, err)

	var apiErr *apiv1.APIError
	require.True(t, errors.As(err, &apiErr))
	assert.Equal(t, http.StatusTooManyRequests, apiErr.Status)
	assert.Equal(t, apiv1.CodeRateLimited, apiErr.Code)
	assert.Equal(t, "slow down", apiErr.Message)
	assert.Equal(t, 37*time.Second, apiErr.RetryAfter)
	assert.Equal(t, "rate_limited: slow down", apiErr.Error())

	noKey := apiv1.NewClient(server.URL, "")
	_, err = noKey.GetUsage(context.Background())
	require.ErrorContains(t, err, "no API key configured")
}
