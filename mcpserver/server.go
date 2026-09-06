// Package mcpserver exposes the six public API operations as MCP tools. The
// tool set is fixed: one tool per endpoint, same names as the CLI commands.
package mcpserver

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/henrwal/postatron-cli/apiv1"
	"github.com/modelcontextprotocol/go-sdk/jsonrpc"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// Version is reported to MCP clients in serverInfo.
// Overridden at release time with -ldflags "-X ...mcpserver.Version=v1.2.3".
var Version = "1.0.0"

// Tool names, kept identical to the CLI commands (with underscores).
const (
	ToolCreatePost   = "create_post"
	ToolListPosts    = "list_posts"
	ToolGetPost      = "get_post"
	ToolDeletePost   = "delete_post"
	ToolListAccounts = "list_accounts"
	ToolGetUsage     = "get_usage"
)

// CreatePostInput is the create_post tool input.
type CreatePostInput struct {
	Content     string   `json:"content" jsonschema:"The text of the post. Links are detected automatically; posts to X that contain a link are metered separately."`
	Platforms   []string `json:"platforms,omitempty" jsonschema:"Platforms to publish to, e.g. [\"x\",\"linkedin\",\"bluesky\",\"threads\",\"facebook\"]. Uses every connected account on each platform. Use this OR account_ids."`
	AccountIDs  []string `json:"account_ids,omitempty" jsonschema:"Specific connected account ids from list_accounts. Use this OR platforms."`
	ScheduledAt string   `json:"scheduled_at,omitempty" jsonschema:"RFC 3339 timestamp (UTC) to schedule the post, at least 5 minutes ahead, e.g. 2026-09-10T09:00:00Z. Omit to publish now."`
}

// ListPostsInput is the list_posts tool input.
type ListPostsInput struct {
	Status   string `json:"status,omitempty" jsonschema:"Filter by status: scheduled, pending, processing, published, failed or partial."`
	Platform string `json:"platform,omitempty" jsonschema:"Only posts that target this platform, e.g. x or linkedin."`
	From     string `json:"from,omitempty" jsonschema:"RFC 3339 start of the date range (scheduled_at for scheduled posts, created_at otherwise)."`
	To       string `json:"to,omitempty" jsonschema:"RFC 3339 end of the date range."`
	Limit    int    `json:"limit,omitempty" jsonschema:"Page size, 1-100 (default 25)."`
	Cursor   string `json:"cursor,omitempty" jsonschema:"next_cursor from a previous page."`
}

// PostIDInput is shared by get_post and delete_post.
type PostIDInput struct {
	ID string `json:"id" jsonschema:"The post id."`
}

// EmptyInput is used by tools that take no arguments.
type EmptyInput struct{}

// New builds an MCP server whose tools call ops.
func New(ops apiv1.Operations) *mcp.Server {
	server := mcp.NewServer(&mcp.Implementation{
		Name:       "postatron",
		Title:      "Postatron",
		Version:    Version,
		WebsiteURL: "https://postatron.com",
	}, nil)

	mcp.AddTool(server, &mcp.Tool{
		Name:        ToolCreatePost,
		Description: "Create a social media post. Publishes immediately unless scheduled_at is set. One post fanned out to N platforms consumes N destination-posts of the monthly quota.",
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in CreatePostInput) (*mcp.CallToolResult, *apiv1.Post, error) {
		req := apiv1.CreatePostRequest{Content: in.Content, Platforms: in.Platforms, AccountIDs: in.AccountIDs}
		if strings.TrimSpace(in.ScheduledAt) != "" {
			when, err := time.Parse(time.RFC3339, strings.TrimSpace(in.ScheduledAt))
			if err != nil {
				return nil, nil, fmt.Errorf("scheduled_at must be RFC 3339, e.g. 2026-09-10T09:00:00Z (got %q)", in.ScheduledAt)
			}
			req.ScheduledAt = &when
		}
		post, err := ops.CreatePost(ctx, req)
		if err != nil {
			return handleError[*apiv1.Post](err)
		}
		return nil, post, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        ToolListPosts,
		Description: "List posts, newest first, with optional status, platform and date filters. Returns next_cursor when more pages exist.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in ListPostsInput) (*mcp.CallToolResult, *apiv1.PostList, error) {
		query := apiv1.ListPostsQuery{Status: in.Status, Platform: in.Platform, Limit: in.Limit, Cursor: in.Cursor}
		if strings.TrimSpace(in.From) != "" {
			from, err := time.Parse(time.RFC3339, strings.TrimSpace(in.From))
			if err != nil {
				return nil, nil, fmt.Errorf("from must be RFC 3339 (got %q)", in.From)
			}
			query.From = &from
		}
		if strings.TrimSpace(in.To) != "" {
			to, err := time.Parse(time.RFC3339, strings.TrimSpace(in.To))
			if err != nil {
				return nil, nil, fmt.Errorf("to must be RFC 3339 (got %q)", in.To)
			}
			query.To = &to
		}
		list, err := ops.ListPosts(ctx, query)
		if err != nil {
			return handleError[*apiv1.PostList](err)
		}
		return nil, list, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        ToolGetPost,
		Description: "Get one post with its per-platform delivery status, URLs and errors.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in PostIDInput) (*mcp.CallToolResult, *apiv1.Post, error) {
		post, err := ops.GetPost(ctx, strings.TrimSpace(in.ID))
		if err != nil {
			return handleError[*apiv1.Post](err)
		}
		return nil, post, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        ToolDeletePost,
		Description: "Cancel a scheduled post. Posts that were already published are left untouched and reported as cancelled=false.",
		Annotations: &mcp.ToolAnnotations{IdempotentHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, in PostIDInput) (*mcp.CallToolResult, *apiv1.DeletePostResponse, error) {
		out, err := ops.DeletePost(ctx, strings.TrimSpace(in.ID))
		if err != nil {
			return handleError[*apiv1.DeletePostResponse](err)
		}
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        ToolListAccounts,
		Description: "List the connected social accounts (id, platform, username, status). Use the ids or platforms with create_post.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, *apiv1.AccountList, error) {
		out, err := ops.ListAccounts(ctx)
		if err != nil {
			return handleError[*apiv1.AccountList](err)
		}
		return nil, out, nil
	})

	mcp.AddTool(server, &mcp.Tool{
		Name:        ToolGetUsage,
		Description: "Show this month's quota: destination-posts used and remaining, X link posts, posts per platform, overage and rate limits.",
		Annotations: &mcp.ToolAnnotations{ReadOnlyHint: true},
	}, func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, *apiv1.UsageReport, error) {
		out, err := ops.GetUsage(ctx)
		if err != nil {
			return handleError[*apiv1.UsageReport](err)
		}
		return nil, out, nil
	})

	return server
}

// handleError renders API errors with their code, retry hint and details.
// The SDK turns any returned error into an isError tool result, so the model
// sees the message instead of a protocol failure.
func handleError[Out any](err error) (*mcp.CallToolResult, Out, error) {
	var zero Out
	var apiErr *apiv1.APIError
	if errors.As(err, &apiErr) {
		msg := fmt.Sprintf("%s: %s", apiErr.Code, apiErr.Message)
		if apiErr.RetryAfter > 0 {
			msg += fmt.Sprintf(" (retry after %s)", apiErr.RetryAfter)
		}
		if len(apiErr.Details) > 0 {
			msg += fmt.Sprintf(" details=%v", apiErr.Details)
		}
		return nil, zero, errors.New(msg)
	}
	return nil, zero, err
}

// Run serves the tools over stdio until the client disconnects.
func Run(ctx context.Context, ops apiv1.Operations) error {
	err := New(ops).Run(ctx, &mcp.StdioTransport{})
	var rpcErr *jsonrpc.Error
	if errors.Is(err, io.EOF) || (errors.As(err, &rpcErr) && rpcErr.Code == codeServerClosing) {
		// The host closed stdin: the normal way an MCP client shuts a server down.
		return nil
	}
	return err
}

// codeServerClosing is the JSON-RPC code the SDK reports when the connection
// is torn down (jsonrpc2.ErrServerClosing, wrapped around the stdin EOF).
const codeServerClosing = -32004
