# Postatron REST API v1

Base URL: `https://api.postatron.com`
Every endpoint is JSON over HTTPS and needs an API key.
The MCP server (remote at `https://api.postatron.com/mcp`, or local as `postatron-mcp`) and the CLI mirror these endpoints one to one.

| Method | Path | What it does | Scope |
| --- | --- | --- | --- |
| `POST` | `/v1/posts` | Create a post; `scheduled_at` schedules it, `queue` puts it in a queue's next free slot | `posts:write` |
| `GET` | `/v1/posts` | List posts, filter by status, platform, profile and date | `posts:read` |
| `GET` | `/v1/posts/{id}` | One post with per-platform delivery status | `posts:read` |
| `PATCH` | `/v1/posts/{id}` | Change a draft or scheduled post, or publish it now | `posts:write` |
| `DELETE` | `/v1/posts/{id}` | Delete a post; a published one is only removed from Postatron | `posts:write` |
| `POST` | `/v1/posts/bulk-delete` | Delete up to 100 posts at once | `posts:write` |
| `GET` | `/v1/accounts` | Connected social accounts and their profiles | `accounts:read` |
| `POST` | `/v1/accounts/connect` | A link the person opens to connect an account | `accounts:read` |
| `GET` | `/v1/profiles` | Profiles and the accounts in each | `accounts:read` |
| `GET` | `/v1/queues` | Posting queues and the next free slot in each | `posts:read` |
| `POST` | `/v1/queues` | Create a queue | `posts:write` |
| `PATCH` | `/v1/queues/{id}` | Change, pause or resume a queue | `posts:write` |
| `DELETE` | `/v1/queues/{id}` | Delete a queue | `posts:write` |
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

### Team workspaces

A key or MCP connection made by a team member can work in the team owner's account instead of the member's own.
Create the key while the dashboard is in that workspace, or choose the workspace on the sign-in page when connecting an agent.
It then sees only the profiles the member was granted, uses the owner's plan and quota, and what it changes appears in the team's activity under the member's name.
Membership is checked on every request: once the member leaves or is removed, or the owner's plan no longer includes teams, the key gets `403 workspace_forbidden`.

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
Deliveries whose text contains a link are counted as well (`link_posts`).
X posts with a link (`x_link_posts`) also have a monthly cap, because X charges far more for them: 30 on Starter, 80 on Growth, 200 on Scale.
At the cap, only X posts with a link are refused (`402 quota_exceeded`, `details.resource` = `x_link_posts`); plain X posts and every other platform carry on.
Each tweet of an X thread counts as its own destination-post, and as an X link post if it has a link.
Links are detected in the post text: `http(s)://` URLs, `www.` hosts and bare domains with a common TLD.

Quota is consumed when the post is created, whether it publishes immediately or later.
Cancelling a scheduled post does not refund it.
The dashboard and the API share the same counters, and `GET /v1/usage` reports them.

Every plan has a fixed price, so the caps are hard stops on monthly and yearly plans alike; nothing is ever billed on top.
When a request would go past a cap the API returns `402 quota_exceeded` and nothing is published:

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
| 403 | `workspace_forbidden` | A team workspace key whose holder is no longer on the team |
| 404 | `not_found` | Unknown endpoint, post, queue or upload |
| 409 | `conflict` | The post started publishing, or a queue is paused or full |
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
- `queue` (a name or id from `GET /v1/queues`) schedules the post for that queue's next free slot instead; it cannot be combined with `scheduled_at`.
  Every account must be in the queue's profile, and without `profile`, `platforms` means that profile's accounts.
  A paused or full queue is refused with `409`; the response's `scheduled_at` is the slot it was given.

Platform options:

`x`, `instagram` and `tiktok` hold what only that platform has, the same settings as the dashboard composer, checked by the same rules.
Each needs an account on its platform among the post's accounts.

```json
{
  "content": "1/ Tea or coffee?",
  "platforms": ["x", "instagram"],
  "media_urls": ["https://example.com/reel.mp4"],
  "x": {
    "content": "1/ Tea or coffee? Vote below",
    "thread": [{ "content": "2/ Results on Friday", "media_urls": ["https://example.com/chart.png"] }],
    "reply_settings": "following"
  },
  "instagram": { "post_type": "reel", "first_comment": "#tea #coffee", "trial_reel": "manual" }
}
```

| Platform | Field | Meaning |
| --- | --- | --- |
| X | `content` | Text for X only, in place of `content` |
| X | `thread` | Replies under the first tweet, each `{content, media_urls, media_ids}`; each tweet counts as a destination-post |
| X | `community` | A Community id or link to post into; not with a thread |
| X | `share_with_followers` | Also show a Community post to followers |
| X | `reply_settings` | `following`, `mentioned_users`, `subscribers` or `verified`; left out, everyone can reply |
| X | `long_post` | Over 280 characters, up to 25,000; X accepts it from Premium accounts only |
| X | `poll` | `{options, duration_minutes}`: 2 to 4 options of up to 25 characters, 5 minutes to 7 days (default a day); no media with a poll |
| Instagram | `post_type` | `auto` (default), `feed`, `story`, `reel` or `carousel` |
| Instagram | `caption` | Text for Instagram only |
| Instagram | `first_comment` | Posted as the first comment; not on stories |
| Instagram | `collaborators` | Up to 3 usernames invited to co-author |
| Instagram | `user_tags` | `{username, x, y, media_index}` tags on images, `x` and `y` from 0 to 1 |
| Instagram | `ai_generated` | Ask Instagram to show its AI label |
| Instagram | `trial_reel` | `manual` or `performance`: share a reel with non-followers first |
| TikTok | `title` | Up to 90 characters |
| TikTok | `description` | A photo post's long description |
| TikTok | `privacy` | `public_to_everyone`, `mutual_follow_friends`, `follower_of_creator` or `self_only` |
| TikTok | `disable_comment`, `disable_duet`, `disable_stitch` | Turn those off |
| TikTok | `commercial_content`, `your_brand`, `branded_content` | Commercial content disclosure |
| TikTok | `music_usage_confirmed` | The person agrees to TikTok's Music Usage Confirmation |

TikTok asks for its settings as a set: once any is given, `title`, `privacy`, `disable_comment`, `commercial_content` and `music_usage_confirmed` are required, plus `disable_duet` and `disable_stitch` for a video, and `your_brand` or `branded_content` when `commercial_content` is true.
Leave `tiktok` out to post with TikTok's defaults.

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

Every post, here and from the other endpoints, also carries:

- `source`: where it was made, `dashboard`, `api` or `mcp`.
- `created_by` and `updated_by`: `{id, name}` of the person who made it and of whoever last changed it; in a team they can differ.
- `queue`: `{id, name}` of the queue that chose its time, when one did (`name` is empty once the queue is deleted).
- `x`, `instagram`, `tiktok`: the platform options it was saved with, when it has any.

### GET /v1/posts

Query parameters, all optional:

| Parameter | Values |
| --- | --- |
| `status` | `draft`, `scheduled`, `pending`, `processing`, `published`, `failed`, `partial` |
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

Deletes a post from Postatron, by the same rule as the dashboard.

- A draft, or a scheduled post more than two minutes from its time, is removed and never goes out: `deleted: true`, and for a scheduled post `cancelled: true` and `status: "cancelled"`.
- A published, failed or partly published post is removed from Postatron only.
  **Nothing is ever deleted on a platform**: the post stays live there, and `message` names where.
- A post being published right now, or due within two minutes, is left alone with `deleted: false` and the reason.

```json
{ "id": "...", "status": "published", "deleted": true, "cancelled": false, "message": "Removed from Postatron only. It is still live on X and LinkedIn; delete it there if it should go." }
```

### POST /v1/posts/bulk-delete

Deletes up to 100 posts, each by the rule above, eight at a time.

```json
{ "ids": ["0AbCd...", "1EfGh..."] }
```

Every id comes back in exactly one list, in the order asked:

```json
{ "deleted": ["0AbCd..."], "failed": [{ "id": "1EfGh...", "error": "Not deleted: it is about to be published." }], "message": "1 of the deleted posts had been published. They were removed from Postatron only and are still live on the platforms." }
```

### PATCH /v1/posts/{id}

Changes a draft or a scheduled post, and schedules or publishes a draft.
Every field is optional; what is left out keeps its value.

```json
{
  "content": "We're live",
  "scheduled_at": "2026-10-08T09:00:00+01:00",
  "add_platforms": ["instagram"],
  "add_account_ids": ["1234567890"],
  "remove_account_ids": ["9876543210"],
  "profile": "Acme",
  "publish_now": false
}
```

`add_platforms` picks the account on each platform the way `platforms` does on create, narrowed by `profile`.
Accounts added count against the monthly quota like a new post, and a new text with a link counts the X accounts already on it as X link posts; nothing is refunded when accounts are removed or the text changes.
The result is checked as a whole: Instagram and TikTok still need the post to have an image or a video, and at least one account must remain.
`x`, `instagram` and `tiktok` replace that platform's options as a whole (see `POST /v1/posts`); `{}` clears them.
Only `draft` and `scheduled` posts change: once a post has started publishing the API returns `409 conflict`.
A draft stays a draft until `scheduled_at` schedules it or `publish_now` publishes it; it needs at least one account by then.
`publish_now: true` publishes straight away after the other changes, for a draft or a scheduled post, and cannot be combined with `scheduled_at`; the post comes back `processing`.
A draft was counted against the quota when it was saved, so scheduling or publishing it costs nothing more unless the same request adds accounts.
Returns the updated post.

### POST /v1/accounts/connect

Returns a link that connects one of your social accounts.

```json
{ "platform": "x", "profile": "Acme" }
```

```json
{ "platform": "x", "connect_url": "https://postatron.com/dashboard/socials?connect=x&profile=...", "profile": "acme", "hint": "Open the link..." }
```

The link opens Postatron and starts that platform's sign-in; nothing is connected until the person finishes it there.
It only works for the person whose key asked for it: signed in as someone else, the page refuses it.
A team workspace key's link also opens the dashboard in that workspace.
A plan whose account allowance is used up gets `402 quota_exceeded` instead of a link.

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

### GET /v1/queues

A queue is a profile's weekly posting times, set up in the dashboard under Queues or with `POST /v1/queues`.
Pass a queue's `name` or `id` as `queue` to `POST /v1/posts` to give a post its next free slot.

```json
{
  "data": [
    {
      "id": "q7k2m9xw4a",
      "name": "Weekday mornings",
      "profile_id": "default",
      "profile_name": "Default",
      "timezone": "Europe/London",
      "slots": [{ "day": 1, "time": "09:00" }, { "day": 3, "time": "09:00" }],
      "active": true,
      "next_slot": "2026-10-12T08:00:00Z",
      "queued": 3
    }
  ]
}
```

`day` is 0 for Sunday to 6 for Saturday, and `time` is in the queue's `timezone`.
A slot already taken by another scheduled post for the same profile is skipped.
`next_slot` is left out when a post added now would not be placed, with `next_slot_error` saying why (the queue is paused, has no slots, or is full for the next year).

### POST /v1/queues

```json
{ "name": "Weekday mornings", "profile": "Acme", "timezone": "Europe/London", "slots": [{ "day": 1, "time": "09:00" }], "paused": false }
```

`name` and `timezone` (an IANA name) are required; `profile` defaults to Default.
Slots are sorted and duplicates dropped.
Returns `201` with the queue as `GET /v1/queues` shows it; an account holds at most 50 queues (`409`).

### PATCH /v1/queues/{id}

`{id}` is the queue's id or its name.
Any of `name`, `profile`, `timezone`, `slots` (replaces every slot) and `active` (`false` pauses, `true` resumes); what is left out keeps its value.
Posts the queue has already placed keep their times.

### DELETE /v1/queues/{id}

Deletes the queue; posts it already placed stay scheduled.

```json
{ "id": "q7k2m9xw4a", "deleted": true, "message": "Posts it had already placed stay scheduled at their times." }
```

### GET /v1/usage

```json
{
  "period": { "start": "2026-09-01T00:00:00Z", "end": "2026-10-01T00:00:00Z" },
  "plan": { "code": "GROWTH", "name": "Growth", "interval": "month", "status": "ACTIVE", "connected_accounts": 6, "max_connected_accounts": 15 },
  "destination_posts": { "used": 412, "limit": 1500, "remaining": 1088 },
  "link_posts": { "used": 96 },
  "x_link_posts": { "used": 11, "limit": 80, "remaining": 69 },
  "by_platform": { "x": 128, "linkedin": 104, "bluesky": 71, "threads": 58, "facebook": 51 },
  "rate_limits": { "writes_per_minute": 30, "reads_per_minute": 60, "uploads_per_minute": 120, "writes_per_hour": 1800 }
}
```

`limit` is what the plan includes each month, and the hard stop: the counters reset on the 1st (UTC).
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

Prices are fixed: nothing is billed beyond the plan, monthly or yearly.
