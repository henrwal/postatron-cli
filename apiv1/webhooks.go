package apiv1

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Webhook event types. A webhook subscribes to some or all of them; each one
// arrives as a WebhookEvent whose Data is the type's data below.
const (
	// WebhookPostPublished: a post went out on every account it was for.
	// Data is WebhookPostData.
	WebhookPostPublished = "post.published"
	// WebhookPostPartial: a post went out on some accounts and failed on
	// others. Data is WebhookPostData; each failed delivery says why.
	WebhookPostPartial = "post.partial"
	// WebhookPostFailed: a post failed on every account. Data is
	// WebhookPostData.
	WebhookPostFailed = "post.failed"
	// WebhookPostUnpublished: a scheduled post came due while the account had
	// no plan and went back to the drafts. Data is WebhookPostData.
	WebhookPostUnpublished = "post.unpublished"
	// WebhookAccountConnected: a social account was connected. Data is
	// WebhookAccountData.
	WebhookAccountConnected = "account.connected"
	// WebhookAccountDisconnected: a social account was removed, with Reason
	// saying how. Data is WebhookAccountData.
	WebhookAccountDisconnected = "account.disconnected"
	// WebhookAccountNeedsAttention: an account has to be connected again
	// before scheduled posts to it can go out. Data is WebhookAccountData,
	// with the posts at risk.
	WebhookAccountNeedsAttention = "account.needs_attention"
	// WebhookTest is what TestWebhook sends. Data is WebhookTestData.
	WebhookTest = "webhook.test"
	// WebhookAllEvents subscribes a webhook to every event, including ones
	// added later.
	WebhookAllEvents = "*"
)

// WebhookEventTypes lists the events a webhook can subscribe to.
var WebhookEventTypes = []string{
	WebhookPostPublished, WebhookPostPartial, WebhookPostFailed, WebhookPostUnpublished,
	WebhookAccountConnected, WebhookAccountDisconnected, WebhookAccountNeedsAttention,
}

// Headers on every webhook request.
const (
	// WebhookSignatureHeader is "t=<unix seconds>,v1=<hex HMAC-SHA256>",
	// the HMAC being of "<t>.<raw body>" keyed with the webhook's secret.
	// For a day after the secret is rotated it carries a second v1, made
	// with the old secret, so a receiver can switch without losing events.
	WebhookSignatureHeader = "Postatron-Signature"
	// WebhookEventHeader is the event's type, as in WebhookEvent.Type.
	WebhookEventHeader = "Postatron-Event"
	// WebhookDeliveryHeader is the delivery's id, as in WebhookEvent.ID. A
	// delivery that is sent again keeps its id, so use it to ignore repeats.
	WebhookDeliveryHeader = "Postatron-Delivery"
	// WebhookSecretPrefix starts every signing secret.
	WebhookSecretPrefix = "whsec_"
	// MaxWebhooks is how many webhooks an account can have.
	MaxWebhooks = 5
	// DefaultWebhookTolerance is how old a signature VerifyWebhook accepts.
	DefaultWebhookTolerance = 5 * time.Minute
)

// Error codes for idempotent requests: POST /v1/posts with an
// Idempotency-Key header (CreatePostRequest.IdempotencyKey).
const (
	// CodeIdempotencyKeyReused: the key was used in the last 24 hours with
	// a different request body. Use a new key for a new request.
	CodeIdempotencyKeyReused = "idempotency_key_reused"
	// CodeIdempotencyKeyInUse: a request with the key is still being
	// handled. Wait a moment and send it again.
	CodeIdempotencyKeyInUse = "idempotency_key_in_use"
)

// Webhook is an endpoint Postatron sends the account's events to, signed
// with its secret. The secret is only ever returned by CreateWebhook and
// RotateWebhookSecret.
type Webhook struct {
	ID  string `json:"id"`
	URL string `json:"url"`
	// Events are the event types it receives, or ["*"] for every one.
	Events  []string `json:"events"`
	Enabled bool     `json:"enabled"`
	// DisabledReason says why Postatron switched it off: it does so after
	// 20 deliveries in a row fail. Enable it again with UpdateWebhook.
	DisabledReason string    `json:"disabled_reason,omitempty"`
	CreatedAt      time.Time `json:"created_at"`
	// LastAttempt is the latest attempt to deliver to it, test or real.
	LastAttempt *WebhookAttempt `json:"last_attempt,omitempty"`
	// ConsecutiveFailures counts deliveries in a row that failed every
	// retry; one that succeeds resets it.
	ConsecutiveFailures int `json:"consecutive_failures"`
	// Secret signs every request. Set only in the response to CreateWebhook
	// and RotateWebhookSecret, and never shown again.
	Secret string `json:"secret,omitempty"`
}

// WebhookAttempt is one attempt to deliver an event.
type WebhookAttempt struct {
	At time.Time `json:"at"`
	// Event is the type of the event it carried.
	Event string `json:"event,omitempty"`
	// Succeeded is true when the endpoint answered with a 2xx status.
	Succeeded bool `json:"succeeded"`
	// StatusCode is the endpoint's answer; 0 when it never answered.
	StatusCode int `json:"status_code,omitempty"`
	// Error says why it failed.
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// WebhookList is the response of GET /v1/webhooks.
type WebhookList struct {
	Data []Webhook `json:"data"`
}

// CreateWebhookRequest is the body of POST /v1/webhooks. URL must be https,
// on a public address. Events left out means every event.
type CreateWebhookRequest struct {
	URL    string   `json:"url"`
	Events []string `json:"events,omitempty"`
}

// UpdateWebhookRequest is the body of PATCH /v1/webhooks/{id}. A field left
// out keeps its value. Enabling a webhook clears its failure count.
type UpdateWebhookRequest struct {
	URL     *string  `json:"url,omitempty"`
	Events  []string `json:"events,omitempty"`
	Enabled *bool    `json:"enabled,omitempty"`
}

// DeleteWebhookResponse is the response of DELETE /v1/webhooks/{id}.
type DeleteWebhookResponse struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// WebhookTestResult is the response of POST /v1/webhooks/{id}/test: a
// webhook.test event sent there and then, and what the endpoint answered.
type WebhookTestResult struct {
	DeliveryID string `json:"delivery_id"`
	Delivered  bool   `json:"delivered"`
	StatusCode int    `json:"status_code,omitempty"`
	Error      string `json:"error,omitempty"`
	DurationMs int64  `json:"duration_ms"`
}

// WebhookEvent is the JSON body of every webhook request.
type WebhookEvent struct {
	// ID is the delivery's id, the same as the Postatron-Delivery header.
	ID        string          `json:"id"`
	Type      string          `json:"type"`
	CreatedAt time.Time       `json:"created_at"`
	Data      json.RawMessage `json:"data"`
}

// WebhookPostData is the data of the post.* events: the post as GetPost
// describes it, with every delivery.
type WebhookPostData struct {
	Post Post `json:"post"`
}

// WebhookAccountData is the data of the account.* events: the account as
// ListAccounts describes it. Its status is disconnected on
// account.disconnected.
type WebhookAccountData struct {
	Account Account `json:"account"`
	// Reason is why an account was disconnected: user, expired or revoked.
	Reason string `json:"reason,omitempty"`
	// Message says what happened, when Postatron lost access or the account
	// needs attention.
	Message string `json:"message,omitempty"`
	// AtRisk are scheduled posts that will fail until the account is
	// connected again, soonest first and at most ten; AtRiskCount counts
	// them all.
	AtRisk      []EventPostRef `json:"at_risk,omitempty"`
	AtRiskCount int            `json:"at_risk_count,omitempty"`
}

// WebhookTestData is the data of a webhook.test event.
type WebhookTestData struct {
	WebhookID string `json:"webhook_id"`
	Message   string `json:"message"`
}

// Webhook verification errors.
var (
	ErrWebhookSignatureMissing = errors.New("the Postatron-Signature header is missing or malformed")
	ErrWebhookSignatureInvalid = errors.New("no signature matches the body and secret")
	ErrWebhookSignatureExpired = errors.New("the signature is older than the tolerance allows")
)

// SignWebhook is the v1 signature of body at time t: the hex HMAC-SHA256 of
// "<t>.<body>" keyed with the secret, whole, "whsec_" included.
func SignWebhook(secret string, t int64, body []byte) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(strconv.FormatInt(t, 10)))
	mac.Write([]byte("."))
	mac.Write(body)
	return hex.EncodeToString(mac.Sum(nil))
}

// VerifyWebhook checks a webhook request: that a v1 signature in header (the
// Postatron-Signature value) is the HMAC of the raw body with secret, and
// that it was made within tolerance of now (DefaultWebhookTolerance when
// zero), so a captured request cannot be replayed later. Pass the body
// exactly as received, before any JSON parsing.
func VerifyWebhook(secret, header string, body []byte, tolerance time.Duration, now time.Time) error {
	if tolerance <= 0 {
		tolerance = DefaultWebhookTolerance
	}
	var (
		t          int64
		signatures []string
	)
	for _, part := range strings.Split(header, ",") {
		name, value, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		switch name {
		case "t":
			parsed, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				return ErrWebhookSignatureMissing
			}
			t = parsed
		case "v1":
			signatures = append(signatures, value)
		}
	}
	if t == 0 || len(signatures) == 0 {
		return ErrWebhookSignatureMissing
	}
	expected := SignWebhook(secret, t, body)
	matched := false
	for _, sig := range signatures {
		if hmac.Equal([]byte(sig), []byte(expected)) {
			matched = true
		}
	}
	if !matched {
		return ErrWebhookSignatureInvalid
	}
	age := now.Sub(time.Unix(t, 0))
	if age > tolerance || age < -tolerance {
		return ErrWebhookSignatureExpired
	}
	return nil
}

// WebhookOperations manages the account's webhooks. Client implements it.
// They are not MCP tools, so they are kept out of Operations.
type WebhookOperations interface {
	ListWebhooks(ctx context.Context) (*WebhookList, error)
	CreateWebhook(ctx context.Context, req CreateWebhookRequest) (*Webhook, error)
	UpdateWebhook(ctx context.Context, id string, req UpdateWebhookRequest) (*Webhook, error)
	DeleteWebhook(ctx context.Context, id string) (*DeleteWebhookResponse, error)
	TestWebhook(ctx context.Context, id string) (*WebhookTestResult, error)
	RotateWebhookSecret(ctx context.Context, id string) (*Webhook, error)
}

var _ WebhookOperations = (*Client)(nil)

// ListWebhooks calls GET /v1/webhooks.
func (c *Client) ListWebhooks(ctx context.Context) (*WebhookList, error) {
	var out WebhookList
	if err := c.do(ctx, http.MethodGet, "/v1/webhooks", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateWebhook calls POST /v1/webhooks. The response carries the signing
// secret, which is not shown again.
func (c *Client) CreateWebhook(ctx context.Context, req CreateWebhookRequest) (*Webhook, error) {
	var out Webhook
	if err := c.do(ctx, http.MethodPost, "/v1/webhooks", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateWebhook calls PATCH /v1/webhooks/{id}.
func (c *Client) UpdateWebhook(ctx context.Context, id string, req UpdateWebhookRequest) (*Webhook, error) {
	var out Webhook
	if err := c.do(ctx, http.MethodPatch, "/v1/webhooks/"+url.PathEscape(id), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteWebhook calls DELETE /v1/webhooks/{id}.
func (c *Client) DeleteWebhook(ctx context.Context, id string) (*DeleteWebhookResponse, error) {
	var out DeleteWebhookResponse
	if err := c.do(ctx, http.MethodDelete, "/v1/webhooks/"+url.PathEscape(id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// TestWebhook calls POST /v1/webhooks/{id}/test.
func (c *Client) TestWebhook(ctx context.Context, id string) (*WebhookTestResult, error) {
	var out WebhookTestResult
	if err := c.do(ctx, http.MethodPost, "/v1/webhooks/"+url.PathEscape(id)+"/test", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// RotateWebhookSecret calls POST /v1/webhooks/{id}/rotate-secret. The
// response carries the new secret; the old one keeps signing alongside it
// for a day.
func (c *Client) RotateWebhookSecret(ctx context.Context, id string) (*Webhook, error) {
	var out Webhook
	if err := c.do(ctx, http.MethodPost, "/v1/webhooks/"+url.PathEscape(id)+"/rotate-secret", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
