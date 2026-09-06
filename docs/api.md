# Postatron REST API v1

Base URL: `https://api.postatron.com`
Every endpoint is JSON over HTTPS and needs an API key.
The API exposes exactly six endpoints; the MCP server and the CLI mirror them one to one.

| Method | Path | What it does | Scope |
| --- | --- | --- | --- |
| `POST` | `/v1/posts` | Create a post; `scheduled_at` makes it scheduled | `posts:write` |
| `GET` | `/v1/posts` | List posts, filter by status, platform and date | `posts:read` |
| `GET` | `/v1/posts/{id}` | One post with per-platform delivery status | `posts:read` |
| `DELETE` | `/v1/posts/{id}` | Cancel if scheduled, no-op if published | `posts:write` |
| `GET` | `/v1/accounts` | Connected social accounts | `accounts:read` |
| `GET` | `/v1/usage` | Quota used and remaining, posts per platform | `usage:read` |

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

## Quota and metering

Usage is metered in **destination-posts**: one post delivered to one account.
A post sent to X, LinkedIn and Bluesky consumes three destination-posts.
Bulk scheduling of the same post at N times consumes N times as many.

Two counters are capped per calendar month (UTC):

- `destination_posts`: every delivery.
- `x_link_posts`: deliveries to X whose text contains a link.
  X bills those at a higher rate, so every plan has a separate allowance.

Link-containing deliveries to every platform are also counted (`link_posts.used`) for visibility, but only the X subset is capped.
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

Limits are per API key and per plan, enforced inside the API.
Writes (`POST`, `DELETE`) are counted per hour because they can spend money; reads (`GET`) per minute.

| Plan | Writes per hour | Reads per minute |
| --- | --- | --- |
| Starter | 60 | 60 |
| Growth | 120 | 120 |
| Scale | 100 | 300 |

Every response carries `X-RateLimit-Limit`, `X-RateLimit-Remaining` and `X-RateLimit-Reset` (unix seconds).
Over the limit you get `429 rate_limited` with a `Retry-After` header in seconds:

```http
HTTP/1.1 429 Too Many Requests
Retry-After: 1740
X-RateLimit-Limit: 60
X-RateLimit-Remaining: 0
X-RateLimit-Reset: 1757070000
```

Windows are fixed and aligned to the hour (writes) or the minute (reads).
The numbers above are the defaults for each plan. `GET /v1/usage` always reports the limits in force for your key, so read them from there rather than hardcoding these values.
The API Gateway stage throttle (250 requests per second, burst 125) remains as a global backstop shared by all callers.

## Errors

Non-2xx responses share one envelope:

```json
{ "error": { "code": "validation_error", "message": "content is required", "details": {} } }
```

| HTTP | `code` | When |
| --- | --- | --- |
| 401 | `unauthorized` | Missing, malformed, unknown or revoked key |
| 402 | `quota_exceeded` | Destination-post or X link post ceiling reached |
| 403 | `insufficient_scope` | Key lacks the scope |
| 403 | `subscription_required` | No active subscription |
| 404 | `not_found` | Unknown endpoint or post |
| 422 | `validation_error` | Bad input; `details` says which field |
| 429 | `rate_limited` | Per-key limit hit; honour `Retry-After` |
| 500 | `internal_error` | Something broke on our side |

## Endpoints

### POST /v1/posts

Creates a text post.
Target accounts come from either `account_ids` (ids from `GET /v1/accounts`) or `platforms` (every connected account on each platform), not both.

```json
{
  "content": "Launch day! https://postatron.com",
  "platforms": ["x", "linkedin"],
  "scheduled_at": "2026-09-10T09:00:00Z"
}
```

Rules:

- `content` is required; per-platform limits apply (X 280 characters, or 25,000 for X Premium accounts; Bluesky 300; Threads 500; LinkedIn 3,000; Facebook 63,206).
- Instagram, TikTok and YouTube need media and cannot be targeted from the API yet (`422`).
- `scheduled_at` is RFC 3339 and must be at least 5 minutes ahead; omit it to publish now.
- Media posts are not supported through the API in this version.

Response `201 Created`:

```json
{
  "id": "0AbCdEfGhIjKlMnOpQrS",
  "status": "scheduled",
  "content": "Launch day! https://postatron.com",
  "media_type": "text",
  "contains_link": true,
  "scheduled_at": "2026-09-10T09:00:00Z",
  "created_at": "2026-09-05T10:12:03Z",
  "deliveries": [
    { "account_id": "1234567890", "platform": "x", "username": "henry", "status": "pending" },
    { "account_id": "abc-def", "platform": "linkedin", "username": "henry-wallis", "status": "pending" }
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
    { "id": "1234567890", "platform": "x", "username": "henry", "display_name": "Henry", "status": "connected", "connected_at": "2026-08-30T18:02:11Z" }
  ]
}
```

`status` is `connected` or `reconnect_required` (token missing or expired without a refresh token).

### GET /v1/usage

```json
{
  "period": { "start": "2026-09-01T00:00:00Z", "end": "2026-10-01T00:00:00Z" },
  "plan": { "code": "GROWTH", "name": "Growth", "interval": "month", "status": "ACTIVE", "connected_accounts": 6, "max_connected_accounts": 15 },
  "destination_posts": { "used": 412, "limit": 1500, "remaining": 1088 },
  "link_posts": { "used": 96 },
  "x_link_posts": { "used": 11, "limit": 40, "remaining": 29 },
  "by_platform": { "x": 128, "linkedin": 104, "bluesky": 71, "threads": 58, "facebook": 51 },
  "overage": { "enabled": true, "destination_posts": 0, "x_link_posts": 0, "destination_posts_ceiling": 3000, "x_link_posts_ceiling": 80, "destination_post_unit_price_usd": 0.03, "x_link_post_unit_price_usd": 0.35 },
  "rate_limits": { "writes_per_hour": 120, "reads_per_minute": 120 }
}
```

`limit` is what the plan includes each month.
`overage.*_ceiling` is the hard stop: the cap plus the overage allowance on monthly plans, or the cap alone on yearly plans.
The dashboard renders this same document on the API page and the Account page.

## Plans

| Plan | Price | Destination-posts / month | X link posts / month | Accounts |
| --- | --- | --- | --- | --- |
| Starter | $15/mo or $150/yr | 300 | 10 | 5 |
| Growth | $39/mo or $390/yr | 1,500 | 40 | 15 |
| Scale | $99/mo or $990/yr | 4,000 | 80 | 50 |

Overage on monthly plans: $0.03 per extra destination-post, $0.35 per extra X link post, up to double the plan's caps.
The reasoning is in `COSTS.md`.
