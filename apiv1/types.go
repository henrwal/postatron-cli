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
type CreatePostRequest struct {
	Content     string     `json:"content"`
	AccountIDs  []string   `json:"account_ids,omitempty"`
	Platforms   []string   `json:"platforms,omitempty"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
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
	From     *time.Time
	To       *time.Time
	Limit    int
	Cursor   string
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

// Account is a connected social account.
type Account struct {
	ID          string    `json:"id"`
	Platform    string    `json:"platform"`
	Username    string    `json:"username"`
	DisplayName string    `json:"display_name"`
	Status      string    `json:"status"`
	ConnectedAt time.Time `json:"connected_at"`
}

// AccountList is the response of GET /v1/accounts.
type AccountList struct {
	Data []Account `json:"data"`
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
	Overage          UsageOverage     `json:"overage"`
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

// UsageCounter is a capped counter. Limit is the amount included in the plan;
// the hard stop including any overage allowance is reported under Overage.
type UsageCounter struct {
	Used      int `json:"used"`
	Limit     int `json:"limit"`
	Remaining int `json:"remaining"`
}

// UsageLinkCounter counts link-containing destination-posts across all platforms.
type UsageLinkCounter struct {
	Used int `json:"used"`
}

// UsageOverage reports metered overage consumed this period and the hard
// ceilings (plan cap plus overage allowance on monthly plans).
type UsageOverage struct {
	Enabled                     bool    `json:"enabled"`
	DestinationPosts            int     `json:"destination_posts"`
	XLinkPosts                  int     `json:"x_link_posts"`
	DestinationPostsCeiling     int     `json:"destination_posts_ceiling"`
	XLinkPostsCeiling           int     `json:"x_link_posts_ceiling"`
	DestinationPostUnitPriceUSD float64 `json:"destination_post_unit_price_usd"`
	XLinkPostUnitPriceUSD       float64 `json:"x_link_post_unit_price_usd"`
}

// UsageRateLimits echoes the per-key limits applied to this plan.
type UsageRateLimits struct {
	WritesPerHour  int `json:"writes_per_hour"`
	ReadsPerMinute int `json:"reads_per_minute"`
}

// Operations is the six-operation surface. The Lambda implements it in
// process; Client implements it over HTTP; the MCP server and CLI consume it.
type Operations interface {
	CreatePost(ctx context.Context, req CreatePostRequest) (*Post, error)
	ListPosts(ctx context.Context, query ListPostsQuery) (*PostList, error)
	GetPost(ctx context.Context, id string) (*Post, error)
	DeletePost(ctx context.Context, id string) (*DeletePostResponse, error)
	ListAccounts(ctx context.Context) (*AccountList, error)
	GetUsage(ctx context.Context) (*UsageReport, error)
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
