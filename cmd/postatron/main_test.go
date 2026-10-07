package main

import (
	"bytes"
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/henrwal/postatron-cli/apiv1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeOps struct {
	updated  []apiv1.UpdatePostRequest
	connects []apiv1.ConnectAccountRequest
	created  []apiv1.CreatePostRequest
	listed   []apiv1.ListPostsQuery
	deleted  []string
	analysed []apiv1.AnalyticsQuery
	purposes []string
}

func (f *fakeOps) CreatePost(_ context.Context, req apiv1.CreatePostRequest) (*apiv1.Post, error) {
	f.created = append(f.created, req)
	return &apiv1.Post{ID: "p1", Status: "pending", Content: req.Content, CreatedAt: time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC),
		Deliveries: []apiv1.Delivery{{Platform: "x", Username: "henry", Status: "pending"}}}, nil
}

func (f *fakeOps) ListPosts(_ context.Context, q apiv1.ListPostsQuery) (*apiv1.PostList, error) {
	f.listed = append(f.listed, q)
	return &apiv1.PostList{Data: []apiv1.Post{{ID: "p1", Status: "published", Content: "hello world", CreatedAt: time.Date(2026, 9, 5, 10, 0, 0, 0, time.UTC),
		Deliveries: []apiv1.Delivery{{Platform: "x"}, {Platform: "linkedin"}}}}, NextCursor: "abc"}, nil
}

func (f *fakeOps) GetPost(_ context.Context, id string) (*apiv1.Post, error) {
	return &apiv1.Post{ID: id, Status: "failed", Content: "x", CreatedAt: time.Unix(0, 0), Deliveries: []apiv1.Delivery{{Platform: "linkedin", Username: "h", Status: "failed", Error: "token expired"}}}, nil
}

func (f *fakeOps) UpdatePost(_ context.Context, id string, req apiv1.UpdatePostRequest) (*apiv1.Post, error) {
	f.updated = append(f.updated, req)
	return &apiv1.Post{ID: id, Status: "scheduled", Deliveries: []apiv1.Delivery{{Platform: "instagram", Username: "henry", Status: "pending"}}}, nil
}

func (f *fakeOps) ConnectAccount(_ context.Context, req apiv1.ConnectAccountRequest) (*apiv1.ConnectLink, error) {
	f.connects = append(f.connects, req)
	return &apiv1.ConnectLink{Platform: req.Platform, ConnectURL: "https://postatron.com/dashboard/socials?connect=" + req.Platform, Hint: "Open the link to connect."}, nil
}

func (f *fakeOps) DeletePost(_ context.Context, id string) (*apiv1.DeletePostResponse, error) {
	f.deleted = append(f.deleted, id)
	if id == "published" {
		return &apiv1.DeletePostResponse{ID: id, Status: "published", Cancelled: false, Message: "already out"}, nil
	}
	return &apiv1.DeletePostResponse{ID: id, Status: "cancelled", Cancelled: true}, nil
}

func (f *fakeOps) ListAccounts(_ context.Context) (*apiv1.AccountList, error) {
	connected := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	return &apiv1.AccountList{Data: []apiv1.Account{
		{ID: "a1", Platform: "x", Username: "henry", Status: "connected", ProfileID: "default", ProfileName: "Default", ConnectedAt: connected},
		{ID: "a2", Platform: "x", Username: "acmehq", Status: "connected", ProfileID: "p-acme", ProfileName: "Acme", ConnectedAt: connected},
	}}, nil
}

func (f *fakeOps) ListProfiles(_ context.Context) (*apiv1.ProfileList, error) {
	return &apiv1.ProfileList{Data: []apiv1.Profile{
		{ID: "default", Name: "Default", IsDefault: true, Accounts: []apiv1.Account{{Platform: "x", Username: "henry"}}},
		{ID: "p-acme", Name: "Acme", Accounts: []apiv1.Account{}},
	}}, nil
}

func (f *fakeOps) GetAnalytics(_ context.Context, q apiv1.AnalyticsQuery) (*apiv1.AnalyticsReport, error) {
	f.analysed = append(f.analysed, q)
	likes := int64(4)
	return &apiv1.AnalyticsReport{
		Range:       apiv1.AnalyticsRange{Key: "7d", Start: "2026-09-29T00:00:00Z", End: "2026-10-06T00:00:00Z"},
		Summary:     apiv1.AnalyticsSummary{EngagementRate: 3.25, EngagementRateBasis: "reach", Engagements: 40, PostsThisPeriod: 3},
		PerPlatform: []apiv1.AnalyticsPlatformRow{{Platform: "instagram", Posts: 3, Likes: 30}},
		TopPosts:    []apiv1.AnalyticsPost{{Platform: "instagram", PublishedAt: "2026-10-01T09:00:00Z", Text: "Launch", Likes: &likes, Engagement: 12}},
	}, nil
}

func (f *fakeOps) CreateUpload(_ context.Context, req apiv1.CreateUploadRequest) (*apiv1.Upload, error) {
	f.purposes = append(f.purposes, req.Purpose)
	return &apiv1.Upload{ID: "u1", Status: "PENDING", UploadURL: "https://postatron.com/dashboard/upload?u=u1", ExpiresAt: "2026-10-07T09:00:00Z"}, nil
}

func (f *fakeOps) GetUpload(_ context.Context, id string) (*apiv1.Upload, error) {
	return &apiv1.Upload{ID: id, Status: "READY", Filename: "shoe.jpg", ExpiresAt: "2026-10-07T09:00:00Z"}, nil
}

func (f *fakeOps) GetUsage(_ context.Context) (*apiv1.UsageReport, error) {
	return &apiv1.UsageReport{
		Period:           apiv1.UsagePeriod{Start: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
		Plan:             apiv1.UsagePlan{Code: "STARTER", Name: "Starter", Interval: "month", Status: "ACTIVE", ConnectedAccounts: 2, MaxConnectedAccounts: 5},
		DestinationPosts: apiv1.UsageCounter{Used: 12, Limit: 300, Remaining: 288},
		XLinkPosts:       apiv1.UsageCounter{Used: 1, Limit: 10, Remaining: 9},
		ByPlatform:       map[string]int{"x": 7, "linkedin": 5},
		RateLimits:       apiv1.UsageRateLimits{WritesPerMinute: 30, ReadsPerMinute: 60, UploadsPerMinute: 120, WritesPerHour: 1800},
	}, nil
}

// uncappedUsage reports X link posts the way the API does since they stopped
// being capped: a count with no limit.
type uncappedUsage struct{ *fakeOps }

func (u *uncappedUsage) GetUsage(ctx context.Context) (*apiv1.UsageReport, error) {
	report, err := u.fakeOps.GetUsage(ctx)
	if err != nil {
		return nil, err
	}
	report.XLinkPosts = apiv1.UsageCounter{Used: 1}
	return report, nil
}

func run(t *testing.T, ops apiv1.Operations, args ...string) (string, error) {
	t.Helper()
	var stdout, stderr bytes.Buffer
	root, g := newRootCommand(&stdout, &stderr)
	g.client = ops
	root.SetArgs(args)
	err := root.Execute()
	return stdout.String(), err
}

func TestCommands(t *testing.T) {
	root, _ := newRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	names := make([]string, 0)
	for _, cmd := range root.Commands() {
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			continue
		}
		names = append(names, cmd.Name())
	}
	assert.ElementsMatch(t, []string{
		"create-post", "list-posts", "get-post", "delete-post", "list-accounts", "list-profiles", "get-usage", "get-analytics",
		"create-upload", "get-upload", "update-post", "connect-account",
	}, names)
}

func TestUpdatePost(t *testing.T) {
	ops := &fakeOps{}
	_, err := run(t, ops, "update-post", "p_1", "--add-platforms", "instagram", "--profile", "Acme")
	require.NoError(t, err)
	require.Len(t, ops.updated, 1)
	assert.Equal(t, []string{"instagram"}, ops.updated[0].AddPlatforms)
	assert.Equal(t, "Acme", ops.updated[0].Profile)
	assert.Nil(t, ops.updated[0].Content, "text left alone unless --content is given")

	_, err = run(t, ops, "update-post", "p_1", "--content", "")
	require.NoError(t, err)
	require.NotNil(t, ops.updated[1].Content, "an explicit --content is sent even when empty, so the API can refuse it")

	_, err = run(t, ops, "update-post", "p_1", "--scheduled-at", "soon")
	require.Error(t, err)
}

func TestConnectAccount(t *testing.T) {
	ops := &fakeOps{}
	out, err := run(t, ops, "connect-account", "x")
	require.NoError(t, err)
	assert.Contains(t, out, "https://postatron.com/dashboard/socials?connect=x")
	assert.Equal(t, "x", ops.connects[0].Platform)
}

func TestCreatePost(t *testing.T) {
	ops := &fakeOps{}
	out, err := run(t, ops, "create-post", "--content", "Launch day!", "--platforms", "x,linkedin", "--scheduled-at", "2026-09-10T09:00:00Z")
	require.NoError(t, err)
	require.Len(t, ops.created, 1)
	assert.Equal(t, []string{"x", "linkedin"}, ops.created[0].Platforms)
	assert.Equal(t, time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC), ops.created[0].ScheduledAt.UTC())
	assert.Contains(t, out, "Post p1  [pending]")
	assert.Contains(t, out, "@henry")

	_, err = run(t, ops, "create-post", "--content", "For Acme", "--platforms", "instagram", "--profile", "Acme",
		"--media-urls", "https://example.com/a.jpg,https://example.com/b.jpg", "--media-ids", "u1")
	require.NoError(t, err)
	require.Len(t, ops.created, 2)
	assert.Equal(t, "Acme", ops.created[1].Profile)
	assert.Equal(t, []string{"https://example.com/a.jpg", "https://example.com/b.jpg"}, ops.created[1].MediaURLs)
	assert.Equal(t, []string{"u1"}, ops.created[1].MediaIDs)

	_, err = run(t, ops, "create-post", "--content", "x", "--scheduled-at", "soon")
	require.Error(t, err)
	assert.Equal(t, 2, exitCode(err))

	_, err = run(t, ops, "create-post")
	require.Error(t, err, "content is required")
}

func TestJSONOutput(t *testing.T) {
	out, err := run(t, &fakeOps{}, "--json", "get-usage")
	require.NoError(t, err)
	var report apiv1.UsageReport
	require.NoError(t, json.Unmarshal([]byte(out), &report))
	assert.Equal(t, 288, report.DestinationPosts.Remaining)
}

func TestHumanOutput(t *testing.T) {
	ops := &fakeOps{}
	out, err := run(t, ops, "list-posts", "--status", "published", "--limit", "5")
	require.NoError(t, err)
	assert.Equal(t, "published", ops.listed[0].Status)
	assert.Equal(t, 5, ops.listed[0].Limit)
	assert.Contains(t, out, "x,linkedin")
	assert.Contains(t, out, "--cursor abc")

	out, err = run(t, ops, "get-post", "p7")
	require.NoError(t, err)
	assert.Contains(t, out, "token expired")

	out, err = run(t, ops, "delete-post", "published")
	require.NoError(t, err)
	assert.Contains(t, out, "Not cancelled")

	out, err = run(t, ops, "list-accounts")
	require.NoError(t, err)
	assert.Contains(t, out, "henry")
	assert.Contains(t, out, "PROFILE")

	out, err = run(t, ops, "list-accounts", "--profile", "acme")
	require.NoError(t, err)
	assert.Contains(t, out, "acmehq", "profile names match regardless of case")
	assert.NotContains(t, out, "henry ")

	out, err = run(t, ops, "list-profiles")
	require.NoError(t, err)
	assert.Contains(t, out, "x:henry")
	assert.Contains(t, out, "(none)")

	out, err = run(t, ops, "get-analytics", "--range", "7d", "--profile", "Acme")
	require.NoError(t, err)
	assert.Equal(t, apiv1.AnalyticsQuery{Range: "7d", Profile: "Acme"}, ops.analysed[0])
	assert.Contains(t, out, "Engagement rate: 3.25% (reach)")
	assert.Contains(t, out, "Launch")

	out, err = run(t, ops, "get-usage")
	require.NoError(t, err)
	assert.Contains(t, out, "Destination-posts: 12 / 300 (288 remaining)")
	assert.Contains(t, out, "30 writes, 60 reads, 120 uploads per minute")
	assert.Contains(t, out, "X link posts:      1 / 10 (9 remaining)", "a capped counter keeps its limit")

	uncapped := &uncappedUsage{fakeOps: ops}
	out, err = run(t, uncapped, "get-usage")
	require.NoError(t, err)
	assert.Contains(t, out, "X link posts:      1 (no cap)")
	assert.Contains(t, out, "linkedin  5")
}

func TestExitCodes(t *testing.T) {
	assert.Equal(t, 3, exitCode(&apiv1.APIError{Code: apiv1.CodeUnauthorized}))
	assert.Equal(t, 4, exitCode(&apiv1.APIError{Code: apiv1.CodeQuotaExceeded}))
	assert.Equal(t, 1, exitCode(&apiv1.APIError{Code: apiv1.CodeInternal}))
}

func TestUploadCommands(t *testing.T) {
	ops := &fakeOps{}
	out, err := run(t, ops, "create-upload", "--purpose", "launch photo")
	require.NoError(t, err)
	assert.Equal(t, []string{"launch photo"}, ops.purposes)
	assert.Contains(t, out, "Link:    https://postatron.com/dashboard/upload?u=u1")

	out, err = run(t, ops, "get-upload", "u1")
	require.NoError(t, err)
	assert.Contains(t, out, "Upload u1  [READY]")
	assert.Contains(t, out, "shoe.jpg")
}
