// Package apiv1 holds the public API's request and response types. They are
// shared by the Lambda that serves the API, the Go client used by the CLI and
// the MCP server, and the usage report embedded in the dashboard.
package apiv1

import (
	"context"
	"fmt"
	"time"
)

// Post statuses as exposed by the API (lower case, "published" instead of the
// internal SUCCESS).
const (
	// StatusDraft is a post saved without a time; it goes nowhere until it is
	// scheduled or published with PATCH /v1/posts/{id}.
	StatusDraft      = "draft"
	StatusScheduled  = "scheduled"
	StatusPending    = "pending"
	StatusProcessing = "processing"
	StatusPublished  = "published"
	StatusFailed     = "failed"
	StatusPartial    = "partial"
	StatusCancelled  = "cancelled"
)

// Error codes returned in the error envelope.
const (
	CodeUnauthorized         = "unauthorized"
	CodeInsufficientScope    = "insufficient_scope"
	CodeSubscriptionRequired = "subscription_required"
	CodeRateLimited          = "rate_limited"
	CodeQuotaExceeded        = "quota_exceeded"
	CodeValidation           = "validation_error"
	CodeNotFound             = "not_found"
	CodeInternal             = "internal_error"
)

// ErrorBody is the JSON envelope for every non-2xx response.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// ErrorDetail carries a stable machine-readable code and a human message.
type ErrorDetail struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

// CreatePostRequest is the body of POST /v1/posts. Exactly one of AccountIDs
// or Platforms must be set. ScheduledAt omitted publishes immediately.
//
// Profile (an id or a name from ListProfiles) limits Platforms to the accounts
// in that profile. It is required when a platform has an account in more than
// one profile, so a post never fans out to every client by accident.
//
// MediaURLs are public https links the server downloads; MediaIDs are upload
// ids from CreateUpload. Instagram and TikTok need at least one image or
// video, YouTube a video.
type CreatePostRequest struct {
	Content     string     `json:"content"`
	AccountIDs  []string   `json:"account_ids,omitempty"`
	Platforms   []string   `json:"platforms,omitempty"`
	Profile     string     `json:"profile,omitempty"`
	MediaURLs   []string   `json:"media_urls,omitempty"`
	MediaIDs    []string   `json:"media_ids,omitempty"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
}

// UpdatePostRequest is the body of PATCH /v1/posts/{id}. Every field is
// optional and anything left out keeps its value. A draft or a scheduled post
// can change; once it has started publishing it is fixed. A draft stays a
// draft unless ScheduledAt or PublishNow says when it goes out.
type UpdatePostRequest struct {
	// Content replaces the post's text.
	Content *string `json:"content,omitempty"`
	// ScheduledAt moves the post, or schedules a draft, at least 5 minutes ahead.
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	// PublishNow publishes the post straight away, after any other change in
	// the same request. It cannot be combined with ScheduledAt.
	PublishNow bool `json:"publish_now,omitempty"`
	// AddPlatforms adds the account on each platform (in Profile), the way
	// CreatePostRequest.Platforms chooses them.
	AddPlatforms []string `json:"add_platforms,omitempty"`
	// AddAccountIDs adds specific connected accounts.
	AddAccountIDs []string `json:"add_account_ids,omitempty"`
	// RemoveAccountIDs takes accounts off the post; at least one must remain.
	RemoveAccountIDs []string `json:"remove_account_ids,omitempty"`
	// Profile chooses which brand AddPlatforms means.
	Profile string `json:"profile,omitempty"`
}

// ConnectAccountRequest is the body of POST /v1/accounts/connect.
type ConnectAccountRequest struct {
	// Platform is the network to connect: x, instagram, facebook, linkedin,
	// tiktok, youtube, threads or bluesky.
	Platform string `json:"platform"`
	// Profile is the profile (name or id) the new account goes into.
	Profile string `json:"profile,omitempty"`
}

// ConnectLink is a link the person opens to connect a social account. It
// opens Postatron and starts that platform's sign-in; nothing is connected
// until they finish it there.
type ConnectLink struct {
	Platform   string `json:"platform"`
	ConnectURL string `json:"connect_url"`
	Profile    string `json:"profile,omitempty"`
	Hint       string `json:"hint"`
}

// Delivery is the per-platform state of a post.
type Delivery struct {
	AccountID string `json:"account_id"`
	Platform  string `json:"platform"`
	Username  string `json:"username"`
	Status    string `json:"status"`
	URL       string `json:"url,omitempty"`
	Error     string `json:"error,omitempty"`
}

// Post is the API representation of a post.
type Post struct {
	ID           string     `json:"id"`
	Status       string     `json:"status"`
	Content      string     `json:"content"`
	MediaType    string     `json:"media_type"`
	ContainsLink bool       `json:"contains_link"`
	ScheduledAt  *time.Time `json:"scheduled_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    *time.Time `json:"updated_at,omitempty"`
	Deliveries   []Delivery `json:"deliveries"`
}

// ListPostsQuery holds the filters accepted by GET /v1/posts.
type ListPostsQuery struct {
	Status   string
	Platform string
	// Profile is a profile id or name; only posts to its accounts are listed.
	Profile string
	From    *time.Time
	To      *time.Time
	Limit   int
	Cursor  string
}

// PostList is the response of GET /v1/posts.
type PostList struct {
	Data       []Post `json:"data"`
	NextCursor string `json:"next_cursor,omitempty"`
}

// DeletePostResponse is the response of DELETE /v1/posts/{id}.
type DeletePostResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Cancelled bool   `json:"cancelled"`
	Message   string `json:"message,omitempty"`
}

// Account is a connected social account. Every account belongs to exactly
// one profile, and a profile holds at most one account per platform.
type Account struct {
	ID          string    `json:"id"`
	Platform    string    `json:"platform"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status"`
	ProfileID   string    `json:"profile_id"`
	ProfileName string    `json:"profile_name"`
	ConnectedAt time.Time `json:"connected_at"`
}

// AccountList is the response of GET /v1/accounts.
type AccountList struct {
	Data []Account `json:"data"`
}

// Profile groups connected accounts, typically one per brand or client. Every
// account has a Default profile; paid plans can add more.
type Profile struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	IsDefault bool      `json:"is_default"`
	Accounts  []Account `json:"accounts"`
}

// ProfileList is the response of GET /v1/profiles.
type ProfileList struct {
	Data []Profile `json:"data"`
}

// UsageReport is the response of GET /v1/usage. The dashboard renders the
// same structure.
type UsageReport struct {
	Period           UsagePeriod      `json:"period"`
	Plan             UsagePlan        `json:"plan"`
	DestinationPosts UsageCounter     `json:"destination_posts"`
	LinkPosts        UsageLinkCounter `json:"link_posts"`
	XLinkPosts       UsageCounter     `json:"x_link_posts"`
	ByPlatform       map[string]int   `json:"by_platform"`
	RateLimits       UsageRateLimits  `json:"rate_limits"`
}

// UsagePeriod is the calendar month the counters belong to.
type UsagePeriod struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// UsagePlan summarises the subscription the quota comes from.
type UsagePlan struct {
	Code                 string `json:"code"`
	Name                 string `json:"name"`
	Interval             string `json:"interval"`
	Status               string `json:"status"`
	ConnectedAccounts    int    `json:"connected_accounts"`
	MaxConnectedAccounts int    `json:"max_connected_accounts"`
}

// UsageCounter is a capped counter. Limit is what the plan includes each
// month and is also the hard stop: there is no overage on any plan.
type UsageCounter struct {
	Used      int `json:"used"`
	Limit     int `json:"limit"`
	Remaining int `json:"remaining"`
}

// UsageLinkCounter counts link-containing destination-posts across all platforms.
type UsageLinkCounter struct {
	Used int `json:"used"`
}

// UsageRateLimits echoes the per-key limits applied to this plan.
type UsageRateLimits struct {
	WritesPerMinute  int `json:"writes_per_minute"`
	ReadsPerMinute   int `json:"reads_per_minute"`
	UploadsPerMinute int `json:"uploads_per_minute"`
	// WritesPerHour is WritesPerMinute * 60, kept for clients written before
	// the write window became per minute.
	//
	// Deprecated: use WritesPerMinute.
	WritesPerHour int `json:"writes_per_hour"`
}

// AnalyticsQuery holds the filters accepted by GET /v1/analytics.
type AnalyticsQuery struct {
	// Range is 7d, 30d or 90d (default 30d).
	Range string
	// Platform limits the report to one platform, e.g. instagram.
	Platform string
	// AccountID limits the report to one connected account.
	AccountID string
	// Profile is a profile id or name.
	Profile string
	// Source is "all" (default) or "postatron" for posts made with Postatron only.
	Source string
}

// AnalyticsReport is the response of GET /v1/analytics. A metric a platform
// does not report is null, never a negative number.
type AnalyticsReport struct {
	Range       AnalyticsRange         `json:"range"`
	Summary     AnalyticsSummary       `json:"summary"`
	PerPlatform []AnalyticsPlatformRow `json:"per_platform"`
	TopPosts    []AnalyticsPost        `json:"top_posts"`
	Accounts    []AnalyticsAccount     `json:"accounts"`
	GeneratedAt string                 `json:"generated_at"`
	Notes       []string               `json:"notes,omitempty"`
}

// AnalyticsRange is the window the report covers.
type AnalyticsRange struct {
	Key   string `json:"key"`
	Start string `json:"start"`
	End   string `json:"end"`
}

// AnalyticsSummary holds the headline numbers.
type AnalyticsSummary struct {
	EngagementRate      float64 `json:"engagement_rate_percent"`
	EngagementRateBasis string  `json:"engagement_rate_basis"`
	Engagements         int64   `json:"engagements"`
	Reach               int64   `json:"reach"`
	Followers           int64   `json:"followers"`
	PostsThisPeriod     int     `json:"posts_this_period"`
}

// AnalyticsPlatformRow totals one platform.
type AnalyticsPlatformRow struct {
	Platform string `json:"platform"`
	Posts    int    `json:"posts"`
	Likes    int64  `json:"likes"`
	Comments int64  `json:"comments"`
	Shares   int64  `json:"shares"`
	Views    int64  `json:"views"`
}

// AnalyticsPost is one of the best performing posts in the window.
type AnalyticsPost struct {
	Platform      string `json:"platform"`
	Username      string `json:"username"`
	URL           string `json:"url"`
	Text          string `json:"text"`
	PublishedAt   string `json:"published_at"`
	Likes         *int64 `json:"likes"`
	Comments      *int64 `json:"comments"`
	Shares        *int64 `json:"shares"`
	Views         *int64 `json:"views"`
	Reach         *int64 `json:"reach"`
	Saves         *int64 `json:"saves"`
	Impressions   *int64 `json:"impressions"`
	Clicks        *int64 `json:"clicks"`
	Follows       *int64 `json:"follows"`
	Engagement    int64  `json:"engagement"`
	FromPostatron bool   `json:"posted_with_postatron"`
}

// AnalyticsAccount says whether an account's numbers could be read.
type AnalyticsAccount struct {
	ID        string `json:"id"`
	Platform  string `json:"platform"`
	Username  string `json:"username"`
	Followers int64  `json:"followers"`
	Supported bool   `json:"supported"`
	// Status is "ok", "unsupported", "reconnect_required", "unavailable" or "error".
	Status string `json:"status"`
	Reason string `json:"reason,omitempty"`
}

// CreateUploadRequest is the body of POST /v1/media/uploads.
type CreateUploadRequest struct {
	// Purpose is shown to the person on the upload page, e.g. "the launch video".
	Purpose string `json:"purpose,omitempty"`
}

// Upload is a slot a person drops a file into from the dashboard, for an
// agent that cannot send the bytes itself. Pass its ID to CreatePost as
// MediaIDs once Status is "ready".
type Upload struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	UploadURL string `json:"upload_url,omitempty"`
	Filename  string `json:"filename,omitempty"`
	ExpiresAt string `json:"expires_at"`
	Hint      string `json:"hint,omitempty"`
}

// Operations is the public API surface. Client implements it over HTTP; the
// MCP server and CLI consume it.
type Operations interface {
	CreatePost(ctx context.Context, req CreatePostRequest) (*Post, error)
	ListPosts(ctx context.Context, query ListPostsQuery) (*PostList, error)
	GetPost(ctx context.Context, id string) (*Post, error)
	UpdatePost(ctx context.Context, id string, req UpdatePostRequest) (*Post, error)
	DeletePost(ctx context.Context, id string) (*DeletePostResponse, error)
	ListAccounts(ctx context.Context) (*AccountList, error)
	ConnectAccount(ctx context.Context, req ConnectAccountRequest) (*ConnectLink, error)
	ListProfiles(ctx context.Context) (*ProfileList, error)
	GetUsage(ctx context.Context) (*UsageReport, error)
	GetAnalytics(ctx context.Context, query AnalyticsQuery) (*AnalyticsReport, error)
	CreateUpload(ctx context.Context, req CreateUploadRequest) (*Upload, error)
	GetUpload(ctx context.Context, id string) (*Upload, error)
}

// APIError is returned by Client for non-2xx responses.
type APIError struct {
	Status     int
	Code       string
	Message    string
	Details    map[string]any
	RetryAfter time.Duration
}

func (e *APIError) Error() string {
	if e.Code == "" {
		return fmt.Sprintf("api error (HTTP %d): %s", e.Status, e.Message)
	}
	return fmt.Sprintf("%s: %s", e.Code, e.Message)
}
