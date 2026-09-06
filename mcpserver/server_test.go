package mcpserver_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/henrwal/postatron-cli/apiv1"
	"github.com/henrwal/postatron-cli/mcpserver"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeOps struct {
	created  []apiv1.CreatePostRequest
	listed   []apiv1.ListPostsQuery
	deleted  []string
	fetched  []string
	usageErr error
}

func (f *fakeOps) CreatePost(_ context.Context, req apiv1.CreatePostRequest) (*apiv1.Post, error) {
	f.created = append(f.created, req)
	return &apiv1.Post{ID: "p1", Status: "scheduled", Content: req.Content, Deliveries: []apiv1.Delivery{}}, nil
}

func (f *fakeOps) ListPosts(_ context.Context, q apiv1.ListPostsQuery) (*apiv1.PostList, error) {
	f.listed = append(f.listed, q)
	return &apiv1.PostList{Data: []apiv1.Post{{ID: "p1", Deliveries: []apiv1.Delivery{}}}, NextCursor: "n"}, nil
}

func (f *fakeOps) GetPost(_ context.Context, id string) (*apiv1.Post, error) {
	f.fetched = append(f.fetched, id)
	if id == "missing" {
		return nil, &apiv1.APIError{Status: http.StatusNotFound, Code: apiv1.CodeNotFound, Message: "post not found"}
	}
	return &apiv1.Post{ID: id, Deliveries: []apiv1.Delivery{}}, nil
}

func (f *fakeOps) DeletePost(_ context.Context, id string) (*apiv1.DeletePostResponse, error) {
	f.deleted = append(f.deleted, id)
	return &apiv1.DeletePostResponse{ID: id, Status: "cancelled", Cancelled: true}, nil
}

func (f *fakeOps) ListAccounts(_ context.Context) (*apiv1.AccountList, error) {
	return &apiv1.AccountList{Data: []apiv1.Account{{ID: "a1", Platform: "x", Username: "henry"}}}, nil
}

func (f *fakeOps) GetUsage(_ context.Context) (*apiv1.UsageReport, error) {
	if f.usageErr != nil {
		return nil, f.usageErr
	}
	return &apiv1.UsageReport{Plan: apiv1.UsagePlan{Code: "STARTER"}, ByPlatform: map[string]int{"x": 3}}, nil
}

func connect(t *testing.T, ops apiv1.Operations) *mcp.ClientSession {
	t.Helper()
	ctx := context.Background()
	serverTransport, clientTransport := mcp.NewInMemoryTransports()

	server := mcpserver.New(ops)
	_, err := server.Connect(ctx, serverTransport, nil)
	require.NoError(t, err)

	client := mcp.NewClient(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	session, err := client.Connect(ctx, clientTransport, nil)
	require.NoError(t, err)
	t.Cleanup(func() { _ = session.Close() })
	return session
}

func TestExposesExactlySixTools(t *testing.T) {
	session := connect(t, &fakeOps{})
	tools, err := session.ListTools(context.Background(), nil)
	require.NoError(t, err)

	names := make([]string, 0, len(tools.Tools))
	for _, tool := range tools.Tools {
		names = append(names, tool.Name)
		assert.NotEmpty(t, tool.Description)
		assert.NotNil(t, tool.InputSchema)
	}
	assert.ElementsMatch(t, []string{"create_post", "list_posts", "get_post", "delete_post", "list_accounts", "get_usage"}, names)
}

func TestCreatePostTool(t *testing.T) {
	ops := &fakeOps{}
	session := connect(t, ops)

	res, err := session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "create_post",
		Arguments: map[string]any{"content": "hello", "platforms": []string{"x", "linkedin"}, "scheduled_at": "2026-09-10T09:00:00Z"},
	})
	require.NoError(t, err)
	assert.False(t, res.IsError)

	require.Len(t, ops.created, 1)
	assert.Equal(t, "hello", ops.created[0].Content)
	assert.Equal(t, []string{"x", "linkedin"}, ops.created[0].Platforms)
	require.NotNil(t, ops.created[0].ScheduledAt)
	assert.Equal(t, time.Date(2026, 9, 10, 9, 0, 0, 0, time.UTC), ops.created[0].ScheduledAt.UTC())

	structured, err := json.Marshal(res.StructuredContent)
	require.NoError(t, err)
	var post apiv1.Post
	require.NoError(t, json.Unmarshal(structured, &post))
	assert.Equal(t, "p1", post.ID)
	require.Len(t, res.Content, 1)
	assert.Contains(t, res.Content[0].(*mcp.TextContent).Text, `"id":"p1"`)

	res, err = session.CallTool(context.Background(), &mcp.CallToolParams{
		Name:      "create_post",
		Arguments: map[string]any{"content": "hello", "platforms": []string{"x"}, "scheduled_at": "tomorrow"},
	})
	require.NoError(t, err)
	assert.True(t, res.IsError, "bad timestamps are reported to the model, not raised")
	assert.Contains(t, res.Content[0].(*mcp.TextContent).Text, "scheduled_at must be RFC 3339")
	assert.Len(t, ops.created, 1)
}

func TestReadToolsAndErrors(t *testing.T) {
	ops := &fakeOps{}
	session := connect(t, ops)
	ctx := context.Background()

	res, err := session.CallTool(ctx, &mcp.CallToolParams{Name: "list_posts", Arguments: map[string]any{"status": "published", "platform": "x", "from": "2026-09-01T00:00:00Z", "limit": 5}})
	require.NoError(t, err)
	assert.False(t, res.IsError)
	require.Len(t, ops.listed, 1)
	assert.Equal(t, "published", ops.listed[0].Status)
	assert.Equal(t, 5, ops.listed[0].Limit)
	require.NotNil(t, ops.listed[0].From)

	res, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "get_post", Arguments: map[string]any{"id": "missing"}})
	require.NoError(t, err)
	assert.True(t, res.IsError)
	assert.Contains(t, res.Content[0].(*mcp.TextContent).Text, "not_found: post not found")

	res, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "delete_post", Arguments: map[string]any{"id": "p9"}})
	require.NoError(t, err)
	assert.False(t, res.IsError)
	assert.Equal(t, []string{"p9"}, ops.deleted)

	res, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "list_accounts", Arguments: map[string]any{}})
	require.NoError(t, err)
	assert.Contains(t, res.Content[0].(*mcp.TextContent).Text, `"platform":"x"`)

	res, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "get_usage", Arguments: map[string]any{}})
	require.NoError(t, err)
	assert.Contains(t, res.Content[0].(*mcp.TextContent).Text, `"STARTER"`)

	ops.usageErr = errors.New("network down")
	res, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "get_usage", Arguments: map[string]any{}})
	require.NoError(t, err)
	assert.True(t, res.IsError, "transport failures surface as tool errors")
	assert.Contains(t, res.Content[0].(*mcp.TextContent).Text, "network down")

	_, err = session.CallTool(ctx, &mcp.CallToolParams{Name: "get_post", Arguments: map[string]any{}})
	require.Error(t, err, "missing required id fails schema validation")
}
