// Package mcpserver exposes the public API operations as MCP tools: one tool
// per endpoint, with the same names, arguments, descriptions and hints as the
// remote server at https://api.postatron.com/mcp (cmd/api/v1/mcp.go in the
// backend). Change both together.
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
	ToolDeletePosts    = "delete_posts"
	ToolUpdatePost     = "update_post"
	ToolRetryPost      = "retry_post"
	ToolListEvents     = "list_events"
	ToolConnectAccount = "connect_account"
	ToolListAccounts   = "list_accounts"
	ToolListProfiles   = "list_profiles"
	ToolListQueues     = "list_queues"
	ToolCreateQueue    = "create_queue"
	ToolUpdateQueue    = "update_queue"
	ToolDeleteQueue    = "delete_queue"
	ToolGetUsage       = "get_usage"
	ToolGetAnalytics   = "get_analytics"
	ToolCreateUpload   = "create_upload"
	ToolGetUpload      = "get_upload"
)

// Descriptions, word for word the remote server's.
const (
	descListAccounts = "List the social accounts connected to this Postatron account, with the id needed to post to each one and the profile it belongs to."
	descListProfiles = "List profiles and the accounts in each. A profile groups connected accounts, usually one per brand or client, " +
		"with at most one account per platform. Pass a profile's name to create_post, list_posts or get_analytics to work with that brand only."
	descListQueues = "List posting queues: weekly time slots per profile, each with its next free slot (next_slot). " +
		"Pass a queue's name to create_post as queue to schedule a post into that slot instead of choosing a time."
	descCreatePost = "Publish a post now, schedule it, or add it to a queue. Omit scheduled_at and queue to publish immediately. " +
		"Instagram and TikTok need an image or a video, YouTube a video. A post to N accounts uses N destination-posts of the monthly quota. " +
		"An account that needs reconnecting is refused with reconnect_required; give the person connect_account's link for it."
	descListPosts = "List posts, most recent first, filtered by status, platform, profile or date. A draft with unpublished.reason no_plan came due while the account " +
		"had no plan and went back to the drafts; once there is a plan, schedule or publish it with update_post."
	descGetPost = "Fetch one post by id, with each account's status and link. A failed delivery has error and error_code (reconnect, rate_limited, platform_unavailable, " +
		"media_rejected, policy, validation, postatron or unknown); a published one with warning true went out, and error says what did not go with it."
	descDeletePosts = "Delete up to 100 posts at once, as delete_post does each: published posts are only removed from Postatron and stay on the platforms. " +
		"Every id comes back in deleted or failed, with the reason."
	descCreateQueue = "Create a posting queue: weekly time slots for one profile. Posts added to it with create_post's queue go out in its next free slot."
	descUpdateQueue = "Change a queue's name, slots, timezone or profile, or pause or resume it. Posts it has already placed keep their times, " +
		"unless move_posts is true with new slots or a new timezone: then its upcoming posts take the new slots in order, except ones given a time of their own."
	descDeleteQueue = "Delete a queue. Posts it has already placed stay scheduled at their times."
	descDeletePost  = "Delete a post from Postatron. A scheduled post or draft is cancelled and never goes out. A published post is only removed from Postatron: " +
		"it stays live on the platforms and the person has to delete it there, so tell them. A post publishing now, or due within two minutes, cannot be deleted."
	descUpdatePost = "Change a draft or scheduled post: its text, its time, or the accounts it goes to. Use add_platforms (e.g. [\"instagram\"]) when the person wants a post they already have on another platform too, " +
		"rather than a second create_post, so it stays one post. A draft (list_posts with status draft) goes out with scheduled_at or publish_now; otherwise it stays a draft. " +
		"publish_now posts to the person's real accounts at once, so confirm first. Accounts added count against the monthly quota like a new post. Instagram and TikTok need the post to have an image or a video."
	descRetryPost = "Send a failed or partial post again to the accounts it failed on. An account it already published to is never posted to twice. " +
		"Check each failed delivery's error_code (get_post) first: reconnect needs connect_account with that account_id before a retry, rate_limited may need a wait, " +
		"and media_rejected or validation will fail the same way, since a retry sends the same post. It posts to the person's real accounts, so confirm first. " +
		"Retries are not charged against the monthly quota again."
	descConnectAccount = "Get a link that connects one of the person's social accounts to Postatron, or signs one in again whose access has gone. Give them the connect_url: " +
		"it opens Postatron and starts that platform's sign-in. Nothing changes until they finish there, so wait for them to say it is done, then call list_accounts. " +
		"Use it when they ask to post somewhere that is not connected yet, and with account_id when an account says reconnect_required or a delivery failed with error_code reconnect."
	descCreateUpload = "Get a link the person can use to attach a photo or video from their own device. Give them the upload_url, wait for them to say they have uploaded it, check with get_upload, then pass the id to create_post as media_ids. Use this whenever the file is not already on the public internet."
	descGetUpload    = "Report whether the person has uploaded their file yet. Status is PENDING until they do and READY afterwards. Do not poll in a tight loop: ask them to tell you when they are done."
	descListEvents   = "The account's event log, newest first, as the dashboard's Logs page shows it: each post's outcome on every account (a failure with its error_code), " +
		"accounts connected, disconnected or needing a reconnect, and API or MCP calls that changed something or were refused. " +
		"Use it to find what failed recently and why; retry_post sends a failed post again."
	descGetAnalytics = "How the user's posts and accounts are performing over the last 7, 30, 90 or 365 days: engagement rate, " +
		"reach, followers, totals per platform, the best posts, and any account that needs reconnecting. " +
		"Metrics a platform does not report come back as null. X is not included. YouTube's numbers are in the per-platform totals as YouTube reports them, " +
		"but YouTube's policies keep them out of engagement totals, rates and top_posts. Each account is read at most once an hour (last_sync_at, next_sync_at)."
	descGetUsage = "How much of the current billing period's post quota has been used, and what the plan allows."
)

// CreatePostInput is the create_post tool input.
type CreatePostInput struct {
	Content     string                  `json:"content" jsonschema:"The post text."`
	AccountIDs  []string                `json:"account_ids,omitempty" jsonschema:"Social account ids from list_accounts or list_profiles."`
	Platforms   []string                `json:"platforms,omitempty" jsonschema:"Alternative to account_ids: post to the account on each of these platforms in profile, e.g. [\"x\", \"linkedin\"]."`
	Profile     string                  `json:"profile,omitempty" jsonschema:"A profile name or id from list_profiles. Needed with platforms when a platform has accounts in more than one profile; if you are not sure which brand the person means, ask rather than guess."`
	ScheduledAt string                  `json:"scheduled_at,omitempty" jsonschema:"RFC 3339 timestamp to publish at. Omit to publish now."`
	Queue       string                  `json:"queue,omitempty" jsonschema:"A queue name or id from list_queues: the post goes out in that queue's next free slot instead of at scheduled_at. Its accounts must be in the queue's profile, which is also what platforms means when profile is omitted."`
	MediaURLs   []string                `json:"media_urls,omitempty" jsonschema:"Public https URLs of images or video to attach. They must be reachable from the open internet."`
	MediaIDs    []string                `json:"media_ids,omitempty" jsonschema:"Upload ids from create_upload, for files the person picked in their browser. Use this for anything on their own device; four media items in total across media_ids and media_urls."`
	X           *apiv1.XOptions         `json:"x,omitempty" jsonschema:"X-only options: a thread, a poll, who can reply, a Community, X-only text. content (X-only text), thread (replies under the first tweet, each with content and optional media_urls or media_ids), community (id or link), share_with_followers, reply_settings (following, mentioned_users, subscribers or verified), long_post (Premium only), poll (options and duration_minutes; no media with a poll)."`
	Instagram   *apiv1.InstagramOptions `json:"instagram,omitempty" jsonschema:"Instagram-only options: post_type (auto, feed, story, reel or carousel), caption (Instagram-only text), first_comment, collaborators (up to 3 usernames), user_tags, ai_generated, trial_reel (manual or performance, reels only)."`
	TikTok      *apiv1.TikTokOptions    `json:"tiktok,omitempty" jsonschema:"TikTok settings. With privacy the post is published directly, and TikTok needs disable_comment, commercial_content and music_usage_confirmed, plus disable_duet and disable_stitch for a video. Ask the person for these rather than choosing them. With draft, or with no tiktok block, it goes to the creator's TikTok inbox to finish in the app. title is a video's caption (up to 2200, replacing content) or a photo post's title (up to 90); description is a photo post's text (up to 4000); privacy is public_to_everyone, mutual_follow_friends, follower_of_creator or self_only. music_usage_confirmed is the person accepting TikTok's declaration: show them \"By posting, you agree to TikTok's Music Usage Confirmation\" (with branded_content: \"...TikTok's Branded Content Policy and Music Usage Confirmation\") and never agree for them."`
	YouTube     *apiv1.YouTubeOptions   `json:"youtube,omitempty" jsonschema:"YouTube options: title (up to 100 characters, no < or >; default the first line of content), description (replaces content, up to 5000 bytes, no < or >), privacy (public, unlisted or private; default public), made_for_kids (ask the person), tags, category_id (default 22, People & Blogs), thumbnail_url or thumbnail_id (JPEG or PNG; verified channels only). first_comment is not available yet: it is refused until Google verifies the scope it needs, so leave it out."`
}

// UpdatePostInput is the update_post tool input.
type UpdatePostInput struct {
	PostID           string                  `json:"post_id" jsonschema:"The id of a draft or scheduled post, from create_post or list_posts."`
	Content          string                  `json:"content,omitempty" jsonschema:"New text for the post. Omit to keep it."`
	ScheduledAt      string                  `json:"scheduled_at,omitempty" jsonschema:"New RFC 3339 time to publish at, at least 5 minutes ahead; on a draft, this schedules it. Omit to keep it."`
	PublishNow       bool                    `json:"publish_now,omitempty" jsonschema:"Publish the post now, after the other changes. Not with scheduled_at."`
	AddPlatforms     []string                `json:"add_platforms,omitempty" jsonschema:"Platforms to add: the account on each in profile, e.g. [\"instagram\"]."`
	AddAccountIDs    []string                `json:"add_account_ids,omitempty" jsonschema:"Specific connected account ids to add, from list_accounts."`
	RemoveAccountIDs []string                `json:"remove_account_ids,omitempty" jsonschema:"Account ids to take off the post. At least one must remain."`
	Profile          string                  `json:"profile,omitempty" jsonschema:"A profile name or id from list_profiles, saying which brand add_platforms means."`
	X                *apiv1.XOptions         `json:"x,omitempty" jsonschema:"Replaces the post's X options as a whole (see create_post); {} clears them."`
	Instagram        *apiv1.InstagramOptions `json:"instagram,omitempty" jsonschema:"Replaces the post's Instagram options as a whole; {} clears them."`
	TikTok           *apiv1.TikTokOptions    `json:"tiktok,omitempty" jsonschema:"Replaces the post's TikTok settings as a whole; {} clears them."`
	YouTube          *apiv1.YouTubeOptions   `json:"youtube,omitempty" jsonschema:"Replaces the post's YouTube options as a whole; {} clears them."`
}

// RetryPostInput is the retry_post tool input.
type RetryPostInput struct {
	PostID     string   `json:"post_id" jsonschema:"The id of a failed or partial post, from list_posts or get_post."`
	AccountIDs []string `json:"account_ids,omitempty" jsonschema:"Account ids of failed deliveries to send again. Omit to retry every account that failed."`
}

// DeletePostsInput is the delete_posts tool input.
type DeletePostsInput struct {
	IDs []string `json:"post_ids" jsonschema:"Up to 100 post ids."`
}

// CreateQueueInput is the create_queue tool input.
type CreateQueueInput struct {
	Name     string            `json:"name" jsonschema:"What to call it, e.g. Weekday mornings."`
	Profile  string            `json:"profile,omitempty" jsonschema:"The profile (name or id from list_profiles) it posts for. Omit for Default."`
	Timezone string            `json:"timezone" jsonschema:"IANA timezone the slot times are in, e.g. Europe/London. Ask the person if you do not know it."`
	Slots    []apiv1.QueueSlot `json:"slots,omitempty" jsonschema:"Weekly slots: day 0 (Sunday) to 6 (Saturday) and time as HH:MM, 24 hour, in the queue's timezone."`
	Paused   bool              `json:"paused,omitempty" jsonschema:"Create it paused, taking no posts until resumed."`
}

// UpdateQueueInput is the update_queue tool input.
type UpdateQueueInput struct {
	Queue    string             `json:"queue" jsonschema:"The queue's name or id from list_queues."`
	Name     *string            `json:"name,omitempty" jsonschema:"A new name."`
	Profile  *string            `json:"profile,omitempty" jsonschema:"Move the queue to another profile."`
	Timezone *string            `json:"timezone,omitempty" jsonschema:"A new IANA timezone."`
	Slots    *[]apiv1.QueueSlot `json:"slots,omitempty" jsonschema:"Weekly slots; this replaces every slot."`
	Active   *bool              `json:"active,omitempty" jsonschema:"false pauses the queue, true resumes it."`
	// MovePosts moves the queue's upcoming posts onto new slots.
	MovePosts bool `json:"move_posts,omitempty" jsonschema:"With new slots or a new timezone, true moves the queue's upcoming posts onto them, in the order they were going out. Posts given a time of their own, and any due within five minutes, keep their times."`
}

// QueueInput is the delete_queue tool input.
type QueueInput struct {
	Queue string `json:"queue" jsonschema:"The queue's name or id from list_queues."`
}

// ConnectAccountInput is the connect_account tool input.
type ConnectAccountInput struct {
	Platform  string `json:"platform,omitempty" jsonschema:"The network to connect: x, instagram, facebook, linkedin, tiktok, youtube, threads or bluesky. Optional with account_id."`
	Profile   string `json:"profile,omitempty" jsonschema:"A profile name or id from list_profiles to put the account in. Omit for the Default profile."`
	AccountID string `json:"account_id,omitempty" jsonschema:"A connected account's id, to sign that account in again where it is, instead of connecting a new one."`
}

// ListPostsInput is the list_posts tool input.
type ListPostsInput struct {
	Status   string `json:"status,omitempty" jsonschema:"One of draft, scheduled, pending, processing, published, failed, partial."`
	Platform string `json:"platform,omitempty" jsonschema:"Only posts to this platform, e.g. x or instagram."`
	Profile  string `json:"profile,omitempty" jsonschema:"Only posts to accounts in this profile (a name or id from list_profiles)."`
	From     string `json:"from,omitempty" jsonschema:"RFC 3339 start of the range (scheduled time for scheduled posts, creation time otherwise)."`
	To       string `json:"to,omitempty" jsonschema:"RFC 3339 end of the range."`
	Limit    int    `json:"limit,omitempty" jsonschema:"How many to return, 1 to 100 (default 25)."`
	Cursor   string `json:"cursor,omitempty" jsonschema:"next_cursor from a previous call."`
}

// PostIDInput is shared by get_post and delete_post. post_id is the remote
// server's name; id is the name this server took before, still accepted.
type PostIDInput struct {
	PostID string `json:"post_id,omitempty" jsonschema:"The post id."`
	ID     string `json:"id,omitempty" jsonschema:"The same as post_id, the name older clients send."`
}

// postID is the post the call names, by either name.
func (in PostIDInput) postID() (string, error) {
	if id := strings.TrimSpace(in.PostID); id != "" {
		return id, nil
	}
	if id := strings.TrimSpace(in.ID); id != "" {
		return id, nil
	}
	return "", errors.New("post_id is required")
}

// ListEventsInput is the list_events tool input.
type ListEventsInput struct {
	Range    string `json:"range,omitempty" jsonschema:"How far back, ending now: 24h, 7d, 30d or 90d. Defaults to 7d; events are kept 90 days."`
	Status   string `json:"status,omitempty" jsonschema:"ok, warning or error; several separated by commas."`
	Type     string `json:"type,omitempty" jsonschema:"posts, accounts or api; several separated by commas."`
	Platform string `json:"platform,omitempty" jsonschema:"Only events about an account on this platform, e.g. instagram."`
	Limit    int    `json:"limit,omitempty" jsonschema:"How many to return, 1 to 100 (default 25)."`
	Cursor   string `json:"cursor,omitempty" jsonschema:"next_cursor from a previous call."`
}

// AnalyticsInput is the get_analytics tool input.
type AnalyticsInput struct {
	Range     string `json:"range,omitempty" jsonschema:"One of 7d, 30d, 90d or 365d. Defaults to 30d."`
	Platform  string `json:"platform,omitempty" jsonschema:"Only this platform, e.g. instagram, facebook, threads, tiktok, bluesky, linkedin, youtube."`
	AccountID string `json:"account_id,omitempty" jsonschema:"Only this connected account (an id from list_accounts)."`
	Profile   string `json:"profile,omitempty" jsonschema:"Only accounts in this profile (a name or id from list_profiles)."`
	Source    string `json:"source,omitempty" jsonschema:"all (default) or postatron, to count only posts published with Postatron."`
}

// CreateUploadInput is the create_upload tool input.
type CreateUploadInput struct {
	Purpose string `json:"purpose,omitempty" jsonschema:"Optional note shown on the upload page, e.g. \"photo for your Tuesday post\", so they know what they are attaching."`
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

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolListAccounts, Description: descListAccounts}),
		func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, *apiv1.AccountList, error) {
			out, err := ops.ListAccounts(ctx)
			if err != nil {
				return handleError[*apiv1.AccountList](err)
			}
			return nil, out, nil
		})

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolListProfiles, Description: descListProfiles}),
		func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, *apiv1.ProfileList, error) {
			out, err := ops.ListProfiles(ctx)
			if err != nil {
				return handleError[*apiv1.ProfileList](err)
			}
			return nil, out, nil
		})

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolListQueues, Description: descListQueues}),
		func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, *apiv1.QueueList, error) {
			out, err := ops.ListQueues(ctx)
			if err != nil {
				return handleError[*apiv1.QueueList](err)
			}
			return nil, out, nil
		})

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolCreatePost, Description: descCreatePost}),
		func(ctx context.Context, _ *mcp.CallToolRequest, in CreatePostInput) (*mcp.CallToolResult, *apiv1.Post, error) {
			req := apiv1.CreatePostRequest{
				Content:    in.Content,
				Platforms:  in.Platforms,
				AccountIDs: in.AccountIDs,
				Profile:    in.Profile,
				MediaURLs:  in.MediaURLs,
				MediaIDs:   in.MediaIDs,
				Queue:      strings.TrimSpace(in.Queue),
				X:          in.X,
				Instagram:  in.Instagram,
				TikTok:     in.TikTok,
				YouTube:    in.YouTube,
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

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolListPosts, Description: descListPosts}),
		func(ctx context.Context, _ *mcp.CallToolRequest, in ListPostsInput) (*mcp.CallToolResult, *apiv1.PostList, error) {
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

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolGetPost, Description: descGetPost}),
		func(ctx context.Context, _ *mcp.CallToolRequest, in PostIDInput) (*mcp.CallToolResult, *apiv1.Post, error) {
			id, err := in.postID()
			if err != nil {
				return nil, nil, err
			}
			post, err := ops.GetPost(ctx, id)
			if err != nil {
				return handleError[*apiv1.Post](err)
			}
			return nil, post, nil
		})

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolDeletePosts, Description: descDeletePosts}),
		func(ctx context.Context, _ *mcp.CallToolRequest, in DeletePostsInput) (*mcp.CallToolResult, *apiv1.DeletePostsResponse, error) {
			out, err := ops.DeletePosts(ctx, apiv1.DeletePostsRequest{IDs: in.IDs})
			if err != nil {
				return handleError[*apiv1.DeletePostsResponse](err)
			}
			return nil, out, nil
		})

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolCreateQueue, Description: descCreateQueue}),
		func(ctx context.Context, _ *mcp.CallToolRequest, in CreateQueueInput) (*mcp.CallToolResult, *apiv1.Queue, error) {
			out, err := ops.CreateQueue(ctx, apiv1.CreateQueueRequest{
				Name: in.Name, Profile: strings.TrimSpace(in.Profile), Timezone: strings.TrimSpace(in.Timezone), Slots: in.Slots, Paused: in.Paused,
			})
			if err != nil {
				return handleError[*apiv1.Queue](err)
			}
			return nil, out, nil
		})

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolUpdateQueue, Description: descUpdateQueue}),
		func(ctx context.Context, _ *mcp.CallToolRequest, in UpdateQueueInput) (*mcp.CallToolResult, *apiv1.Queue, error) {
			out, err := ops.UpdateQueue(ctx, strings.TrimSpace(in.Queue), apiv1.UpdateQueueRequest{
				Name: in.Name, Profile: in.Profile, Timezone: in.Timezone, Slots: in.Slots, Active: in.Active, MovePosts: in.MovePosts,
			})
			if err != nil {
				return handleError[*apiv1.Queue](err)
			}
			return nil, out, nil
		})

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolDeleteQueue, Description: descDeleteQueue}),
		func(ctx context.Context, _ *mcp.CallToolRequest, in QueueInput) (*mcp.CallToolResult, *apiv1.DeleteQueueResponse, error) {
			out, err := ops.DeleteQueue(ctx, strings.TrimSpace(in.Queue))
			if err != nil {
				return handleError[*apiv1.DeleteQueueResponse](err)
			}
			return nil, out, nil
		})

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolDeletePost, Description: descDeletePost}),
		func(ctx context.Context, _ *mcp.CallToolRequest, in PostIDInput) (*mcp.CallToolResult, *apiv1.DeletePostResponse, error) {
			id, err := in.postID()
			if err != nil {
				return nil, nil, err
			}
			out, err := ops.DeletePost(ctx, id)
			if err != nil {
				return handleError[*apiv1.DeletePostResponse](err)
			}
			return nil, out, nil
		})

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolUpdatePost, Description: descUpdatePost}),
		func(ctx context.Context, _ *mcp.CallToolRequest, in UpdatePostInput) (*mcp.CallToolResult, *apiv1.Post, error) {
			req := apiv1.UpdatePostRequest{
				AddPlatforms:     in.AddPlatforms,
				AddAccountIDs:    in.AddAccountIDs,
				RemoveAccountIDs: in.RemoveAccountIDs,
				Profile:          in.Profile,
				PublishNow:       in.PublishNow,
				X:                in.X,
				Instagram:        in.Instagram,
				TikTok:           in.TikTok,
				YouTube:          in.YouTube,
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

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolRetryPost, Description: descRetryPost}),
		func(ctx context.Context, _ *mcp.CallToolRequest, in RetryPostInput) (*mcp.CallToolResult, *apiv1.Post, error) {
			id := strings.TrimSpace(in.PostID)
			if id == "" {
				return nil, nil, errors.New("post_id is required")
			}
			post, err := ops.RetryPost(ctx, id, apiv1.RetryPostRequest{AccountIDs: in.AccountIDs})
			if err != nil {
				return handleError[*apiv1.Post](err)
			}
			return nil, post, nil
		})

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolConnectAccount, Description: descConnectAccount}),
		func(ctx context.Context, _ *mcp.CallToolRequest, in ConnectAccountInput) (*mcp.CallToolResult, *apiv1.ConnectLink, error) {
			out, err := ops.ConnectAccount(ctx, apiv1.ConnectAccountRequest{
				Platform: strings.TrimSpace(in.Platform), Profile: strings.TrimSpace(in.Profile), AccountID: strings.TrimSpace(in.AccountID),
			})
			if err != nil {
				return handleError[*apiv1.ConnectLink](err)
			}
			return nil, out, nil
		})

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolCreateUpload, Description: descCreateUpload}),
		func(ctx context.Context, _ *mcp.CallToolRequest, in CreateUploadInput) (*mcp.CallToolResult, *apiv1.Upload, error) {
			out, err := ops.CreateUpload(ctx, apiv1.CreateUploadRequest{Purpose: in.Purpose})
			if err != nil {
				return handleError[*apiv1.Upload](err)
			}
			return nil, out, nil
		})

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolGetUpload, Description: descGetUpload}),
		func(ctx context.Context, _ *mcp.CallToolRequest, in UploadIDInput) (*mcp.CallToolResult, *apiv1.Upload, error) {
			out, err := ops.GetUpload(ctx, strings.TrimSpace(in.ID))
			if err != nil {
				return handleError[*apiv1.Upload](err)
			}
			return nil, out, nil
		})

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolListEvents, Description: descListEvents}),
		func(ctx context.Context, _ *mcp.CallToolRequest, in ListEventsInput) (*mcp.CallToolResult, *apiv1.EventList, error) {
			out, err := ops.ListEvents(ctx, apiv1.ListEventsQuery{
				Range: in.Range, Status: in.Status, Type: in.Type, Platform: in.Platform, Limit: in.Limit, Cursor: in.Cursor,
			})
			if err != nil {
				return handleError[*apiv1.EventList](err)
			}
			return nil, out, nil
		})

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolGetAnalytics, Description: descGetAnalytics}),
		func(ctx context.Context, _ *mcp.CallToolRequest, in AnalyticsInput) (*mcp.CallToolResult, *apiv1.AnalyticsReport, error) {
			out, err := ops.GetAnalytics(ctx, apiv1.AnalyticsQuery{
				Range: in.Range, Platform: in.Platform, AccountID: in.AccountID, Profile: in.Profile, Source: in.Source,
			})
			if err != nil {
				return handleError[*apiv1.AnalyticsReport](err)
			}
			return nil, out, nil
		})

	mcp.AddTool(server, describe(&mcp.Tool{Name: ToolGetUsage, Description: descGetUsage}),
		func(ctx context.Context, _ *mcp.CallToolRequest, _ EmptyInput) (*mcp.CallToolResult, *apiv1.UsageReport, error) {
			out, err := ops.GetUsage(ctx)
			if err != nil {
				return handleError[*apiv1.UsageReport](err)
			}
			return nil, out, nil
		})

	return server
}

// toolNotes gives every tool the same title and hints as the remote server,
// so a client treats each tool alike whichever server it came from. Claude
// runs read-only tools without asking and always confirms a destructive one;
// publishing to someone's real accounts counts as destructive.
var toolNotes = map[string]mcp.ToolAnnotations{
	ToolListAccounts: readOnly("List connected accounts"),
	ToolListProfiles: readOnly("List profiles"),
	ToolListQueues:   readOnly("List queues"),
	ToolCreatePost:   destructive("Create or schedule a post", false),
	ToolListPosts:    readOnly("List posts"),
	ToolGetPost:      readOnly("Get one post"),
	ToolDeletePost:   destructive("Delete a post", true),
	ToolDeletePosts:  destructive("Delete several posts", true),
	ToolUpdatePost:   destructive("Change, schedule or publish a post", false),
	// A retry publishes to real accounts, so it is confirmed like a post.
	ToolRetryPost: destructive("Retry a failed post", false),
	// A queue changes the person's settings, not anything anyone else sees.
	ToolCreateQueue: {Title: "Create a queue", DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	ToolUpdateQueue: {Title: "Change a queue", DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	ToolDeleteQueue: {Title: "Delete a queue", DestructiveHint: boolPtr(true), IdempotentHint: true, OpenWorldHint: boolPtr(false)},
	// A link only; nothing changes until the person finishes signing in.
	ToolConnectAccount: {Title: "Connect a social account", DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(true)},
	ToolCreateUpload:   {Title: "Ask the person to attach a file", DestructiveHint: boolPtr(false), OpenWorldHint: boolPtr(false)},
	ToolGetUpload:      readOnly("Check whether a file has been attached"),
	ToolListEvents:     readOnly("List recent events"),
	ToolGetAnalytics:   readOnly("Get post analytics"),
	ToolGetUsage:       readOnly("Get plan usage"),
}

// readOnly is the remote server's readOnly(): reads only, safe to repeat.
func readOnly(title string) mcp.ToolAnnotations {
	return mcp.ToolAnnotations{Title: title, ReadOnlyHint: true, DestructiveHint: boolPtr(false), IdempotentHint: true, OpenWorldHint: boolPtr(true)}
}

// destructive is the remote server's destructive(): visible to other people
// or not undoable.
func destructive(title string, idempotent bool) mcp.ToolAnnotations {
	return mcp.ToolAnnotations{Title: title, DestructiveHint: boolPtr(true), IdempotentHint: idempotent, OpenWorldHint: boolPtr(true)}
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
