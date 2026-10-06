# Postatron REST API v1

Base URL: `https://api.postatron.com`
Every endpoint is JSON over HTTPS and needs an API key.
The MCP server (remote at `https://api.postatron.com/mcp`, or local as `postatron-mcp`) and the CLI mirror these endpoints one to one.

| Method | Path | What it does | Scope |
| --- | --- | --- | --- |
| `POST` | `/v1/posts` | Create a post; `scheduled_at` makes it scheduled | `posts:write` |
| `GET` | `/v1/posts` | List posts, filter by status, platform, profile and date | `posts:read` |
| `GET` | `/v1/posts/{id}` | One post with per-platform delivery status | `posts:read` |
| `DELETE` | `/v1/posts/{id}` | Cancel if scheduled, no-op if published | `posts:write` |
| `GET` | `/v1/accounts` | Connected social accounts and their profiles | `accounts:read` |
| `GET` | `/v1/profiles` | Profiles and the accounts in each | `accounts:read` |
| `GET` | `/v1/usage` | Quota used and remaining, posts per platform | `usage:read` |
| `GET` | `/v1/analytics` | Engagement, reach, followers and best posts | `posts:read` |
| `POST` | `/v1/media/uploads` | A link a person uses to attach a file from their device | `posts:write` |
| `GET` | `/v1/media/uploads/{id}` | Whether that file has arrived | `posts:read` |

## Authentication

Create a key in the dashboard under **API & MCP** (`/dashboard/api`).
Keys look like `ptn_` followed by 40 characters and are shown once.
Only a SHA-256 hash is stored, so a lost key has to be revoked and replaced.

Send the key as a bearer token:

```bash
curl https://api.postatron.com/v1/usage \
  -H "Authorization: Bearer ptn_..."
```

`X-API-Key: ptn_...` is accepted as well.

Each key carries scopes chosen at creation time: `posts:read`, `posts:write`, `accounts:read`, `usage:read`.
A key without the scope an endpoint needs gets `403 insufficient_scope`.
Revoking a key takes effect immediately.
A user can hold up to 10 active keys.

API access is included on every paid plan.
Keys keep working through a cancellation-pending period and stop when the subscription ends (`403 subscription_required`).

The remote MCP endpoint also accepts OAuth access tokens (`pto_...`), which MCP clients obtain themselves through the authorisation code flow with PKCE.

## Profiles

A profile groups connected accounts, usually one per brand or client.
Every user has a Default profile, and paid plans can add more (Starter 2, Growth 5, Scale 20).
A profile holds at most one account per platform, so a second Instagram account lives in a second profile.

Wherever the API takes a `profile`, it accepts either the profile's `id` or its name, ignoring case.
An unknown profile is a `422 validation_error` whose `details.profiles` lists the ones that exist.

## Quota and metering

Usage is metered in **destination-posts**: one post delivered to one account.
A post sent to X, LinkedIn and Bluesky consumes three destination-posts.

`destination_posts` is capped per calendar month (UTC).
Deliveries whose text contains a link are counted as well (`link_posts`, and `x_link_posts` for X), for visibility only: they have no cap and no extra charge.
Links are detected in the post text: `http(s)://` URLs, `www.` hosts and bare domains with a common TLD.

Quota is consumed when the post is created, whether it publishes immediately or later.
Cancelling a scheduled post does not refund it.
The dashboard and the API share the same counters, and `GET /v1/usage` reports them.

Monthly plans can exceed the cap on metered overage, up to twice the cap.
Yearly plans stop at the cap.
When a request would breach the hard ceiling the API returns `402 quota_exceeded` and nothing is published:

```json
{
  "error": {
    "code": "quota_exceeded",
    "message": "destination-post limit reached: 300 of 300 used this month, 2 requested; resets 2026-10-01T00:00:00Z. Upgrade your plan or wait for the next period.",
    "details": {
      "resource": "destination_posts",
      "used": 300,
      "limit": 300,
      "requested": 2,
      "resets_at": "2026-10-01T00:00:00Z"
    }
  }
}
```

## Rate limits

Limits are per API key (per user and client for OAuth), counted in fixed one-minute windows.

| Class | Requests per minute |
| --- | --- |
| Writes (`POST`, `DELETE`) | 30 |
| Reads (`GET`) | 60 |
| Upload sessions (`POST /v1/media/uploads`) | 120 |

Every response carries `X-RateLimit-Limit`, `X-RateLimit-Remaining` and `X-RateLimit-Reset` (unix seconds).
Over the limit you get `429 rate_limited` with a `Retry-After` header in seconds:

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 17
X-RateLimit-Limit: 30
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1757070000
```

`GET /v1/usage` always reports the limits in force for your key, so read them from there rather than hardcoding these values.

## Errors

Non-2xx responses share one envelope:

```json
{ "error": { "code": "validation_error", "message": "content is required", "details": {} } }
```

| HTTP | `code` | When |
| --- | --- | --- |
| 400 | `validation_error` | A `media_urls` entry could not be fetched, is not public, is too large or is not an image or video |
| 401 | `unauthorized` | Missing, malformed, unknown or revoked key |
| 402 | `quota_exceeded` | Destination-post ceiling reached |
| 403 | `insufficient_scope` | Key lacks the scope |
| 403 | `subscription_required` | No active subscription |
| 404 | `not_found` | Unknown endpoint, post or upload |
| 422 | `validation_error` | Bad input; `details` says which field |
| 429 | `rate_limited` | Per-key limit hit; honour `Retry-After` |
| 500 | `internal_error` | Something broke on our side |

## Endpoints

### POST /v1/posts

Creates a post, now or at `scheduled_at`.

```json
{
  "content": "Launch day! https://postatron.com",
  "platforms": ["x", "instagram"],
  "profile": "Acme",
  "media_urls": ["https://example.com/launch.jpg"],
  "scheduled_at": "2026-09-10T09:00:00Z"
}
```

Choosing accounts:

- `account_ids` (ids from `GET /v1/accounts`) or `platforms`, not both.
- `platforms` picks the account on each platform in `profile`.
  Without `profile` it uses the account on each platform across all profiles, which is only allowed while each platform has one account.
  Once a platform has accounts in two profiles, the request is refused with `422` and `details.profiles` lists the candidates, so a post never goes to every brand by accident.
- With `account_ids`, `profile` is optional; when given, every account must belong to it.

Content and media:

- `content` is required; per-platform limits apply (X 280 characters, or 25,000 for X Premium accounts; Bluesky 300; Threads 500; LinkedIn 3,000; Facebook 63,206; Instagram 2,200; TikTok 150).
- `media_urls`: public `https` links to images or one video, fetched by the server.
  Addresses inside a private network are refused.
- `media_ids`: upload ids from `POST /v1/media/uploads`, for files on someone's own device.
- Up to four media items across both, and a post cannot mix images and video.
- Instagram and TikTok need an image or a video; YouTube needs a video.
- `scheduled_at` is RFC 3339 and must be at least 5 minutes ahead; omit it to publish now.

Response `201 Created`:

```json
{
  "id": "0AbCdEfGhIjKlMnOpQrS",
  "status": "scheduled",
  "content": "Launch day! https://postatron.com",
  "media_type": "image",
  "contains_link": true,
  "scheduled_at": "2026-09-10T09:00:00Z",
  "created_at": "2026-09-05T10:12:03Z",
  "deliveries": [
    { "account_id": "1234567890", "platform": "x", "username": "acmehq", "status": "pending" },
    { "account_id": "abc-def", "platform": "instagram", "username": "acme", "status": "pending" }
  ]
}
```

`status` is `pending` for immediate posts (the worker picks them up within seconds) or `scheduled`.

### GET /v1/posts

Query parameters, all optional:

| Parameter | Values |
| --- | --- |
| `status` | `scheduled`, `pending`, `processing`, `published`, `failed`, `partial` |
| `platform` | `x`, `linkedin`, `bluesky`, `threads`, `facebook`, `instagram`, `tiktok`, `youtube` |
| `profile` | A profile id or name: only posts to its accounts |
| `from`, `to` | RFC 3339; applies to `scheduled_at` for scheduled posts and `created_at` otherwise |
| `limit` | 1 to 100, default 25 |
| `cursor` | `next_cursor` from the previous page |

Posts come newest first.
Response:

```json
{ "data": [ { "id": "...", "status": "published", "deliveries": [ ... ] } ], "next_cursor": "eyJvIjoyNX0" }
```

### GET /v1/posts/{id}

Returns the post with a `deliveries` entry per account: `status` (`pending`, `processing`, `published`, `failed`), `url` when published and `error` when failed.
Unknown ids return `404`.

### DELETE /v1/posts/{id}

Cancels a scheduled post and removes it.
Posts that are pending, processing or already published are left alone and reported with `cancelled: false`:

```json
{ "id": "...", "status": "published", "cancelled": false, "message": "post has already been published; nothing to cancel" }
```

### GET /v1/accounts

```json
{
  "data": [
    {
      "id": "1234567890",
      "platform": "x",
      "username": "henry",
      "display_name": "Henry",
      "status": "connected",
      "profile_id": "default",
      "profile_name": "Default",
      "connected_at": "2026-08-30T18:02:11Z"
    }
  ]
}
```

`status` is `connected` or `reconnect_required` (token missing or expired without a refresh token).

### GET /v1/profiles

```json
{
  "data": [
    { "id": "default", "name": "Default", "is_default": true, "accounts": [ { "id": "1234567890", "platform": "x", "username": "henry", "...": "..." } ] },
    { "id": "8f2c1a9e", "name": "Acme", "is_default": false, "accounts": [] }
  ]
}
```

Default comes first, then the rest oldest first.
`accounts` has the same shape as `GET /v1/accounts`.

### GET /v1/usage

```json
{
  "period": { "start": "2026-09-01T00:00:00Z", "end": "2026-10-01T00:00:00Z" },
  "plan": { "code": "GROWTH", "name": "Growth", "interval": "month", "status": "ACTIVE", "connected_accounts": 6, "max_connected_accounts": 15 },
  "destination_posts": { "used": 412, "limit": 1500, "remaining": 1088 },
  "link_posts": { "used": 96 },
  "x_link_posts": { "used": 11, "limit": 0, "remaining": 0 },
  "by_platform": { "x": 128, "linkedin": 104, "bluesky": 71, "threads": 58, "facebook": 51 },
  "overage": { "enabled": true, "destination_posts": 0, "x_link_posts": 0, "destination_posts_ceiling": 3000, "x_link_posts_ceiling": 0, "destination_post_unit_price_usd": 0.03, "x_link_post_unit_price_usd": 0 },
  "rate_limits": { "writes_per_minute": 30, "reads_per_minute": 60, "uploads_per_minute": 120, "writes_per_hour": 1800 }
}
```

`limit` is what the plan includes each month; `x_link_posts` is counted but uncapped, so its `limit` is 0.
`overage.*_ceiling` is the hard stop: the cap plus the overage allowance on monthly plans, or the cap alone on yearly plans.
`rate_limits.writes_per_hour` is kept for older clients; use `writes_per_minute`.

### GET /v1/analytics

The numbers behind the dashboard's Analytics page.

| Parameter | Values |
| --- | --- |
| `range` | `7d`, `30d` (default) or `90d` |
| `platform` | Only this platform |
| `account_id` | Only this connected account |
| `profile` | Only accounts in this profile (id or name) |
| `source` | `all` (default) or `postatron` for posts made with Postatron only |

```json
{
  "range": { "key": "30d", "start": "2026-09-06T00:00:00Z", "end": "2026-10-06T00:00:00Z" },
  "summary": { "engagement_rate_percent": 3.4, "engagement_rate_basis": "reach", "engagements": 1290, "reach": 37900, "followers": 4210, "posts_this_period": 24 },
  "per_platform": [ { "platform": "instagram", "posts": 10, "likes": 820, "comments": 64, "shares": 31, "views": 15200 } ],
  "top_posts": [ { "platform": "instagram", "url": "https://instagram.com/p/...", "likes": 210, "clicks": null, "engagement": 260, "posted_with_postatron": true, "...": "..." } ],
  "accounts": [ { "id": "abc", "platform": "instagram", "username": "acme", "followers": 3900, "supported": true, "status": "ok" } ],
  "generated_at": "2026-10-06T08:00:00Z",
  "notes": ["X is not included: X bills per read, so Postatron does not fetch X analytics."]
}
```

A metric a platform does not report is `null`, never a negative number.
`top_posts` holds at most ten posts.
Numbers are cached for up to an hour per account.

### POST /v1/media/uploads

For an agent that cannot send a file itself: it gets a link, the person opens it while signed in to Postatron and picks the file.

```json
{ "purpose": "photo for Tuesday's post" }
```

Response `201 Created`:

```json
{ "id": "u_...", "status": "PENDING", "upload_url": "https://postatron.com/dashboard/upload?u=u_...", "expires_at": "2026-10-07T08:00:00Z" }
```

The link carries no credential; someone who intercepts it gets a sign-in page.

### GET /v1/media/uploads/{id}

`status` is `PENDING` until the file arrives and `READY` afterwards.
Pass a ready id to `POST /v1/posts` in `media_ids`.

## Plans

| Plan | Price | Destination-posts / month | Accounts | Profiles |
| --- | --- | --- | --- | --- |
| Starter | $15/mo or $150/yr | 300 | 5 | 2 |
| Growth | $39/mo or $390/yr | 1,500 | 15 | 5 |
| Scale | $99/mo or $990/yr | 4,000 | 50 | 20 |

Overage on monthly plans: $0.03 per extra destination-post, up to double the plan's cap.
