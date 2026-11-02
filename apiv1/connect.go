package apiv1

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// Hosted connect lets an app built on Postatron have its own customers
// connect their social accounts without a Postatron login. The app makes a
// profile per customer, creates a connect session for it, and sends the
// customer to the session's URL: a page on postatron.com that says who is
// asking, runs the platform's own sign-in, puts the account in the profile,
// and sends the customer back to RedirectURL.

// Connect session statuses.
const (
	// ConnectSessionPending: nobody has connected an account through the
	// link yet, and it has not expired.
	ConnectSessionPending = "pending"
	// ConnectSessionConnected: an account was connected through the link,
	// which then stops working. ConnectSession.Account is that account.
	ConnectSessionConnected = "connected"
	// ConnectSessionExpired: the link ran out unused.
	ConnectSessionExpired = "expired"
)

// What the person's browser is sent back with, in RedirectURL's query:
// status=connected with account_id, platform, profile and session, or
// status=error with error and session. The query can be typed by anyone, so
// confirm an account with GetConnectSession (or the account.connected
// webhook) before relying on it.
const (
	// ConnectRedirectConnected is status when an account was connected.
	ConnectRedirectConnected = "connected"
	// ConnectRedirectError is status when the person came back without one;
	// error then says why.
	ConnectRedirectError = "error"
	// ConnectRedirectDeclined is error when the person said no on the
	// platform's screen, or chose to go back without connecting.
	ConnectRedirectDeclined = "access_denied"
	// ConnectRedirectAccountLimit is error when the plan's connected accounts
	// are all in use.
	ConnectRedirectAccountLimit = "account_limit"
	// ConnectRedirectConflict is error when the profile already has an
	// account on that platform, or the account is already in another profile.
	ConnectRedirectConflict = "conflict"
	// ConnectRedirectFailed is error for anything else: the platform's
	// sign-in did not complete.
	ConnectRedirectFailed = "failed"
)

// ConnectSessionTTL is how long a connect link works by default, and the
// shortest and longest a session can ask for with ExpiresIn.
const (
	ConnectSessionTTL    = time.Hour
	ConnectSessionMinTTL = 5 * time.Minute
	ConnectSessionMaxTTL = 24 * time.Hour
)

const (
	profilesPath        = "/v1/profiles"
	connectSessionsPath = "/v1/connect/sessions"
)

// CreateProfileRequest is the body of POST /v1/profiles.
type CreateProfileRequest struct {
	// Name is up to 40 characters, unique in the account, ignoring case.
	Name string `json:"name"`
}

// UpdateProfileRequest is the body of PATCH /v1/profiles/{profile_id}.
type UpdateProfileRequest struct {
	// Name is the profile's new name.
	Name string `json:"name"`
}

// DeleteProfileResponse is the response of DELETE /v1/profiles/{profile_id}.
type DeleteProfileResponse struct {
	ID      string `json:"id"`
	Deleted bool   `json:"deleted"`
}

// CreateConnectSessionRequest is the body of POST /v1/connect/sessions.
type CreateConnectSessionRequest struct {
	// Profile is the profile (name or id) the account joins: typically one
	// made for this customer with CreateProfile. Required unless AccountID
	// is given.
	Profile string `json:"profile,omitempty"`
	// Platforms the person may choose from: x, instagram, facebook,
	// linkedin, linkedin_page, tiktok, youtube, threads, bluesky or
	// pinterest. Empty offers every platform the profile has no account on.
	Platforms []string `json:"platforms,omitempty"`
	// AccountID signs that connected account in again where it is, instead
	// of connecting a new one: for an account that says reconnect_required.
	AccountID string `json:"account_id,omitempty"`
	// RedirectURL is where the person's browser goes afterwards: https, or
	// http on localhost for development, with no fragment.
	RedirectURL string `json:"redirect_url"`
	// ExpiresIn is how many seconds the link works for, 300 to 86400.
	// Zero means an hour.
	ExpiresIn int `json:"expires_in,omitempty"`
}

// ConnectSession is a hosted connect link and what came of it.
type ConnectSession struct {
	ID string `json:"id"`
	// URL is the page to send the person to. It is only in the answer to
	// CreateConnectSession: Postatron keeps a hash of it, not the link.
	URL string `json:"url,omitempty"`
	// Status is pending, connected or expired.
	Status      string   `json:"status"`
	ProfileID   string   `json:"profile_id"`
	ProfileName string   `json:"profile_name"`
	Platforms   []string `json:"platforms"`
	// AccountID is the account the session signs in again, when it was made
	// for one.
	AccountID   string    `json:"account_id,omitempty"`
	RedirectURL string    `json:"redirect_url"`
	CreatedAt   time.Time `json:"created_at"`
	// ExpiresAt is when the link stops working if nobody has used it.
	ExpiresAt   time.Time  `json:"expires_at"`
	ConnectedAt *time.Time `json:"connected_at,omitempty"`
	// Account is the account connected through the link, once Status is
	// connected and while it is still connected.
	Account *Account `json:"account,omitempty"`
}

// CreateProfile calls POST /v1/profiles.
func (c *Client) CreateProfile(ctx context.Context, req CreateProfileRequest) (*Profile, error) {
	var out Profile
	if err := c.do(ctx, http.MethodPost, profilesPath, nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// UpdateProfile calls PATCH /v1/profiles/{id}; id may be the profile's name.
func (c *Client) UpdateProfile(ctx context.Context, id string, req UpdateProfileRequest) (*Profile, error) {
	var out Profile
	if err := c.do(ctx, http.MethodPatch, profilesPath+"/"+url.PathEscape(id), nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// DeleteProfile calls DELETE /v1/profiles/{id}; id may be the profile's name.
func (c *Client) DeleteProfile(ctx context.Context, id string) (*DeleteProfileResponse, error) {
	var out DeleteProfileResponse
	if err := c.do(ctx, http.MethodDelete, profilesPath+"/"+url.PathEscape(id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// CreateConnectSession calls POST /v1/connect/sessions.
func (c *Client) CreateConnectSession(ctx context.Context, req CreateConnectSessionRequest) (*ConnectSession, error) {
	var out ConnectSession
	if err := c.do(ctx, http.MethodPost, connectSessionsPath, nil, req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// GetConnectSession calls GET /v1/connect/sessions/{id}.
func (c *Client) GetConnectSession(ctx context.Context, id string) (*ConnectSession, error) {
	var out ConnectSession
	if err := c.do(ctx, http.MethodGet, connectSessionsPath+"/"+url.PathEscape(id), nil, nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}
