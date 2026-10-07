// Package mcpserver exposes the public API operations as MCP tools: one tool
// per endpoint, with the same names and arguments as the remote server at
// https://api.postatron.com/mcp.
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
	ToolCreatePost     = "create_post"
	ToolListPosts      = "list_posts"
	ToolGetPost        = "get_post"
	ToolDeletePost     = "delete_post"
	ToolUpdatePost     = "update_post"
	ToolConnectAccount = "connect_account"
	ToolListAccounts   = "list_accounts"
	ToolListProfiles   = "list_profiles"
	ToolGetUsage       = "get_usage"
	ToolGetAnalytics   = "get_analytics"
	ToolCreateUpload   = "create_upload"
	ToolGetUpload      = "get_upload"
)

// CreatePostInput is the create_post tool input.
type CreatePostInput struct {
	Content     string   `json:"content" jsonschema:"The text of the post. Links are detected automatically; posts to X that contain a link are metered separately."`
	Platforms   []string `json:"platforms,omitempty" jsonschema:"Platforms to publish to, e.g. [\"x\",\"linkedin\",\"bluesky\",\"threads\",\"facebook\"]. Posts to the account on each platform in profile. Use this OR account_ids."`
	AccountIDs  []string `json:"account_ids,omitempty" jsonschema:"Specific connected account ids from list_accounts. Use this OR platforms."`
	Profile     string   `json:"profile,omitempty" jsonschema:"A profile name or id from list_profiles. Required with platforms when a platform has accounts in more than one profile."`
	MediaURLs   []string `json:"media_urls,omitempty" jsonschema:"Public https URLs of images or a video to attach (up to 4). Instagram and TikTok need one; YouTube needs a video."`
	MediaIDs    []string `json:"media_ids,omitempty" jsonschema:"Upload ids from create_upload, for files on the person's own device."`
	ScheduledAt string   `json:"scheduled_at,omitempty" jsonschema:"RFC 3339 timestamp (UTC) to schedule the post, at least 5 minutes ahead, e.g. 2026-09-10T09:00:00Z. Omit to publish now."`
}

// UpdatePostInput is the update_post tool input.
type UpdatePostInput struct {
	PostID           string   `json:"post_id" jsonschema:"The id of a draft or scheduled post, from create_post or list_posts."`
	Content          string   `json:"content,omitempty" jsonschema:"New text for the post. Omit to keep it."`
	ScheduledAt      string   `json:"scheduled_at,omitempty" jsonschema:"New RFC 3339 time, at least 5 minutes ahead; on a draft, this schedules it. Omit to keep it."`
	PublishNow       bool     `json:"publish_now,omitempty" jsonschema:"Publish the post now, after the other changes. Not with scheduled_at."`
	AddPlatforms     []string `json:"add_platforms,omitempty" jsonschema:"Platforms to add, e.g. [\"instagram\"]: the account on each in profile. Instagram and TikTok need the post to have an image or a video."`
	AddAccountIDs    []string `json:"add_account_ids,omitempty" jsonschema:"Specific connected account ids to add, from list_accounts."`
	RemoveAccountIDs []string `json:"remove_account_ids,omitempty" jsonschema:"Account ids to take off the post. At least one must remain."`
	Profile          string   `json:"profile,omitempty" jsonschema:"A profile name or id from list_profiles, saying which brand add_platforms means."`
}

// ConnectAccountInput is the connect_account tool input.
type ConnectAccountInput struct {
	Platform string `json:"platform" jsonschema:"The network to connect: x, instagram, facebook, linkedin, tiktok, youtube, threads or bluesky."`
	Profile  string `json:"profile,omitempty" jsonschema:"A profile name or id from list_profiles to put the account in. Omit for the Default profile."`
}

// ListPostsInput is the list_posts tool input.
type ListPostsInput struct {
	Status   string `json:"status,omitempty" jsonschema:"Filter by status: draft, scheduled, pending, processing, published, failed or partial."`
	Platform string `json:"platform,omitempty" jsonschema:"Only posts that target this platform, e.g. x or linkedin."`
	Profile  string `json:"profile,omitempty" jsonschema:"Only posts to accounts in this profile (a name or id from list_profiles)."`
	From     string `json:"from,omitempty" jsonschema:"RFC 3339 start of the date range (scheduled_at for scheduled posts, created_at otherwise)."`
	To       string `json:"to,omitempty" jsonschema:"RFC 3339 end of the date range."`
	Limit    int    `json:"limit,omitempty" jsonschema:"Page size, 1-100 (default 25)."`
	Cursor   string `json:"cursor,omitempty" jsonschema:"next_cursor from a previous page."`
}

// PostIDInput is shared by get_post and delete_post.
type PostIDInput struct {
	ID string `json:"id" jsonschema:"The post id."`
}

// AnalyticsInput is the get_analytics tool input.
type AnalyticsInput struct {
	Range     string `json:"range,omitempty" jsonschema:"One of 7d, 30d or 90d. Defaults to 30d."`
	Platform  string `json:"platform,omitempty" jsonschema:"Only this platform, e.g. instagram, facebook, threads, tiktok, bluesky, linkedin, youtube."`
	AccountID string `json:"account_id,omitempty" jsonschema:"Only this connected account (an id from list_accounts)."`
	Profile   string `json:"profile,omitempty" jsonschema:"Only accounts in this profile (a name or id from list_profiles)."`
	Source    string `json:"source,omitempty" jsonschema:"all (default) or postatron, to count only posts published with Postatron."`
}

// CreateUploadInput is the create_upload tool input.
type CreateUploadInput struct {
	Purpose string `json:"purpose,omitempty" jsonschema:"Optional note shown on the upload page, e.g. \"photo for your Tuesday post\"."`
}

// UploadIDInput is the get_upload tool input.
type UploadIDInput struct {
	ID string `json:"upload_id" jsonschema:"The id from create_upload."`
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

	mcp.AddTool(server, describe(&mcp.Tool{
		Name:        ToolCreatePost,
		Description: "Create a social media post. Publishes immediately unless scheduled_at is set. One post fanned out to N platforms consumes N destination-posts of the monthly quota.",
	}), func(ctx context.Context, _ *mcp.CallToolRequest, in CreatePostInput) (*mcp.CallToolResult, *apiv1.Post, error) {
		req := apiv1.CreatePostRequest{
			Content:    in.Content,
			Platforms:  in.Platforms,
			AccountIDs: in.AccountIDs,
			Profile:    in.Profile,
			MediaURLs:  in.MediaURLs,
			MediaIDs:   in.MediaIDs,
		}
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

	mcp.AddTool(server, describe(&mcp.Tool{
		Name:        ToolListPosts,
		Description: "List posts, newest first, with optional status, platform and date filters. Returns next_cursor when more pages exist.",
	}), func(ctx context.Context, _ *mcp.CallToolRequest, in ListPostsInput) (*mcp.CallToolResult, *apiv1.PostList, error) {
		query := apiv1.ListPostsQuery{Status: in.Status, Platform: in.Platform, Profile: in.Profile, Limit: in.Limit, Cursor: in.Cursor}
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

	mcp.AddTool(server, describe(&mcp.Tool{
		Name:        ToolGetPost,
		Description: "Get one post with its per-platform delivery status, URLs and errors.",
	}), func(ctx context.Context, _ *mcp.CallToolRequest, in PostIDInput) (*mcp.CallToolResult, *apiv1.Post, error) {
		post, err := ops.GetPost(ctx, strings.TrimSpace(in.ID))
		if err != nil {
			return handleError[*apiv1.Post](err)
		}
		return nil, post, nil
	})

	mcp.AddTool(server, describe(&mcp.Tool{
		Name:        ToolDeletePost,
		Description: "Cancel a scheduled post. Posts that were already published are left untouched and reported as cancelled=false.",
	}), func(ctx context.Context, _ *mcp.CallToolRequest, in PostIDInput) (*mcp.CallToolResult, *apiv1.DeletePostResponse, error) {
		out, err := ops.DeletePost(ctx, strings.TrimSpace(in.ID))
		if err != nil {
			return handleError[*apiv1.DeletePostResponse](err)
		}
		return nil, out, nil
	})

	mcp.AddTool(server, describe(&mcp.Tool{
		Name: ToolUpdatePost,
		Description: "Change a draft or scheduled post: its text, its time, or the accounts it goes to (add_platforms to add, e.g. Instagram, remove_account_ids to drop one). " +
			"Use this rather than a second create_post when the person adds to a post they already have. A draft (list_posts with status draft) goes out with scheduled_at or publish_now; otherwise it stays a draft. " +
			"publish_now posts to the person's real accounts at once, so confirm first. Accounts added count against the monthly quota like a new post.",
	}), func(ctx context.Context, _ *mcp.CallToolRequest, in UpdatePostInput) (*mcp.CallToolResult, *apiv1.Post, error) {
		req := apiv1.UpdatePostRequest{
			AddPlatforms:     in.AddPlatforms,
			AddAccountIDs:    in.AddAccountIDs,
			RemoveAccountIDs: in.RemoveAccountIDs,
			Profile:          in.Profile,
			PublishNow:       in.PublishNow,
		}
		if content := strings.TrimSpace(in.Content); content != "" {
			req.Content = &content
		}
		if strings.TrimSpace(in.ScheduledAt) != "" {
			when, err := time.Parse(time.RFC3339, strings.TrimSpace(in.ScheduledAt))
			if err != nil {
				return nil, nil, fmt.Errorf("scheduled_at must be RFC 3339, e.g. 2026-09-10T09:00:00Z (got %q)", in.ScheduledAt)
			}
			req.ScheduledAt = &when
		}
		post, err := ops.UpdatePost(ctx, strings.TrimSpace(in.PostID), req)
		if err != nil {
			return handleError[*apiv1.Post](err)
		}
		return nil, post, nil
	})

	mcp.AddTool(server, describe(&mcp.Tool{
		Name: ToolConnectAccount,
		Description: "Get a link that connects one of the person's social accounts to Postatron. Give them the connect_url: it opens Postatron and starts that platform's sign-in. " +
			"Nothing is connected until they finish there, so wait for them to say it is done, then call list_accounts.",
	}), func(ctx context.Context, _ *mcp.CallToolRequest, in ConnectAccountInput) (*mcp.CallToolResult, *apiv1.ConnectLink, error) {
		out, err := ops.ConnectAccount(ctx, apiv1.ConnectAccountRequest{Platform: strings.TrimSpace(in.Platform), Profile: strings.TrimSpace(in.Profile)})
		if err != nil {
			return handleError[*apiv1.ConnectLink](err)
		}
		return nil, out, nil
	})

	mcp.AddTool(server, describe(&mcp.Tool{
		Name:        ToolListAccounts,
		Description: "List the connected social accounts (id, platform, username, profile, status). Use the ids or platforms with create_post.",
	}), func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, *apiv1.AccountList, error) {
		out, err := ops.ListAccounts(ctx)
		if err != nil {
			return handleError[*apiv1.AccountList](err)
		}
		return nil, out, nil
	})

	mcp.AddTool(server, describe(&mcp.Tool{
		Name:        ToolListProfiles,
		Description: "List profiles and the accounts in each. A profile groups accounts, usually one per brand or client, with at most one account per platform. Pass a profile to create_post to post as that brand.",
	}), func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, *apiv1.ProfileList, error) {
		out, err := ops.ListProfiles(ctx)
		if err != nil {
			return handleError[*apiv1.ProfileList](err)
		}
		return nil, out, nil
	})

	mcp.AddTool(server, describe(&mcp.Tool{
		Name: ToolGetAnalytics,
		Description: "How the posts and accounts are performing over the last 7, 30 or 90 days: engagement rate, reach, followers, " +
			"totals per platform, the best posts, and any account that needs reconnecting. Metrics a platform does not report are null. X is not included.",
	}), func(ctx context.Context, _ *mcp.CallToolRequest, in AnalyticsInput) (*mcp.CallToolResult, *apiv1.AnalyticsReport, error) {
		out, err := ops.GetAnalytics(ctx, apiv1.AnalyticsQuery{
			Range: in.Range, Platform: in.Platform, AccountID: in.AccountID, Profile: in.Profile, Source: in.Source,
		})
		if err != nil {
			return handleError[*apiv1.AnalyticsReport](err)
		}
		return nil, out, nil
	})

	mcp.AddTool(server, describe(&mcp.Tool{
		Name: ToolCreateUpload,
		Description: "Get a link the person can use to attach a photo or video from their own device. Give them the upload_url, wait for them to say they have uploaded it, " +
			"check with get_upload, then pass the id to create_post as media_ids. Use this whenever the file is not already on the public internet.",
	}), func(ctx context.Context, _ *mcp.CallToolRequest, in CreateUploadInput) (*mcp.CallToolResult, *apiv1.Upload, error) {
		out, err := ops.CreateUpload(ctx, apiv1.CreateUploadRequest{Purpose: in.Purpose})
		if err != nil {
			return handleError[*apiv1.Upload](err)
		}
		return nil, out, nil
	})

	mcp.AddTool(server, describe(&mcp.Tool{
		Name:        ToolGetUpload,
		Description: "Report whether the person has uploaded their file yet: PENDING until they do, READY afterwards. Ask them to tell you when they are done rather than polling.",
	}), func(ctx context.Context, _ *mcp.CallToolRequest, in UploadIDInput) (*mcp.CallToolResult, *apiv1.Upload, error) {
		out, err := ops.GetUpload(ctx, strings.TrimSpace(in.ID))
		if err != nil {
			return handleError[*apiv1.Upload](err)
		}
		return nil, out, nil
	})

	mcp.AddTool(server, describe(&mcp.Tool{
		Name:        ToolGetUsage,
		Description: "Show this month's quota: destination-posts used and remaining, X link posts, posts per platform and rate limits.",
	}), func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, *apiv1.UsageReport, error) {
		out, err := ops.GetUsage(ctx)
		if err != nil {
			return handleError[*apiv1.UsageReport](err)
		}
		return nil, out, nil
	})

	return server
}

// toolNotes gives every tool the same title and hints as the remote server
// (cmd/api/v1/mcp.go in the backend), so a client treats each tool alike
// whichever server it came from. Claude runs read-only tools without asking
// and always confirms a destructive one; publishing to someone's real
// accounts counts as destructive.
var toolNotes = map[string]mcp.ToolAnnotations{
	ToolListAccounts: {Title: "List connected accounts", ReadOnlyHint: true, IdempotentHint: true},
	ToolListProfiles: {Title: "List profiles", ReadOnlyHint: true, IdempotentHint: true},
	ToolCreatePost:   {Title: "Create or schedule a post", DestructiveHint: boolPtr(true)},
	ToolListPosts:    {Title: "List posts", ReadOnlyHint: true, IdempotentHint: true},
	ToolGetPost:      {Title: "Get one post", ReadOnlyHint: true, IdempotentHint: true},
	ToolDeletePost:   {Title: "Cancel a scheduled post", DestructiveHint: boolPtr(true), IdempotentHint: true},
	ToolUpdatePost:   {Title: "Change, schedule or publish a post", DestructiveHint: boolPtr(true)},
	// A link only; nothing changes until the person finishes signing in.
	ToolConnectAccount: {Title: "Connect a social account", DestructiveHint: boolPtr(false)},
	ToolCreateUpload:   {Title: "Ask the person to attach a file", DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	ToolGetUpload:      {Title: "Check whether a file has been attached", ReadOnlyHint: true, IdempotentHint: true},
	ToolGetAnalytics:   {Title: "Get post analytics", ReadOnlyHint: true, IdempotentHint: true},
	ToolGetUsage:       {Title: "Get plan usage", ReadOnlyHint: true, IdempotentHint: true},
}

// describe adds the tool's title and hints from toolNotes.
func describe(tool *mcp.Tool) *mcp.Tool {
	if notes, ok := toolNotes[tool.Name]; ok {
		tool.Title = notes.Title
		tool.Annotations = &notes
	}
	return tool
}

func boolPtr(v bool) *bool { return &v }

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
