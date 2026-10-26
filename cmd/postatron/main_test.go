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
	updated      []apiv1.UpdatePostRequest
	connects     []apiv1.ConnectAccountRequest
	created      []apiv1.CreatePostRequest
	listed       []apiv1.ListPostsQuery
	deleted      []string
	analysed     []apiv1.AnalyticsQuery
	purposes     []string
	bulkDeleted  []string
	queueCreates []apiv1.CreateQueueRequest
	queueUpdates []apiv1.UpdateQueueRequest
	queueDeletes []string
	retried      map[string][]string
	eventQueries []apiv1.ListEventsQuery
}

func (f *fakeOps) RetryPost(_ context.Context, id string, req apiv1.RetryPostRequest) (*apiv1.Post, error) {
	if f.retried == nil {
		f.retried = map[string][]string{}
	}
	f.retried[id] = req.AccountIDs
	return &apiv1.Post{ID: id, Status: apiv1.StatusProcessing, Deliveries: []apiv1.Delivery{
		{Platform: "x", Username: "henry", Status: apiv1.StatusPublished, URL: "https://x.com/henry/status/1"},
		{Platform: "instagram", Username: "henrygram", Status: apiv1.StatusPending},
	}}, nil
}

func (f *fakeOps) ListEvents(_ context.Context, q apiv1.ListEventsQuery) (*apiv1.EventList, error) {
	f.eventQueries = append(f.eventQueries, q)
	return &apiv1.EventList{NextCursor: "e0", Data: []apiv1.Event{{
		ID: "e1", At: time.Date(2026, 10, 22, 9, 0, 0, 0, time.UTC), Kind: "post.partial", Status: apiv1.EventStatusError, PostID: "p1",
		Message: "Failed on 1 of 2 accounts.",
		Accounts: []apiv1.EventAccount{
			{ID: "x1", Platform: "x", Status: apiv1.EventStatusOK},
			{ID: "ig1", Platform: "instagram", Status: apiv1.EventStatusError, Message: "Reconnect it.", ErrorCode: apiv1.ErrorReconnect},
		},
	}}}, nil
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

func (f *fakeOps) ListQueues(_ context.Context) (*apiv1.QueueList, error) {
	next := time.Date(2026, 10, 12, 9, 0, 0, 0, time.UTC)
	return &apiv1.QueueList{Data: []apiv1.Queue{
		{ID: "q1", Name: "Weekday mornings", ProfileName: "Default", Timezone: "UTC", Slots: []apiv1.QueueSlot{{Day: 1, Time: "09:00"}, {Day: 2, Time: "09:00"}}, Active: true, NextSlot: &next, Queued: 3},
		{ID: "q2", Name: "Weekends", ProfileName: "Acme", Timezone: "UTC", NextSlotError: "This queue is paused."},
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
func (f *fakeOps) DeletePosts(_ context.Context, req apiv1.DeletePostsRequest) (*apiv1.DeletePostsResponse, error) {
	f.bulkDeleted = append(f.bulkDeleted, req.IDs...)
	return &apiv1.DeletePostsResponse{Deleted: req.IDs, Failed: []apiv1.DeletePostsError{}}, nil
}

func (f *fakeOps) CreateQueue(_ context.Context, req apiv1.CreateQueueRequest) (*apiv1.Queue, error) {
	f.queueCreates = append(f.queueCreates, req)
	return &apiv1.Queue{ID: "q2", Name: req.Name, Timezone: req.Timezone, Slots: req.Slots, Active: !req.Paused}, nil
}

func (f *fakeOps) UpdateQueue(_ context.Context, id string, req apiv1.UpdateQueueRequest) (*apiv1.Queue, error) {
	f.queueUpdates = append(f.queueUpdates, req)
	return &apiv1.Queue{ID: id, Name: id, Timezone: "UTC", Active: req.Active == nil || *req.Active}, nil
}

func (f *fakeOps) DeleteQueue(_ context.Context, id string) (*apiv1.DeleteQueueResponse, error) {
	f.queueDeletes = append(f.queueDeletes, id)
	return &apiv1.DeleteQueueResponse{ID: id, Deleted: true, Message: "Posts it had placed stay scheduled."}, nil
}

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
		"create-post", "list-posts", "get-post", "delete-post", "list-accounts", "list-profiles", "list-queues", "get-usage", "get-analytics",
		"create-upload", "get-upload", "update-post", "connect-account",
		"delete-posts", "create-queue", "update-queue", "delete-queue", "retry-post", "list-events",
	}, names)
}

func TestRetryPostAndEvents(t *testing.T) {
	ops := &fakeOps{}
	out, err := run(t, ops, "retry-post", "p1", "--account-ids", "ig1")
	require.NoError(t, err)
	assert.Equal(t, []string{"ig1"}, ops.retried["p1"])
	assert.Contains(t, out, "Post p1  [processing]")
	assert.Contains(t, out, "postatron get-post p1")

	_, err = run(t, ops, "retry-post", "p2")
	require.NoError(t, err)
	assert.Nil(t, ops.retried["p2"], "no accounts named retries every one that failed")

	out, err = run(t, ops, "list-events", "--range", "24h", "--status", "error", "--limit", "5")
	require.NoError(t, err)
	assert.Equal(t, apiv1.ListEventsQuery{Range: "24h", Status: "error", Limit: 5}, ops.eventQueries[0])
	assert.Contains(t, out, "post.partial")
	assert.Contains(t, out, "[reconnect] Reconnect it.")
	assert.Contains(t, out, "--cursor e0")

	out, err = run(t, ops, "connect-account", "--account-id", "ig1")
	require.NoError(t, err)
	assert.Equal(t, "ig1", ops.connects[0].AccountID)
	assert.Empty(t, ops.connects[0].Platform)

	_, err = run(t, ops, "connect-account")
	require.Error(t, err, "a platform or an account to reconnect")
}

func TestAFailedPostSaysWhyAndHowToRetry(t *testing.T) {
	var buf bytes.Buffer
	printPost(&buf, apiv1.Post{ID: "p1", Status: apiv1.StatusPartial, Deliveries: []apiv1.Delivery{
		{Platform: "youtube", Username: "henrytube", Status: apiv1.StatusPublished, URL: "https://youtu.be/1", Error: "The thumbnail was not set.", Warning: true},
		{Platform: "instagram", Username: "henrygram", Status: apiv1.StatusFailed, Error: "Reconnect it.", ErrorCode: apiv1.ErrorReconnect},
	}})
	out := buf.String()
	assert.Contains(t, out, "published (warning)")
	assert.Contains(t, out, "[reconnect] Reconnect it.")
	assert.Contains(t, out, "postatron retry-post p1")
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
	_, err = run(t, ops, "update-post", "draft_1", "--add-platforms", "linkedin", "--publish-now")
	require.NoError(t, err)
	assert.True(t, ops.updated[len(ops.updated)-1].PublishNow)

	_, err = run(t, ops, "update-post", "draft_1", "--publish-now", "--scheduled-at", "2026-09-10T09:00:00Z")
	require.Error(t, err, "publishing now and at a time are contradictory")
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
	assert.Contains(t, out, "Not deleted")

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

func TestQueues(t *testing.T) {
	ops := &fakeOps{}
	out, err := run(t, ops, "list-queues")
	require.NoError(t, err)
	assert.Contains(t, out, "Weekday mornings")
	assert.Contains(t, out, "2026-10-12T09:00:00Z")
	assert.Contains(t, out, "This queue is paused.")

	_, err = run(t, ops, "create-post", "--content", "Whenever", "--platforms", "x", "--queue", "Weekday mornings")
	require.NoError(t, err)
	require.Len(t, ops.created, 1)
	assert.Equal(t, "Weekday mornings", ops.created[0].Queue)
	assert.Nil(t, ops.created[0].ScheduledAt)

	_, err = run(t, ops, "create-post", "--content", "Both", "--platforms", "x", "--queue", "q1", "--scheduled-at", "2026-10-12T09:00:00Z")
	assert.Error(t, err, "a queue picks the time, so a time as well is refused")
}

func TestQueueCommands(t *testing.T) {
	ops := &fakeOps{}
	out, err := run(t, ops, "create-queue", "--name", "Mornings", "--timezone", "Europe/London", "--slots", "mon 09:00,Thursday 17:30")
	require.NoError(t, err)
	require.Len(t, ops.queueCreates, 1)
	assert.Equal(t, []apiv1.QueueSlot{{Day: 1, Time: "09:00"}, {Day: 4, Time: "17:30"}}, ops.queueCreates[0].Slots)
	assert.Contains(t, out, "Mornings")

	_, err = run(t, ops, "create-queue", "--name", "Bad", "--timezone", "UTC", "--slots", "someday 09:00")
	require.Error(t, err)

	_, err = run(t, ops, "update-queue", "Mornings", "--pause")
	require.NoError(t, err)
	require.Len(t, ops.queueUpdates, 1)
	require.NotNil(t, ops.queueUpdates[0].Active)
	assert.False(t, *ops.queueUpdates[0].Active)
	assert.Nil(t, ops.queueUpdates[0].Slots)
	assert.False(t, ops.queueUpdates[0].MovePosts, "posts stay put unless asked")

	_, err = run(t, ops, "update-queue", "Mornings", "--slots", "tue 10:00", "--move-posts")
	require.NoError(t, err)
	require.Len(t, ops.queueUpdates, 2)
	assert.True(t, ops.queueUpdates[1].MovePosts)

	out, err = run(t, ops, "delete-queue", "Mornings")
	require.NoError(t, err)
	assert.Equal(t, []string{"Mornings"}, ops.queueDeletes)
	assert.Contains(t, out, "stay scheduled")
}

func TestDeletePostsAndOptions(t *testing.T) {
	ops := &fakeOps{}
	out, err := run(t, ops, "delete-posts", "p1", "p2")
	require.NoError(t, err)
	assert.Equal(t, []string{"p1", "p2"}, ops.bulkDeleted)
	assert.Contains(t, out, "Deleted 2 of 2")

	_, err = run(t, ops, "create-post", "--content", "Tea or coffee?", "--platforms", "x", "--poll", "Tea,Coffee",
		"--thread", "2/ more, with a comma", "--options", `{"instagram":{"post_type":"reel"}}`)
	require.NoError(t, err)
	require.Len(t, ops.created, 1)
	req := ops.created[0]
	require.NotNil(t, req.X)
	assert.Equal(t, []string{"Tea", "Coffee"}, req.X.Poll.Options)
	assert.Equal(t, []apiv1.XThreadTweet{{Content: "2/ more, with a comma"}}, req.X.Thread)
	assert.Equal(t, "reel", req.Instagram.PostType)

	_, err = run(t, ops, "create-post", "--content", "x", "--platforms", "x", "--options", `{"x":{"polll":{}}}`)
	require.Error(t, err, "a misspelt option is refused rather than ignored")
}
