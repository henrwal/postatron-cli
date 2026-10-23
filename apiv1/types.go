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
	// CodeWorkspaceForbidden means the key or connection was made for a team
	// workspace its holder can no longer work in: they left or were removed,
	// or the owner's plan no longer includes teams.
	CodeWorkspaceForbidden = "workspace_forbidden"
	CodeRateLimited        = "rate_limited"
	CodeQuotaExceeded      = "quota_exceeded"
	CodeValidation         = "validation_error"
	CodeNotFound           = "not_found"
	CodeInternal           = "internal_error"
	// CodeReconnectRequired means an account the request would post to cannot
	// post until the person connects it again (ConnectAccount). Details name
	// the accounts in account_ids.
	CodeReconnectRequired = "reconnect_required"
	// CodeConflict means the post is not in a state the request can act on,
	// such as editing a post already published or retrying one that did not
	// fail.
	CodeConflict = "conflict"
)

// Delivery error codes: why one account of a post failed (Delivery.ErrorCode),
// and so what to do about it.
const (
	// ErrorReconnect: the account's access is gone. Connect it again
	// (ConnectAccount), then RetryPost.
	ErrorReconnect = "reconnect"
	// ErrorRateLimited: the platform asked Postatron to slow down, or a daily
	// limit was reached. RetryPost later.
	ErrorRateLimited = "rate_limited"
	// ErrorPlatformUnavailable: the platform failed or timed out. RetryPost.
	ErrorPlatformUnavailable = "platform_unavailable"
	// ErrorMediaRejected: the platform would not take the image or video. A
	// retry sends the same file; make a new post with another.
	ErrorMediaRejected = "media_rejected"
	// ErrorPolicy: the platform's rules refused it, such as TikTok posting
	// only to private accounts for an unaudited app, or the account had no
	// plan when a retry ran. Error says what would change it.
	ErrorPolicy = "policy"
	// ErrorValidation: the post breaks a platform rule, such as a caption
	// that is too long or a duplicate. Make a new post with the change.
	ErrorValidation = "validation"
	// ErrorPostatron: something went wrong on Postatron's side. RetryPost.
	ErrorPostatron = "postatron"
	// ErrorUnknown: a refusal Postatron does not recognise; Error has the
	// platform's words.
	ErrorUnknown = "unknown"
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
	// Queue (a name or id from GET /v1/queues) schedules the post for that
	// queue's next free slot instead of ScheduledAt. Its accounts must all be
	// in the queue's profile, which is also what Platforms means when Profile
	// is left out.
	Queue string `json:"queue,omitempty"`
	// X, Instagram, TikTok and YouTube hold the options only that platform
	// has. Each needs an account on its platform among the post's accounts.
	X         *XOptions         `json:"x,omitempty"`
	Instagram *InstagramOptions `json:"instagram,omitempty"`
	TikTok    *TikTokOptions    `json:"tiktok,omitempty"`
	YouTube   *YouTubeOptions   `json:"youtube,omitempty"`
}

// X reply audiences for XOptions.ReplySettings. Empty means everyone.
const (
	XRepliesFollowing      = "following"
	XRepliesMentionedUsers = "mentioned_users"
	XRepliesSubscribers    = "subscribers"
	XRepliesVerified       = "verified"
)

// XOptions are the settings only X has.
type XOptions struct {
	// Content replaces the post's text on X only.
	Content string `json:"content,omitempty"`
	// Thread adds replies under the first tweet, in order: Thread[0] is the
	// second tweet. A thread cannot go into a Community.
	Thread []XThreadTweet `json:"thread,omitempty"`
	// Community is the id of the X Community to post into, or its link
	// (https://x.com/i/communities/<id>).
	Community string `json:"community,omitempty"`
	// ShareWithFollowers also shows a Community post on followers' timelines.
	ShareWithFollowers bool `json:"share_with_followers,omitempty"`
	// ReplySettings limits who can reply: following, mentioned_users,
	// subscribers or verified. Empty lets everyone reply.
	ReplySettings string `json:"reply_settings,omitempty"`
	// LongPost allows tweets over 280 characters, which X accepts from
	// Premium accounts only.
	LongPost bool `json:"long_post,omitempty"`
	// Poll makes the first tweet a poll. A poll cannot carry media.
	Poll *XPoll `json:"poll,omitempty"`
}

// XThreadTweet is one reply in a thread. Media works as on the post itself;
// when a post is read back, MediaURLs holds the media it carries.
type XThreadTweet struct {
	Content   string   `json:"content"`
	MediaURLs []string `json:"media_urls,omitempty"`
	MediaIDs  []string `json:"media_ids,omitempty"`
}

// XPoll is a poll on the first tweet: 2 to 4 options of up to 25
// characters, open for 5 minutes to 7 days (default one day).
type XPoll struct {
	Options         []string `json:"options"`
	DurationMinutes int      `json:"duration_minutes,omitempty"`
}

// InstagramOptions are the settings only Instagram has.
type InstagramOptions struct {
	// PostType is auto (the default: chosen from the media), feed, story,
	// reel or carousel.
	PostType string `json:"post_type,omitempty"`
	// Caption replaces the post's text on Instagram only.
	Caption string `json:"caption,omitempty"`
	// FirstComment is posted as the first comment, often for hashtags. Not
	// on stories.
	FirstComment string `json:"first_comment,omitempty"`
	// Collaborators are up to 3 usernames invited to co-author the post.
	Collaborators []string `json:"collaborators,omitempty"`
	// UserTags tag people on images.
	UserTags []InstagramUserTag `json:"user_tags,omitempty"`
	// AIGenerated asks Instagram to show its AI-content label.
	AIGenerated bool `json:"ai_generated,omitempty"`
	// TrialReel shares a reel with non-followers first: manual (graduate it
	// by hand) or performance (Instagram graduates it if it does well).
	TrialReel string `json:"trial_reel,omitempty"`
}

// InstagramUserTag tags a username on one image, at X and Y from 0 to 1
// across and down it. MediaIndex counts the post's media from 0.
type InstagramUserTag struct {
	Username   string  `json:"username"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
	MediaIndex int     `json:"media_index,omitempty"`
}

// TikTok privacy levels for TikTokOptions.Privacy.
const (
	TikTokPublic    = "public_to_everyone"
	TikTokFriends   = "mutual_follow_friends"
	TikTokFollowers = "follower_of_creator"
	TikTokPrivate   = "self_only"
)

// TikTokOptions are the settings TikTok asks for. A post is published
// directly once Privacy is given, and TikTok then needs the creator's choice
// of each of Privacy, DisableComment, CommercialContent and
// MusicUsageConfirmed, plus DisableDuet and DisableStitch for a video, and
// one of YourBrand or BrandedContent when CommercialContent is true.
//
// With Draft, or with the whole block left out, the post goes to the
// creator's TikTok inbox instead, and they finish it in the app: a draft
// takes only Title and Description (photos), and a video draft takes
// neither.
type TikTokOptions struct {
	// Title is a video's caption, up to 2200 characters, replacing the
	// post's text there; or a photo post's title, up to 90. TikTok counts
	// characters in UTF-16, so an emoji counts as two.
	Title string `json:"title,omitempty"`
	// Description is a photo post's long description, up to 4000
	// characters; it replaces the post's text there.
	Description string `json:"description,omitempty"`
	// Draft uploads to the creator's TikTok inbox instead of publishing.
	Draft bool `json:"draft,omitempty"`
	// Privacy is public_to_everyone, mutual_follow_friends,
	// follower_of_creator or self_only.
	Privacy           string `json:"privacy,omitempty"`
	DisableComment    *bool  `json:"disable_comment,omitempty"`
	DisableDuet       *bool  `json:"disable_duet,omitempty"`
	DisableStitch     *bool  `json:"disable_stitch,omitempty"`
	CommercialContent *bool  `json:"commercial_content,omitempty"`
	// YourBrand discloses promotion of the creator's own business.
	YourBrand *bool `json:"your_brand,omitempty"`
	// BrandedContent discloses a paid partnership; it cannot be private.
	BrandedContent *bool `json:"branded_content,omitempty"`
	// MusicUsageConfirmed is the person accepting TikTok's declaration,
	// which TikTok requires before it publishes: "By posting, you agree to
	// TikTok's Music Usage Confirmation", or with BrandedContent "...TikTok's
	// Branded Content Policy and Music Usage Confirmation". Show it to them;
	// never agree for them.
	MusicUsageConfirmed *bool `json:"music_usage_confirmed,omitempty"`
	// AutoAddMusic lets TikTok add recommended music to a photo post.
	AutoAddMusic bool `json:"auto_add_music,omitempty"`
	// AIGenerated asks TikTok to label the post as AI-generated.
	AIGenerated bool `json:"ai_generated,omitempty"`
}

// YouTube privacy statuses for YouTubeOptions.Privacy.
const (
	YouTubePublic   = "public"
	YouTubeUnlisted = "unlisted"
	YouTubePrivate  = "private"
)

// YouTubeOptions are the settings only YouTube has. Leave the block out to
// upload a public video titled with the first line of the content, described
// by the content, in People & Blogs.
type YouTubeOptions struct {
	// Title is up to 100 characters, without < or >. Left out, it is the
	// first line of the content.
	Title string `json:"title,omitempty"`
	// Description replaces the content as the video's description: up to
	// 5000 bytes, without < or >.
	Description string `json:"description,omitempty"`
	// Privacy is public (the default), unlisted or private.
	Privacy string `json:"privacy,omitempty"`
	// MadeForKids is the uploader's declaration that the video is made for
	// children, which YouTube asks of every upload. Ask the person rather
	// than choosing; left out, nothing is declared and the channel's default
	// audience setting, if it has one, applies.
	MadeForKids *bool `json:"made_for_kids,omitempty"`
	// Tags are up to 500 characters in all, counting a comma between tags
	// and quotes around a tag with a space in it.
	Tags []string `json:"tags,omitempty"`
	// CategoryID is a YouTube category id: 1 Film & Animation, 2 Autos &
	// Vehicles, 10 Music, 15 Pets & Animals, 17 Sports, 19 Travel & Events,
	// 20 Gaming, 22 People & Blogs (the default), 23 Comedy, 24
	// Entertainment, 25 News & Politics, 26 Howto & Style, 27 Education, 28
	// Science & Technology or 29 Nonprofits & Activism.
	CategoryID string `json:"category_id,omitempty"`
	// FirstComment is posted on the video once it is up, up to 10000
	// characters. The channel must have allowed first comments in the
	// dashboard composer, and a video made for kids takes no comments.
	FirstComment string `json:"first_comment,omitempty"`
	// ThumbnailURL (a public https link to a JPEG or PNG) or ThumbnailID
	// (an upload id from CreateUpload) sets a custom thumbnail, which
	// YouTube allows only on verified channels. When a post is read back,
	// ThumbnailURL holds the thumbnail it carries.
	ThumbnailURL string `json:"thumbnail_url,omitempty"`
	ThumbnailID  string `json:"thumbnail_id,omitempty"`
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
	// X, Instagram, TikTok and YouTube replace that platform's options as a
	// whole; an empty object ({}) clears them.
	X         *XOptions         `json:"x,omitempty"`
	Instagram *InstagramOptions `json:"instagram,omitempty"`
	TikTok    *TikTokOptions    `json:"tiktok,omitempty"`
	YouTube   *YouTubeOptions   `json:"youtube,omitempty"`
}

// ConnectAccountRequest is the body of POST /v1/accounts/connect.
type ConnectAccountRequest struct {
	// Platform is the network to connect: x, instagram, facebook, linkedin,
	// tiktok, youtube, threads or bluesky.
	Platform string `json:"platform"`
	// Profile is the profile (name or id) the new account goes into.
	Profile string `json:"profile,omitempty"`
	// AccountID signs a connected account in again instead, for one whose
	// access has gone (status reconnect_required, or a failed delivery with
	// error_code reconnect). It keeps its profile, and Platform may be left
	// out. Asking for a platform whose account in Profile needs reconnecting
	// does the same.
	AccountID string `json:"account_id,omitempty"`
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

// Delivery is the per-platform state of a post. Status is one of draft,
// scheduled, pending, processing, published or failed: a draft's or a
// scheduled post's accounts read as the post does until it is sent.
type Delivery struct {
	AccountID string `json:"account_id"`
	Platform  string `json:"platform"`
	Username  string `json:"username"`
	Status    string `json:"status"`
	URL       string `json:"url,omitempty"`
	// Error says why a delivery failed. On a published one it says what
	// did not go through after the post itself, such as a first comment or
	// a YouTube thumbnail, and Warning is true.
	Error string `json:"error,omitempty"`
	// ErrorCode is why a failed delivery failed, one of the Error* codes,
	// so an agent can act on it without reading Error. Empty unless failed.
	ErrorCode string `json:"error_code,omitempty"`
	// Warning is true on a published delivery that carries an Error: the
	// post is live, and Error is about something after it.
	Warning bool `json:"warning,omitempty"`
}

// Unpublished reasons for PostUnpublished.Reason.
const (
	// UnpublishedNoPlan: the post came due while the account had no plan,
	// so it went back to the drafts instead of out.
	UnpublishedNoPlan = "no_plan"
)

// PostUnpublished is set on a draft that came due and went back to the
// drafts instead of out. Scheduling or publishing it again clears it.
type PostUnpublished struct {
	Reason string    `json:"reason"`
	DueAt  time.Time `json:"due_at"`
}

// RetryPostRequest is the optional body of POST /v1/posts/{id}/retry.
// AccountIDs names the failed accounts to send again; empty sends every one
// that failed. Accounts that published are never sent again.
type RetryPostRequest struct {
	AccountIDs []string `json:"account_ids,omitempty"`
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
	// Source is where the post was made: dashboard, api or mcp. Empty on
	// posts from before it was recorded.
	Source string `json:"source,omitempty"`
	// CreatedBy is the person who made the post, and UpdatedBy whoever last
	// changed it, when someone has. They differ in a team workspace.
	CreatedBy *PostAuthor `json:"created_by,omitempty"`
	UpdatedBy *PostAuthor `json:"updated_by,omitempty"`
	// Queue is the queue that chose the post's time, when one did.
	Queue *PostQueue `json:"queue,omitempty"`
	// Unpublished is set on a draft that came due without a plan and went
	// back to the drafts: why, and when it was due.
	Unpublished *PostUnpublished `json:"unpublished,omitempty"`
	// The platform options the post was saved with, when it has any.
	X         *XOptions         `json:"x,omitempty"`
	Instagram *InstagramOptions `json:"instagram,omitempty"`
	TikTok    *TikTokOptions    `json:"tiktok,omitempty"`
	YouTube   *YouTubeOptions   `json:"youtube,omitempty"`
}

// PostAuthor names a person on the account. Name is empty when the person
// has since left the team or deleted their account.
type PostAuthor struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
}

// PostQueue names the queue a post came from. Name is empty when the queue
// has since been deleted.
type PostQueue struct {
	ID   string `json:"id"`
	Name string `json:"name,omitempty"`
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
//
// Deleted is true when the post is gone from Postatron. For a scheduled post
// that also means Cancelled: it will never go out. A published post is only
// removed from Postatron and stays on the platforms; Message says so. A post
// being published right now, or due within two minutes, is refused with
// Deleted false and Message saying why.
type DeletePostResponse struct {
	ID        string `json:"id"`
	Status    string `json:"status"`
	Deleted   bool   `json:"deleted"`
	Cancelled bool   `json:"cancelled"`
	Message   string `json:"message,omitempty"`
}

// MaxBulkDelete is the most posts one DeletePosts call takes.
const MaxBulkDelete = 100

// DeletePostsRequest is the body of POST /v1/posts/bulk-delete.
type DeletePostsRequest struct {
	IDs []string `json:"ids"`
}

// DeletePostsResponse is the response of POST /v1/posts/bulk-delete: every
// id is in exactly one of Deleted and Failed. As with DeletePost, published
// posts are only removed from Postatron, never from the platforms.
type DeletePostsResponse struct {
	Deleted []string           `json:"deleted"`
	Failed  []DeletePostsError `json:"failed"`
	Message string             `json:"message,omitempty"`
}

// DeletePostsError says why one post was not deleted.
type DeletePostsError struct {
	ID    string `json:"id"`
	Error string `json:"error"`
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

// QueueSlot is a weekly posting time: Day 0 is Sunday to 6 Saturday, Time
// "HH:MM" in the queue's timezone.
type QueueSlot struct {
	Day  int    `json:"day"`
	Time string `json:"time"`
}

// Queue is a weekly posting schedule for one profile. Posts added to it go
// out in its next free slot.
type Queue struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	ProfileID   string      `json:"profile_id"`
	ProfileName string      `json:"profile_name"`
	Timezone    string      `json:"timezone"`
	Slots       []QueueSlot `json:"slots"`
	// Active is false while the queue is paused and takes no new posts.
	Active bool `json:"active"`
	// NextSlot is when a post added now would go out; nil when none would,
	// with NextSlotError saying why.
	NextSlot      *time.Time `json:"next_slot,omitempty"`
	NextSlotError string     `json:"next_slot_error,omitempty"`
	// Queued is how many of its posts are still to go out.
	Queued int `json:"queued"`
}

// QueueList is the response of GET /v1/queues.
type QueueList struct {
	Data []Queue `json:"data"`
}

// CreateQueueRequest is the body of POST /v1/queues.
type CreateQueueRequest struct {
	Name string `json:"name"`
	// Profile is the profile (name or id) the queue posts for; Default when
	// left out. A post added to the queue must go to accounts in it.
	Profile string `json:"profile,omitempty"`
	// Timezone is an IANA name such as Europe/London; slot times are wall
	// clock times there, across daylight saving.
	Timezone string      `json:"timezone"`
	Slots    []QueueSlot `json:"slots,omitempty"`
	// Paused creates the queue paused, taking no posts until resumed.
	Paused bool `json:"paused,omitempty"`
}

// UpdateQueueRequest is the body of PATCH /v1/queues/{id}. Anything left out
// keeps its value. Changing the slots or pausing the queue leaves posts it
// has already placed where they are.
type UpdateQueueRequest struct {
	Name     *string      `json:"name,omitempty"`
	Profile  *string      `json:"profile,omitempty"`
	Timezone *string      `json:"timezone,omitempty"`
	Slots    *[]QueueSlot `json:"slots,omitempty"`
	// Active false pauses the queue; true resumes it.
	Active *bool `json:"active,omitempty"`
}

// DeleteQueueResponse is the response of DELETE /v1/queues/{id}. Posts the
// queue had already placed stay scheduled.
type DeleteQueueResponse struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
	Message string `json:"message,omitempty"`
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

// Analytics ranges for AnalyticsQuery.Range.
const (
	Range7d   = "7d"
	Range30d  = "30d"
	Range90d  = "90d"
	Range365d = "365d"
)

// AnalyticsQuery holds the filters accepted by GET /v1/analytics.
type AnalyticsQuery struct {
	// Range is 7d, 30d, 90d or 365d (default 30d).
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
	// LastSyncAt is when the oldest numbers in the report were read from a
	// platform, and NextSyncAt when they will next be read (RFC 3339).
	// Each account is read at most once an hour; asking sooner returns the
	// same numbers.
	LastSyncAt string `json:"last_sync_at,omitempty"`
	NextSyncAt string `json:"next_sync_at,omitempty"`
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

// Event statuses, types and ranges for ListEventsQuery.
const (
	EventStatusOK      = "ok"
	EventStatusWarning = "warning"
	EventStatusError   = "error"

	EventTypePosts    = "posts"
	EventTypeAccounts = "accounts"
	EventTypeAPI      = "api"

	EventRange24h = "24h"
	EventRange7d  = "7d"
	EventRange30d = "30d"
	EventRange90d = "90d"
)

// ListEventsQuery holds the filters accepted by GET /v1/events. Status, Type
// and Platform take one value or several separated by commas.
type ListEventsQuery struct {
	// Range is 24h, 7d (the default), 30d or 90d, ending now. Events are
	// kept for 90 days.
	Range string
	// Status is ok, warning or error.
	Status string
	// Type is posts, accounts or api.
	Type string
	// Platform keeps events about an account on that platform, e.g. instagram.
	Platform string
	Limit    int
	Cursor   string
}

// Event is one thing that happened in the account, newest first in
// EventList: a post's outcome on every account it went to, an account
// connected, disconnected or needing a reconnect, or a call to the API or
// MCP that changed something or was refused. It is the dashboard's Logs page.
type Event struct {
	ID string    `json:"id"`
	At time.Time `json:"at"`
	// Kind is post.published, post.partial, post.failed, post.unpublished,
	// account.connected, account.disconnected, account.needs_attention or
	// api.request.
	Kind   string `json:"kind"`
	Status string `json:"status"`
	// Source is where a post or request came from: dashboard, api or mcp.
	// Empty for what Postatron did by itself.
	Source string `json:"source,omitempty"`
	// ActorID is the person behind it: the owner, or a team member.
	ActorID    string   `json:"actor_id,omitempty"`
	ProfileIDs []string `json:"profile_ids,omitempty"`
	PostID     string   `json:"post_id,omitempty"`
	// Excerpt is the start of the post's text.
	Excerpt  string         `json:"excerpt,omitempty"`
	Message  string         `json:"message,omitempty"`
	Accounts []EventAccount `json:"accounts,omitempty"`
	Request  *EventRequest  `json:"request,omitempty"`
	// Reason is why an account was disconnected: user, expired or revoked.
	Reason string `json:"reason,omitempty"`
	// AtRisk are the scheduled posts an account problem will stop going out,
	// soonest first and at most ten, and AtRiskCount how many there are.
	AtRisk      []EventPostRef `json:"at_risk,omitempty"`
	AtRiskCount int            `json:"at_risk_count,omitempty"`
}

// EventAccount is an account an event concerns. On a post's outcome it is
// one delivery, with its own status, message and link.
type EventAccount struct {
	ID       string `json:"id"`
	Platform string `json:"platform"`
	Username string `json:"username,omitempty"`
	Name     string `json:"name,omitempty"`
	Status   string `json:"status,omitempty"`
	Message  string `json:"message,omitempty"`
	URL      string `json:"url,omitempty"`
	// ErrorCode is a failed delivery's code, as on Delivery.
	ErrorCode string `json:"error_code,omitempty"`
}

// EventRequest is the API or MCP call behind an api.request event.
type EventRequest struct {
	Method     string `json:"method"`
	Path       string `json:"path"`
	StatusCode int    `json:"status_code"`
	// Code is the API's error code on a refusal.
	Code       string `json:"code,omitempty"`
	DurationMs int64  `json:"duration_ms"`
	// Tool is the MCP tool that made the call.
	Tool string `json:"tool,omitempty"`
	// KeyName names the API key or connected app that called.
	KeyName string `json:"key_name,omitempty"`
}

// EventPostRef is a scheduled post an account problem puts at risk.
type EventPostRef struct {
	ID      string    `json:"id"`
	At      time.Time `json:"at"`
	Excerpt string    `json:"excerpt,omitempty"`
}

// EventList is the response of GET /v1/events.
type EventList struct {
	Data       []Event `json:"data"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

// Operations is the public API surface. Client implements it over HTTP; the
// MCP server and CLI consume it.
type Operations interface {
	CreatePost(ctx context.Context, req CreatePostRequest) (*Post, error)
	ListPosts(ctx context.Context, query ListPostsQuery) (*PostList, error)
	GetPost(ctx context.Context, id string) (*Post, error)
	UpdatePost(ctx context.Context, id string, req UpdatePostRequest) (*Post, error)
	DeletePost(ctx context.Context, id string) (*DeletePostResponse, error)
	DeletePosts(ctx context.Context, req DeletePostsRequest) (*DeletePostsResponse, error)
	RetryPost(ctx context.Context, id string, req RetryPostRequest) (*Post, error)
	ListEvents(ctx context.Context, query ListEventsQuery) (*EventList, error)
	ListAccounts(ctx context.Context) (*AccountList, error)
	ConnectAccount(ctx context.Context, req ConnectAccountRequest) (*ConnectLink, error)
	ListProfiles(ctx context.Context) (*ProfileList, error)
	ListQueues(ctx context.Context) (*QueueList, error)
	CreateQueue(ctx context.Context, req CreateQueueRequest) (*Queue, error)
	UpdateQueue(ctx context.Context, id string, req UpdateQueueRequest) (*Queue, error)
	DeleteQueue(ctx context.Context, id string) (*DeleteQueueResponse, error)
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
