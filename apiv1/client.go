package apiv1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// DefaultBaseURL is the production API host.
const DefaultBaseURL = "https://api.postatron.com"

// Client calls the public API with an API key.
type Client struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	UserAgent  string
}

// NewClient returns a Client for baseURL (DefaultBaseURL when empty).
func NewClient(baseURL, apiKey string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIKey:     apiKey,
		HTTPClient: &http.Client{Timeout: 30 * time.Second},
		UserAgent:  "postatron-go",
	}
}

var _ Operations = (*Client)(nil)

// CreatePost calls POST /v1/posts.
func (c *Client) CreatePost(ctx context.Context, req CreatePostRequest) (*Post, error) {
	var out Post
	if err := c.do(ctx, http.MethodPost, "/v1/posts", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListPosts calls GET /v1/posts.
func (c *Client) ListPosts(ctx context.Context, query ListPostsQuery) (*PostList, error) {
	params := url.Values{}
	if query.Status != "" {
		params.Set("status", query.Status)
	}
	if query.Platform != "" {
		params.Set("platform", query.Platform)
	}
	if query.Profile != "" {
		params.Set("profile", query.Profile)
	}
	if query.From != nil {
		params.Set("from", query.From.UTC().Format(time.RFC3339))
	}
	if query.To != nil {
		params.Set("to", query.To.UTC().Format(time.RFC3339))
	}
	if query.Limit > 0 {
		params.Set("limit", strconv.Itoa(query.Limit))
	}
	if query.Cursor != "" {
		params.Set("cursor", query.Cursor)
	}

	var out PostList
	if err := c.do(ctx, http.MethodGet, "/v1/posts", params, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetPost calls GET /v1/posts/{id}.
func (c *Client) GetPost(ctx context.Context, id string) (*Post, error) {
	var out Post
	if err := c.do(ctx, http.MethodGet, "/v1/posts/"+url.PathEscape(id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdatePost calls PATCH /v1/posts/{id}.
func (c *Client) UpdatePost(ctx context.Context, id string, req UpdatePostRequest) (*Post, error) {
	var out Post
	if err := c.do(ctx, http.MethodPatch, "/v1/posts/"+url.PathEscape(id), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ConnectAccount calls POST /v1/accounts/connect.
func (c *Client) ConnectAccount(ctx context.Context, req ConnectAccountRequest) (*ConnectLink, error) {
	var out ConnectLink
	if err := c.do(ctx, http.MethodPost, "/v1/accounts/connect", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeletePost calls DELETE /v1/posts/{id}.
func (c *Client) DeletePost(ctx context.Context, id string) (*DeletePostResponse, error) {
	var out DeletePostResponse
	if err := c.do(ctx, http.MethodDelete, "/v1/posts/"+url.PathEscape(id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListAccounts calls GET /v1/accounts.
func (c *Client) ListAccounts(ctx context.Context) (*AccountList, error) {
	var out AccountList
	if err := c.do(ctx, http.MethodGet, "/v1/accounts", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListProfiles calls GET /v1/profiles.
func (c *Client) ListProfiles(ctx context.Context) (*ProfileList, error) {
	var out ProfileList
	if err := c.do(ctx, http.MethodGet, "/v1/profiles", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// ListQueues calls GET /v1/queues.
func (c *Client) ListQueues(ctx context.Context) (*QueueList, error) {
	var out QueueList
	if err := c.do(ctx, http.MethodGet, "/v1/queues", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetAnalytics calls GET /v1/analytics.
func (c *Client) GetAnalytics(ctx context.Context, query AnalyticsQuery) (*AnalyticsReport, error) {
	params := url.Values{}
	for key, value := range map[string]string{
		"range":      query.Range,
		"platform":   query.Platform,
		"account_id": query.AccountID,
		"profile":    query.Profile,
		"source":     query.Source,
	} {
		if value != "" {
			params.Set(key, value)
		}
	}
	var out AnalyticsReport
	if err := c.do(ctx, http.MethodGet, "/v1/analytics", params, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateUpload calls POST /v1/media/uploads.
func (c *Client) CreateUpload(ctx context.Context, req CreateUploadRequest) (*Upload, error) {
	var out Upload
	if err := c.do(ctx, http.MethodPost, "/v1/media/uploads", nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetUpload calls GET /v1/media/uploads/{id}.
func (c *Client) GetUpload(ctx context.Context, id string) (*Upload, error) {
	var out Upload
	if err := c.do(ctx, http.MethodGet, "/v1/media/uploads/"+url.PathEscape(id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetUsage calls GET /v1/usage.
func (c *Client) GetUsage(ctx context.Context) (*UsageReport, error) {
	var out UsageReport
	if err := c.do(ctx, http.MethodGet, "/v1/usage", nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

func (c *Client) do(ctx context.Context, method, path string, params url.Values, body, out any) error {
	if c.APIKey == "" {
		return fmt.Errorf("no API key configured: set POSTATRON_API_KEY or pass --api-key")
	}

	endpoint := c.BaseURL + path
	if len(params) > 0 {
		endpoint += "?" + params.Encode()
	}

	var reader io.Reader
	if body != nil {
		payload, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encoding request: %w", err)
		}
		reader = bytes.NewReader(payload)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint, reader)
	if err != nil {
		return fmt.Errorf("building request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.APIKey)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.UserAgent)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling %s %s: %w", method, path, err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode > 299 {
		return decodeError(resp, raw)
	}

	if out == nil || len(raw) == 0 {
		return nil
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("decoding response: %w", err)
	}
	return nil
}

func decodeError(resp *http.Response, raw []byte) error {
	apiErr := &APIError{Status: resp.StatusCode, Message: strings.TrimSpace(string(raw))}

	var envelope ErrorBody
	if err := json.Unmarshal(raw, &envelope); err == nil && envelope.Error.Code != "" {
		apiErr.Code = envelope.Error.Code
		apiErr.Message = envelope.Error.Message
		apiErr.Details = envelope.Error.Details
	}
	if apiErr.Message == "" {
		apiErr.Message = http.StatusText(resp.StatusCode)
	}

	if retry := resp.Header.Get("Retry-After"); retry != "" {
		if seconds, err := strconv.Atoi(retry); err == nil {
			apiErr.RetryAfter = time.Duration(seconds) * time.Second
		}
	}
	return apiErr
}
