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
	created []apiv1.CreatePostRequest
	listed  []apiv1.ListPostsQuery
	deleted []string
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

func (f *fakeOps) DeletePost(_ context.Context, id string) (*apiv1.DeletePostResponse, error) {
	f.deleted = append(f.deleted, id)
	if id == "published" {
		return &apiv1.DeletePostResponse{ID: id, Status: "published", Cancelled: false, Message: "already out"}, nil
	}
	return &apiv1.DeletePostResponse{ID: id, Status: "cancelled", Cancelled: true}, nil
}

func (f *fakeOps) ListAccounts(_ context.Context) (*apiv1.AccountList, error) {
	return &apiv1.AccountList{Data: []apiv1.Account{{ID: "a1", Platform: "x", Username: "henry", Status: "connected", ConnectedAt: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)}}}, nil
}

func (f *fakeOps) GetUsage(_ context.Context) (*apiv1.UsageReport, error) {
	return &apiv1.UsageReport{
		Period:           apiv1.UsagePeriod{Start: time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC), End: time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC)},
		Plan:             apiv1.UsagePlan{Code: "STARTER", Name: "Starter", Interval: "month", Status: "ACTIVE", ConnectedAccounts: 2, MaxConnectedAccounts: 5},
		DestinationPosts: apiv1.UsageCounter{Used: 12, Limit: 300, Remaining: 288},
		XLinkPosts:       apiv1.UsageCounter{Used: 1, Limit: 10, Remaining: 9},
		ByPlatform:       map[string]int{"x": 7, "linkedin": 5},
		RateLimits:       apiv1.UsageRateLimits{WritesPerHour: 60, ReadsPerMinute: 60},
	}, nil
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

func TestSixCommands(t *testing.T) {
	root, _ := newRootCommand(&bytes.Buffer{}, &bytes.Buffer{})
	names := make([]string, 0)
	for _, cmd := range root.Commands() {
		if cmd.Name() == "help" || cmd.Name() == "completion" {
			continue
		}
		names = append(names, cmd.Name())
	}
	assert.ElementsMatch(t, []string{"create-post", "list-posts", "get-post", "delete-post", "list-accounts", "get-usage"}, names)
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

	out, err = run(t, ops, "get-usage")
	require.NoError(t, err)
	assert.Contains(t, out, "Destination-posts: 12 / 300 (288 remaining)")
	assert.Contains(t, out, "linkedin  5")
}

func TestExitCodes(t *testing.T) {
	assert.Equal(t, 3, exitCode(&apiv1.APIError{Code: apiv1.CodeUnauthorized}))
	assert.Equal(t, 4, exitCode(&apiv1.APIError{Code: apiv1.CodeQuotaExceeded}))
	assert.Equal(t, 1, exitCode(&apiv1.APIError{Code: apiv1.CodeInternal}))
}
