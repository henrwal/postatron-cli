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
		case r.URL.Path == "/v1/profiles":
			_ = json.NewEncoder(w).Encode(apiv1.ProfileList{Data: []apiv1.Profile{{ID: "default", Name: "Default", IsDefault: true}}})
		case r.URL.Path == "/v1/analytics":
			_ = json.NewEncoder(w).Encode(apiv1.AnalyticsReport{Range: apiv1.AnalyticsRange{Key: "7d"}})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/media/uploads":
			w.WriteHeader(http.StatusCreated)
			_ = json.NewEncoder(w).Encode(apiv1.Upload{ID: "u1", Status: "PENDING"})
		case r.URL.Path == "/v1/media/uploads/u1":
			_ = json.NewEncoder(w).Encode(apiv1.Upload{ID: "u1", Status: "READY"})
		case r.URL.Path == "/v1/usage":
			_ = json.NewEncoder(w).Encode(apiv1.UsageReport{Plan: apiv1.UsagePlan{Code: "STARTER"}})
		case r.Method == http.MethodPost && r.URL.Path == "/v1/posts/p1/retry":
			var in apiv1.RetryPostRequest
			_ = json.NewDecoder(r.Body).Decode(&in)
			_ = json.NewEncoder(w).Encode(apiv1.Post{ID: "p1", Status: apiv1.StatusProcessing, Deliveries: []apiv1.Delivery{{AccountID: in.AccountIDs[0], Status: apiv1.StatusPending}}})
		case r.URL.Path == "/v1/events":
			_ = json.NewEncoder(w).Encode(apiv1.EventList{Data: []apiv1.Event{{ID: "e1", Kind: "post.failed", Status: apiv1.EventStatusError}}, NextCursor: "e0"})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer server.Close()

	client := apiv1.NewClient(server.URL+"/", "ptn_test")
	ctx := context.Background()

	when := time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC)
	post, err := client.CreatePost(ctx, apiv1.CreatePostRequest{
		Content: "hi", Platforms: []string{"x"}, Profile: "Acme",
		MediaURLs: []string{"https://example.com/a.jpg"}, ScheduledAt: &when,
	})
	require.NoError(t, err)
	assert.Equal(t, "p1", post.ID)
	assert.Equal(t, "Bearer ptn_test", gotAuth)
	assert.Equal(t, "hi", gotBody.Content)
	assert.Equal(t, []string{"x"}, gotBody.Platforms)
	assert.Equal(t, "Acme", gotBody.Profile)
	assert.Equal(t, []string{"https://example.com/a.jpg"}, gotBody.MediaURLs)

	from := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	list, err := client.ListPosts(ctx, apiv1.ListPostsQuery{Status: "scheduled", Platform: "x", Profile: "acme", From: &from, Limit: 10, Cursor: "c1"})
	require.NoError(t, err)
	assert.Equal(t, "c2", list.NextCursor)
	assert.Equal(t, "cursor=c1&from=2026-09-01T00%3A00%3A00Z&limit=10&platform=x&profile=acme&status=scheduled", gotQuery)

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

	profiles, err := client.ListProfiles(ctx)
	require.NoError(t, err)
	assert.True(t, profiles.Data[0].IsDefault)

	report, err := client.GetAnalytics(ctx, apiv1.AnalyticsQuery{Range: "7d", Profile: "Acme"})
	require.NoError(t, err)
	assert.Equal(t, "7d", report.Range.Key)
	assert.Equal(t, "profile=Acme&range=7d", gotQuery)

	upload, err := client.CreateUpload(ctx, apiv1.CreateUploadRequest{Purpose: "launch photo"})
	require.NoError(t, err)
	assert.Equal(t, "PENDING", upload.Status)
	upload, err = client.GetUpload(ctx, "u1")
	require.NoError(t, err)
	assert.Equal(t, "READY", upload.Status)

	usage, err := client.GetUsage(ctx)
	require.NoError(t, err)
	assert.Equal(t, "STARTER", usage.Plan.Code)

	retried, err := client.RetryPost(ctx, "p1", apiv1.RetryPostRequest{AccountIDs: []string{"ig1"}})
	require.NoError(t, err)
	assert.Equal(t, apiv1.StatusProcessing, retried.Status)
	assert.Equal(t, "ig1", retried.Deliveries[0].AccountID)
	assert.Equal(t, http.MethodPost, gotMethod)

	events, err := client.ListEvents(ctx, apiv1.ListEventsQuery{Range: "24h", Status: "error", Type: "posts", Limit: 5})
	require.NoError(t, err)
	assert.Equal(t, "e0", events.NextCursor)
	assert.Equal(t, "limit=5&range=24h&status=error&type=posts", gotQuery)
}

// The fields added for failures read back as an agent sees them.
func TestDeliveryAndPostCarryWhyTheyFailed(t *testing.T) {
	due := time.Date(2026, 10, 20, 9, 0, 0, 0, time.UTC)
	raw, err := json.Marshal(apiv1.Post{ID: "p1", Unpublished: &apiv1.PostUnpublished{Reason: apiv1.UnpublishedNoPlan, DueAt: due},
		Deliveries: []apiv1.Delivery{
			{AccountID: "ig1", Status: apiv1.StatusFailed, Error: "Reconnect it.", ErrorCode: apiv1.ErrorReconnect},
			{AccountID: "yt1", Status: apiv1.StatusPublished, Error: "The thumbnail was not set.", Warning: true},
			{AccountID: "x1", Status: apiv1.StatusPublished},
		}})
	require.NoError(t, err)
	assert.Contains(t, string(raw), `"unpublished":{"reason":"no_plan","due_at":"2026-10-20T09:00:00Z"}`)
	assert.Contains(t, string(raw), `"error_code":"reconnect"`)
	assert.Contains(t, string(raw), `"warning":true`)
	assert.Contains(t, string(raw), `{"account_id":"x1","platform":"","username":"","status":"published"}`, "a clean delivery carries neither")
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
