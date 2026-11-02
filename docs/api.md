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
| `POST` | `/v1/posts/{id}/retry` | Send a failed post again to the accounts it failed on | `posts:write` |
| `GET` | `/v1/events` | The event log: post outcomes, account changes and API calls | `posts:read` |
| `GET` | `/v1/accounts` | Connected social accounts and their profiles | `accounts:read` |
| `POST` | `/v1/accounts/connect` | A link the person opens to connect an account, or sign one in again | `accounts:read` |
| `GET` | `/v1/profiles` | Profiles and the accounts in each | `accounts:read` |
| `GET` | `/v1/queues` | Posting queues and the next free slot in each | `posts:read` |
| `POST` | `/v1/queues` | Create a queue | `posts:write` |
| `PATCH` | `/v1/queues/{id}` | Change, pause or resume a queue | `posts:write` |
| `DELETE` | `/v1/queues/{id}` | Delete a queue | `posts:write` |
| `GET` | `/v1/usage` | Posts per platform, and the X caps used and remaining | `usage:read` |
| `GET` | `/v1/analytics` | Engagement, reach, followers and best posts | `posts:read` |
| `POST` | `/v1/media/uploads` | A link a person uses to attach a file from their device | `posts:write` |
| `GET` | `/v1/media/uploads/{id}` | Whether that file has arrived | `posts:read` |
| `GET` | `/v1/webhooks` | The account's webhooks and how each last delivery went | `posts:read` |
| `POST` | `/v1/webhooks` | Add a webhook; the answer carries its signing secret, once | `posts:write` and `posts:read` |
| `PATCH` | `/v1/webhooks/{id}` | Change a webhook's URL or events, or switch it off or on | `posts:write` |
| `DELETE` | `/v1/webhooks/{id}` | Delete a webhook | `posts:write` |
| `POST` | `/v1/webhooks/{id}/test` | Send a signed `webhook.test` event now | `posts:write` |
| `POST` | `/v1/webhooks/{id}/rotate-secret` | A new signing secret; the old one signs beside it for a day | `posts:write` |

## OpenAPI

The API is also described in OpenAPI 3.1, at `https://api.postatron.com/v1/openapi.json`.
That address needs no key and no plan.
The description covers every route above, with its parameters, bodies and errors, the rate limit and idempotency headers, and every webhook event with its headers.
Each operation's security requirement names the scope it needs, and `x-postatron-rate-limit` names its rate limit class.
Point a code generator or an agent at it for a typed client, or read it from this module as `apiv1.OpenAPI` (`apiv1/openapi.json`).

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
Keys keep working through a cancellation-pending period and stop when the subscription ends.
Without a plan every endpoint answers `403 subscription_required`, reads included, so nothing is posted, scheduled, sent or uploaded; the error's `details.plans_url` is where to choose one.
The same goes for the MCP server and the CLI, which use this API.

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

Posts are not capped on any plan.
They are counted in **destination-posts**, one post delivered to one account, so a post sent to X, LinkedIn and Bluesky is three destination-posts.
`destination_posts` reports them with a `null` limit, and deliveries whose text contains a link are counted as well (`link_posts`).

X is the one platform with a monthly cap, because X bills Postatron for every post and every other platform is free to post to.
`x_posts` counts posts to X per calendar month (UTC): 300 on Starter, 1,500 on Growth and 4,000 on Scale.
X posts with a link (`x_link_posts`) have a smaller cap inside that, because X charges far more for them: 30 on Starter, 80 on Growth, 200 on Scale.
Each tweet of an X thread counts as its own X post, and as an X link post if it has a link.
Links are detected in the post text: `http(s)://` URLs, `www.` hosts and bare domains with a common TLD.

At the X post cap, a post that goes to X is refused (`402 quota_exceeded`, `details.resource` = `x_posts`), and posts to every other platform carry on.
At the X link cap, only X posts with a link are refused (`details.resource` = `x_link_posts`); plain X posts and every other platform carry on.
A refused request publishes nothing, so take the X accounts off a post, or the link off its X text, to send the rest.

Posts are counted when they are created, whether they publish immediately or later.
Cancelling a scheduled post does not refund it, and retrying a failed one (`POST /v1/posts/{id}/retry`) does not count it again.
The dashboard and the API share the same counters, and `GET /v1/usage` reports them.

Every plan has a fixed price, so the X caps are hard stops on monthly and yearly plans alike; nothing is ever billed on top:

```json
{
  "error": {
    "code": "quota_exceeded",
    "message": "You have reached your monthly X post limit of 300. Posts to every other platform still go out, so take X off this post to send it now. The limit resets on 1 October, or upgrade for more.",
    "details": {
      "resource": "x_posts",
      "used": 300,
      "limit": 300,
      "requested": 2,
      "resets_at": "2026-10-01T00:00:00Z"
    }
  }
}
```

Unlimited is meant for people and agents posting their own content.
Automated spam, or using Postatron to break a platform's rules, can get an account suspended (see the [terms](https://postatron.com/terms)).

## Rate limits

Limits are per API key (per user and client for OAuth), counted in fixed one-minute windows.

| Class | Requests per minute |
| --- | --- |
| Writes (`POST`, `PATCH`, `DELETE`) | 30 |
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
| 402 | `quota_exceeded` | A plan cap reached: X posts or X posts with a link this month (`details.resource` `x_posts` or `x_link_posts`), connected accounts or queues |
| 403 | `insufficient_scope` | Key lacks the scope |
| 403 | `subscription_required` | No active plan; `details.plans_url` is where to choose one |
| 403 | `workspace_forbidden` | A team workspace key whose holder is no longer on the team, or one asking for something only the owner may do, such as managing webhooks |
| 404 | `not_found` | Unknown endpoint, post, queue, upload or webhook |
| 409 | `conflict` | The post started publishing, a queue is paused or full, or a retry found nothing that failed |
| 409 | `idempotency_key_in_use` | A request with the same `Idempotency-Key` is still being handled; send it again in a moment |
| 422 | `idempotency_key_reused` | The `Idempotency-Key` was used in the last 24 hours for a different request |
| 422 | `reconnect_required` | An account the post would go to cannot post until it is connected again; `details.account_ids` names them, and `POST /v1/accounts/connect` with `account_id` gives the link |
| 422 | `validation_error` | Bad input; `details` says which field |
| 429 | `rate_limited` | Per-key limit hit; honour `Retry-After` |
| 500 | `internal_error` | Something broke on our side |

## Idempotent requests

`POST /v1/posts` takes an `Idempotency-Key` header, so a request whose answer you never saw (a timeout, a dropped connection) can be sent again without making a second post.
The key is any string of 1 to 255 printable ASCII characters that you choose, unique to the post you mean to make; a UUID is a good choice.

```bash
curl https://api.postatron.com/v1/posts \
  -H "Authorization: Bearer ptn_..." \
  -H "Idempotency-Key: 3f6c1d2e-8a4b-4f0e-9c1d-7b2a5e6f8d90" \
  -H "Content-Type: application/json" \
  -d '{"content": "Launch day!", "platforms": ["x", "linkedin"]}'
```

- The first request with a key runs as usual, and if it succeeds its answer is kept for 24 hours.
- The same request with the same key in that time gets the same answer back, with the header `Idempotent-Replayed: true`, and makes no post and uses no quota.
  The body is compared as JSON, so the order of its fields and its spacing do not matter.
- The same key with a different request is refused with `422 idempotency_key_reused`.
- The same key while the first request is still running is refused with `409 idempotency_key_in_use` and `Retry-After`; send it again then.
  If that first request never finished, the key is free again two minutes after it started.
- A request that was refused (a validation error, a quota, a reconnect) is not kept, so the same key can be used again once it is put right.
- Keys are your own: a teammate using the same key in the same account does not collide with you.

Over MCP, `create_post` takes the same key as `idempotency_key`; the CLI takes it as `create-post --idempotency-key`.

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

- `content` is required; per-platform limits apply (X 280 characters, or 25,000 for X Premium accounts; Bluesky 300; Threads 500; LinkedIn 3,000; Facebook 63,206; Instagram 2,200; TikTok 2,200 for a video's caption and 4,000 for a photo post's description; Pinterest 800, as the Pin's description).
  Each platform is judged on the text it receives, so `x.content`, `instagram.caption`, `tiktok.title` on a video, `tiktok.description` on photos and `pinterest.description` are measured in place of `content`.
  TikTok counts in UTF-16, so an emoji counts as two.
- `media_urls`: public `https` links to images or one video, fetched by the server.
  Addresses inside a private network are refused.
- `media_ids`: upload ids from `POST /v1/media/uploads`, for files on someone's own device.
- Up to four media items across both, and a post cannot mix images and video.
- Instagram and TikTok need an image or a video; YouTube needs a video.
- Pinterest needs an image, two to four images for a carousel, or one video, and a board for every Pinterest account, named in `pinterest` or the account's default (see Pinterest boards below).
- An account that cannot post until it is connected again (`status: "reconnect_required"` in `GET /v1/accounts`) is refused with `422 reconnect_required`, as the dashboard refuses it, rather than accepted only to fail when the post goes out.
- `scheduled_at` is RFC 3339 and must be at least 5 minutes ahead; omit it to publish now.
- `queue` (a name or id from `GET /v1/queues`) schedules the post for that queue's next free slot instead; it cannot be combined with `scheduled_at`.
  Every account must be in the queue's profile, and without `profile`, `platforms` means that profile's accounts.
  A paused or full queue is refused with `409`; the response's `scheduled_at` is the slot it was given.

Platform options:

`x`, `instagram`, `tiktok`, `youtube` and `pinterest` hold what only that platform has, the same settings as the dashboard composer, checked by the same rules.
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
| X | `thread` | Replies under the first tweet, each `{content, media_urls, media_ids}`; each tweet counts as an X post |
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
| TikTok | `title` | A video's caption, up to 2,200 characters, in place of `content`; or a photo post's title, up to 90 |
| TikTok | `description` | A photo post's text, up to 4,000 characters, in place of `content` |
| TikTok | `draft` | Send it to the creator's TikTok inbox to finish in the app, instead of publishing |
| TikTok | `privacy` | `public_to_everyone`, `mutual_follow_friends`, `follower_of_creator` or `self_only` |
| TikTok | `disable_comment`, `disable_duet`, `disable_stitch` | Turn those off |
| TikTok | `commercial_content`, `your_brand`, `branded_content` | Commercial content disclosure |
| TikTok | `music_usage_confirmed` | The person accepts TikTok's declaration: "By posting, you agree to TikTok's Music Usage Confirmation", or with `branded_content` "...TikTok's Branded Content Policy and Music Usage Confirmation". Show it to them; never agree for them |
| TikTok | `auto_add_music` | Let TikTok add recommended music to a photo post |
| TikTok | `ai_generated` | Ask TikTok to label the post as AI-generated |
| YouTube | `title` | Up to 100 characters; left out, the first line of `content` |
| YouTube | `description` | Replaces `content` as the description, up to 5000 bytes |
| YouTube | `privacy` | `public` (default), `unlisted` or `private` |
| YouTube | `made_for_kids` | The uploader's declaration that the video is made for children; ask the person |
| YouTube | `tags` | Up to 500 characters in all, counting a comma between tags and quotes around a tag with a space |
| YouTube | `category_id` | A YouTube category id, default `22` (People & Blogs); see below |
| YouTube | `first_comment` | Not available yet: refused until Google verifies the permission it needs |
| YouTube | `thumbnail_url`, `thumbnail_id` | A JPEG or PNG link, or an upload id, for a custom thumbnail |
| Pinterest | `board` | The board to save the Pin to, by id or name, for every Pinterest account in the post; left out, each account's default board |
| Pinterest | `boards` | For a post to several Pinterest accounts: each account id (from `GET /v1/accounts`) to its board, by id or name; wins over `board` for the accounts it names |
| Pinterest | `title` | The Pin's title, up to 100 characters; left out, the first line of `content`, cut to 100 |
| Pinterest | `description` | Replaces `content` as the Pin's description, up to 800 characters |
| Pinterest | `link` | Where the Pin leads when someone taps it: a full `https` or `http` address, up to 2,048 characters |

A TikTok post is published directly once it has a `privacy`, and TikTok then asks for its settings as a set: `privacy`, `disable_comment`, `commercial_content` and `music_usage_confirmed` are required, plus `disable_duet` and `disable_stitch` for a video, and `your_brand` or `branded_content` when `commercial_content` is true.
With `draft: true`, or with `tiktok` left out, the post goes to the creator's TikTok inbox instead, and they finish it in the TikTok app, which asks for the rest itself.
A draft takes only `title` and `description` on photos; a video draft carries the video alone, so it takes neither, and the other settings are refused.
A delivery to the inbox has no `url`, since nothing is public until the creator posts it.

YouTube refuses `<` and `>` in a title or description, so they are refused here too, including in `content` when it is the description.
The categories a video can be given are `1` Film & Animation, `2` Autos & Vehicles, `10` Music, `15` Pets & Animals, `17` Sports, `19` Travel & Events, `20` Gaming, `22` People & Blogs, `23` Comedy, `24` Entertainment, `25` News & Politics, `26` Howto & Style, `27` Education, `28` Science & Technology and `29` Nonprofits & Activism.
YouTube first comments are switched off for now, so `youtube.first_comment` is refused; YouTube only lets verified channels set a custom thumbnail.
A thumbnail cannot fail the post: the video is up before it is tried, so a refusal shows as the delivery's `error` on a `published` delivery, with `warning: true`.
The same goes for an Instagram first comment.

Pinterest boards:

Every Pin is saved to one of the account's boards: `pinterest.board`, or `pinterest.boards` when the post goes to more than one Pinterest account.
Left out, the account's default board is used, if one was chosen for it on the dashboard's Accounts page; `GET /v1/accounts` shows its id as `default_board_id`.
A board can be given by its id or by its name, ignoring case, and is stored as its id.
An unknown board is refused with `422 validation_error`; the message names the account and lists its boards, and `details.boards` holds their names, so ask the person which one they mean.
A Pinterest account with no board named and no default is refused the same way, which is also how to see its boards.
Pinterest only works with business accounts, so a personal account has to be switched to a (free) business account in Pinterest before it can be connected.

```json
{
  "content": "Autumn soup in 20 minutes",
  "platforms": ["pinterest"],
  "media_urls": ["https://example.com/soup.jpg"],
  "pinterest": { "board": "Recipes", "title": "Autumn soup", "link": "https://example.com/soup" }
}
```

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
- `x`, `instagram`, `tiktok`, `youtube`, `pinterest`: the platform options it was saved with, when it has any (a YouTube thumbnail comes back as `thumbnail_url`; Pinterest boards come back as ids in `boards`, keyed by account id, a default board included, and as `board` too when there is one Pinterest account).
- `unpublished`: `{reason, due_at}` on a draft that came due and went back to the drafts instead of out.
  The only reason is `no_plan`: the account had no plan when the post was due.
  Nothing is lost; once there is a plan, schedule or publish it again with `PATCH /v1/posts/{id}`, which clears `unpublished`.

### Deliveries

Each post has a `deliveries` entry per account:

| Field | Meaning |
| --- | --- |
| `account_id`, `platform`, `username` | The account |
| `status` | `draft` or `scheduled` while the post is (an account is not attempted until the post goes out), then `pending`, `processing`, `published` or `failed` |
| `url` | The post on the platform, once published (none for a TikTok inbox draft) |
| `error` | Why it failed, in words for the person; on a `published` delivery, what did not go with it, such as an Instagram first comment or a YouTube thumbnail |
| `error_code` | On a `failed` delivery, what kind of failure, so an agent can act without reading `error` |
| `warning` | `true` on a `published` delivery that carries an `error`: the post is live, and the error is about something after it |

What to do about each `error_code`:

| `error_code` | Meaning | What to do |
| --- | --- | --- |
| `reconnect` | The account's access has gone | `POST /v1/accounts/connect` with its `account_id`, give the person the link, then retry |
| `rate_limited` | The platform asked Postatron to slow down, or a daily limit was reached | Retry later |
| `platform_unavailable` | The platform failed or timed out | Retry |
| `postatron` | Something went wrong on Postatron's side | Retry |
| `media_rejected` | The platform would not take the image or video | A retry sends the same file; make a new post with another |
| `validation` | The post breaks a platform rule, such as a duplicate or a caption too long | Make a new post with the change |
| `policy` | The platform's rules refused it, such as TikTok posting only to private accounts for an unaudited app, or there was no plan when a retry ran | `error` says what would change it |
| `unknown` | A refusal Postatron does not recognise | `error` has the platform's words |

### POST /v1/posts/{id}/retry

Sends a `failed` or `partial` post again, to the accounts it failed on only.
An account it already published to is never posted to twice.

```json
{ "account_ids": ["1784140000000"] }
```

The body is optional: without `account_ids`, every account that failed goes again.
The response is the post, now `processing`, with the retried deliveries `pending` (`202 Accepted`); poll `GET /v1/posts/{id}` for the outcome, which also appears in `GET /v1/events`.
A retry is not counted again, so it uses none of the X caps: the post was counted when it was made.

Refusals:

- `409 conflict` when the post is not `failed` or `partial`, nothing on it failed, or a named account published (`details.reason` is `not_retryable` or `not_failed`), or the post changed while the retry was being made (`details.reason` is `conflict`; reload it and try again).
- `422 validation_error` when a named account is not on the post.
- `422 reconnect_required` when an account to retry cannot post until it is connected again, or is no longer connected; `details.account_ids` names them.
  Nothing is sent while any of them would only fail again.

### GET /v1/posts

Query parameters, all optional:

| Parameter | Values |
| --- | --- |
| `status` | `draft`, `scheduled`, `pending`, `processing`, `published`, `failed`, `partial` |
| `platform` | `x`, `linkedin`, `bluesky`, `threads`, `facebook`, `instagram`, `tiktok`, `youtube`, `pinterest` |
| `profile` | A profile id or name: only posts to its accounts |
| `from`, `to` | RFC 3339; applies to `scheduled_at` for scheduled posts and `created_at` otherwise |
| `limit` | 1 to 100, default 25; more than 100 is treated as 100 |
| `cursor` | `next_cursor` from the previous page |

Posts come newest first.
Response:

```json
{ "data": [ { "id": "...", "status": "published", "deliveries": [ ... ] } ], "next_cursor": "eyJvIjoyNX0" }
```

### GET /v1/posts/{id}

Returns the post with a `deliveries` entry per account (see [Deliveries](#deliveries)).
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
Accounts added are counted like a new post, so an X account added counts against the X post cap, and a new text with a link counts the X accounts already on it as X link posts; nothing is refunded when accounts are removed or the text changes.
The result is checked as a whole: Instagram, TikTok and Pinterest still need the post to have an image or a video, every Pinterest account needs a board (its default board, if it has one, when none is named), and at least one account must remain.
Unless the post stays a draft, an account that cannot post until it is connected again is refused with `422 reconnect_required`.
`x`, `instagram`, `tiktok`, `youtube` and `pinterest` replace that platform's options as a whole (see `POST /v1/posts`); `{}` clears them.
A Pinterest account added without `pinterest` goes to its default board; one without a default needs `pinterest` in the same request, which then names a board for every Pinterest account the post will have.
Only `draft` and `scheduled` posts change: once a post has started publishing the API returns `409 conflict`.
A draft stays a draft until `scheduled_at` schedules it or `publish_now` publishes it; it needs at least one account by then.
`publish_now: true` publishes straight away after the other changes, for a draft or a scheduled post, and cannot be combined with `scheduled_at`; the post comes back `processing`.
A draft was counted when it was saved, so scheduling or publishing it counts nothing more unless the same request adds accounts.
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
A plan whose account allowance is used up gets `402 quota_exceeded` instead of a link, and a profile that already has an account on that platform gets `409 conflict`.
`platform` is one of `x` (or `twitter`), `instagram`, `facebook`, `linkedin`, `linkedin_page` (a LinkedIn company Page), `tiktok`, `youtube`, `threads` or `bluesky`, and `pinterest` once Postatron's Pinterest app is approved.
`linkedin_page` is accepted only while company Pages are switched on; the answer to an unsupported platform lists the ones that are.
A Pinterest account must be a Pinterest business account; a free one will do, and a personal account can be switched in Pinterest's settings.

To sign in again an account whose access has gone (`reconnect_required`, or a delivery with `error_code: "reconnect"`), pass its `account_id`; `platform` can then be left out.

```json
{ "account_id": "1784140000000" }
```

The link signs that account in again where it is, in its own profile, and does not count against the allowance.
Asking for a `platform` whose account in `profile` needs reconnecting does the same.

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

`status` is `connected` or `reconnect_required`: no sign-in stored, one the platform has refused, or one expired with nothing to renew it.
A Pinterest account with a default board also has `default_board_id`, the board its Pins go to when a post names none.
A post to a `reconnect_required` account is refused; `POST /v1/accounts/connect` with its `account_id` gives the link that fixes it.

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
Returns `201` with the queue as `GET /v1/queues` shows it.
Each plan has a number of queues, paused ones included: Starter 1, Growth 5, Scale unlimited (up to 100, a safety ceiling).
Past it, creating one returns `402 quota_exceeded` with `details.resource` = `queues`.
After a move to a smaller plan the oldest queues keep working; the rest stop taking posts (`402 quota_exceeded` on `create_post` with `queue`, and `next_slot_error` saying why), while posts they already placed still go out.

### PATCH /v1/queues/{id}

`{id}` is the queue's id or its name.
Any of `name`, `profile`, `timezone`, `slots` (replaces every slot) and `active` (`false` pauses, `true` resumes); what is left out keeps its value.
Posts the queue has already placed keep their times, unless the body has `"move_posts": true` and changes `slots` or `timezone`.
Then its upcoming posts take the new slots, soonest first, in the order they were going out, through the same check as a new post, so none lands on a slot another post already has.
A post given a time of its own (one not on the queue's old slots), or due within five minutes, keeps its time.
The response is the queue as `GET /v1/queues` shows it, plus `rescheduled` (`{moved, kept}`), or `reschedule_error` saying why the posts did not move (no slots left, or no plan); the change to the queue is saved either way.

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
  "destination_posts": { "used": 412, "limit": null, "remaining": null },
  "x_posts": { "used": 128, "limit": 1500, "remaining": 1372 },
  "link_posts": { "used": 96 },
  "x_link_posts": { "used": 11, "limit": 80, "remaining": 69 },
  "by_platform": { "x": 128, "linkedin": 104, "bluesky": 71, "threads": 58, "facebook": 51 },
  "rate_limits": { "writes_per_minute": 30, "reads_per_minute": 60, "uploads_per_minute": 120, "writes_per_hour": 1800 }
}
```

`limit` is what the plan includes each month, and the hard stop: the counters reset on the 1st (UTC).
`limit` and `remaining` are `null` for a counter the plan does not cap, which `destination_posts` always is: posts are unlimited, and only `x_posts` and `x_link_posts` have caps.
`rate_limits.writes_per_hour` is kept for older clients; use `writes_per_minute`.

### GET /v1/analytics

The numbers behind the dashboard's Analytics page.

| Parameter | Values |
| --- | --- |
| `range` | `7d`, `30d` (default), `90d` or `365d` |
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
  "last_sync_at": "2026-10-06T07:31:00Z",
  "next_sync_at": "2026-10-06T08:31:00Z",
  "notes": ["X is not included: X bills per read, so Postatron does not fetch X analytics."]
}
```

A metric a platform does not report is `null`, never a negative number.
`top_posts` holds at most ten posts.
YouTube's numbers are in `per_platform` as YouTube reports them, but YouTube's API policies keep them out of engagement totals, engagement rates and `top_posts`.
X and Pinterest are not included; their accounts come back with `status: "unsupported"` and the reason.
Each account is read from its platform at most once an hour: `last_sync_at` is when the oldest numbers were read and `next_sync_at` when they will be read again, so asking sooner returns the same report.

### GET /v1/events

The account's event log, newest first: the dashboard's Logs page.
Each post's outcome on every account it went to, accounts connected, disconnected or needing a reconnect, and calls to the API or MCP that changed something or were refused.
Events are kept for 90 days.

| Parameter | Values |
| --- | --- |
| `range` | `24h`, `7d` (default), `30d` or `90d`, ending now |
| `status` | `ok`, `warning` or `error`; several separated by commas |
| `type` | `posts`, `accounts` or `api`; several separated by commas |
| `platform` | Only events about an account on this platform; several may be given, separated by commas |
| `limit` | 1 to 100, default 25; more than 100 is treated as 100 |
| `cursor` | `next_cursor` from the previous page |

```json
{
  "data": [
    {
      "id": "0001792922400123456789ab12cd34",
      "at": "2026-10-22T09:00:00Z",
      "kind": "post.partial",
      "status": "error",
      "source": "mcp",
      "post_id": "0AbCdEfGhIjKlMnOpQrS",
      "excerpt": "Launch day",
      "message": "Failed on 1 of 2 accounts.",
      "accounts": [
        { "id": "1234567890", "platform": "x", "username": "acmehq", "status": "ok", "url": "https://x.com/acmehq/status/1" },
        { "id": "1784140000000", "platform": "instagram", "username": "acme", "status": "error", "message": "Instagram no longer accepts Postatron's access...", "error_code": "reconnect" }
      ]
    }
  ],
  "next_cursor": "0001792922300..."
}
```

`kind` is `post.published`, `post.partial`, `post.failed`, `post.unpublished` (came due without a plan), `account.connected`, `account.disconnected` (with `reason`: `user`, `expired` or `revoked`), `account.needs_attention` (with `at_risk` and `at_risk_count`, the scheduled posts it will fail) or `api.request` (with `request`: `method`, `path`, `status_code`, `code`, `duration_ms`, `tool`, `key_name`).
A failed account carries `error_code`, as a delivery does.
A team workspace key sees what the Logs page shows that member: events about their profiles, and what they did themselves.
It takes the `posts:read` scope.

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

## Webhooks

A webhook sends the account's events to your own server as they happen: Postatron POSTs a signed JSON body to its URL.
An account can have five, each receiving every event or only those it chooses.
Add them in the dashboard under **API & MCP** or with the endpoints below.

### Events

| `type` | When | `data` |
| --- | --- | --- |
| `post.published` | A post went out on every account it was for | `post` |
| `post.partial` | A post went out on some accounts and failed on others | `post` |
| `post.failed` | A post failed on every account | `post` |
| `post.unpublished` | A scheduled post came due while the account had no plan, and went back to the drafts | `post` |
| `account.connected` | A social account was connected | `account` |
| `account.disconnected` | A social account was removed; `reason` is `user`, `expired` or `revoked` | `account`, `reason`, `message`, `at_risk` |
| `account.needs_attention` | An account must be connected again before its scheduled posts can go out | `account`, `message`, `at_risk` |
| `webhook.test` | Sent by `POST /v1/webhooks/{id}/test` | `webhook_id`, `message` |

`post` is the post exactly as `GET /v1/posts/{id}` describes it, with every delivery and its `error_code`.
`account` is the account as `GET /v1/accounts` describes it; on `account.disconnected` its `status` is `disconnected`.
`at_risk` lists up to ten scheduled posts that will fail until the account is connected again, and `at_risk_count` counts them all.
No token or secret is ever in a payload.

```json
{
  "id": "whd_9f2c4e1b7a3d5e6f8091a2b3c4d5e6f7",
  "type": "post.partial",
  "created_at": "2026-11-01T09:00:00Z",
  "data": {
    "post": {
      "id": "0AbCdEfGhIjKlMnOpQrS",
      "status": "partial",
      "content": "Launch day!",
      "media_type": "text",
      "contains_link": false,
      "created_at": "2026-11-01T08:59:58Z",
      "source": "api",
      "deliveries": [
        { "account_id": "1234567890", "platform": "x", "username": "acmehq", "status": "published", "url": "https://x.com/acmehq/status/1" },
        { "account_id": "1784140000000", "platform": "instagram", "username": "acme", "status": "failed", "error": "Instagram needs an image or a video.", "error_code": "validation" }
      ]
    }
  }
}
```

Every request carries these headers:

| Header | Value |
| --- | --- |
| `Content-Type` | `application/json` |
| `User-Agent` | `Postatron-Webhooks/1` |
| `Postatron-Event` | The event's `type` |
| `Postatron-Delivery` | The delivery's id, the same as the body's `id` |
| `Postatron-Signature` | `t=<unix seconds>,v1=<signature>` |

### Checking the signature

Every request is signed with the webhook's secret, which starts `whsec_` and is shown once, when the webhook is made or its secret rotated.
The signature is the hex HMAC-SHA256 of `<t>.<raw body>`, keyed with the whole secret, `whsec_` included, where `t` is the header's timestamp.
Check it on the raw body exactly as it arrived, before parsing the JSON, and refuse a timestamp more than five minutes from now, so a captured request cannot be replayed later.
For a day after the secret is rotated the header carries two `v1` signatures, one made with each secret; accept the request if either matches.

Go, with this module:

```go
import "github.com/henrwal/postatron-cli/apiv1"

func handle(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err := apiv1.VerifyWebhook(os.Getenv("POSTATRON_WEBHOOK_SECRET"), r.Header.Get("Postatron-Signature"), body, 0, time.Now()); err != nil {
		http.Error(w, "bad signature", http.StatusBadRequest)
		return
	}
	var event apiv1.WebhookEvent
	_ = json.Unmarshal(body, &event)
	w.WriteHeader(http.StatusNoContent) // answer first, then do the work
}
```

Node:

```js
import crypto from 'node:crypto';

export function verifyPostatron(secret, header, rawBody, toleranceSeconds = 300) {
  let t = '';
  const signatures = [];
  for (const part of (header || '').split(',')) {
    const [key, value] = part.trim().split('=');
    if (key === 't') t = value;
    if (key === 'v1') signatures.push(value);
  }
  if (!/^\d+$/.test(t) || Math.abs(Date.now() / 1000 - Number(t)) > toleranceSeconds) return false;
  const expected = crypto.createHmac('sha256', secret).update(`${t}.`).update(rawBody).digest('hex');
  return signatures.some(s => s.length === expected.length && crypto.timingSafeEqual(Buffer.from(s), Buffer.from(expected)));
}

// Express: keep the raw body for the check.
app.post('/webhooks/postatron', express.raw({ type: 'application/json' }), (req, res) => {
  if (!verifyPostatron(process.env.POSTATRON_WEBHOOK_SECRET, req.get('Postatron-Signature'), req.body)) return res.sendStatus(400);
  const event = JSON.parse(req.body);
  res.sendStatus(204);
});
```

Python:

```python
import hashlib
import hmac
import time


def verify_postatron(secret: str, header: str, raw_body: bytes, tolerance: int = 300) -> bool:
    t, signatures = "", []
    for part in (header or "").split(","):
        key, _, value = part.strip().partition("=")
        if key == "t":
            t = value
        elif key == "v1":
            signatures.append(value)
    if not t.isdigit() or abs(time.time() - int(t)) > tolerance:
        return False
    expected = hmac.new(secret.encode(), t.encode() + b"." + raw_body, hashlib.sha256).hexdigest()
    return any(hmac.compare_digest(expected, s) for s in signatures)
```

### Delivery and retries

- Answer with any 2xx status within 10 seconds; anything else, including a redirect, counts as a failure.
  Do slow work after answering.
- Redirects are never followed, so give the final address.
- A failed delivery is tried six times in all: at once, then after about 1 minute, 10 minutes, 1 hour, 6 hours and 12 hours.
- Every attempt, and any repeat, carries the same delivery id, so use `Postatron-Delivery` to ignore one you have already handled.
  Deliveries are not guaranteed to arrive in order; use `created_at`.
- After 20 deliveries in a row fail every attempt, the webhook is switched off, with `disabled_reason` saying so; switch it back on with `PATCH` and `"enabled": true`, which also clears the count.
- A webhook that is switched off, deleted or no longer subscribed to an event is sent nothing more, including deliveries already waiting.
- Nothing is sent while the account has no plan.
- A payload is a few kilobytes; one over 256 KiB is not sent, and the webhook's `last_attempt` says so.

### Addresses

A webhook's URL must be `https`, on port 443 or another port given in the URL, with no user name, password or `#fragment`.
Its host must resolve only to public addresses: private, loopback, link-local, carrier-grade NAT, cloud metadata and other reserved ranges, IPv4 and IPv6, are refused.
This is checked when the webhook is saved and again before every request, on the address actually connected to.

### GET /v1/webhooks

```json
{
  "data": [
    {
      "id": "wh_5c0f3a9e2b7d4c1e8f6a0b9d",
      "url": "https://example.com/webhooks/postatron",
      "events": ["*"],
      "enabled": true,
      "created_at": "2026-11-01T08:00:00Z",
      "last_attempt": { "at": "2026-11-01T09:00:01Z", "event": "post.partial", "succeeded": true, "status_code": 200, "duration_ms": 182 },
      "consecutive_failures": 0
    }
  ]
}
```

`events` is `["*"]` for every event, including ones added later.
`last_attempt` is the latest attempt, a test or a real event, with `error` when it failed.
`consecutive_failures` counts deliveries in a row that failed every attempt.
The secret is never listed.

### POST /v1/webhooks

```json
{ "url": "https://example.com/webhooks/postatron", "events": ["post.failed", "post.partial", "account.needs_attention"] }
```

Leave `events` out for every event.
The answer, `201 Created`, is the webhook with its `secret`, which is not shown again.
Choosing what a webhook receives also needs the `posts:read` scope, since it will receive the account's posts.

### PATCH /v1/webhooks/{id}

Any of `url`, `events` and `enabled`; what is left out is kept.
A new `url` is checked like a first one, and changing `url` or `events` also needs `posts:read`.

### DELETE /v1/webhooks/{id}

Answers `{ "id": "wh_...", "deleted": true }`.

### POST /v1/webhooks/{id}/test

Sends a signed `webhook.test` event there and then, even to a webhook that is switched off, and answers with what the endpoint said:

```json
{ "delivery_id": "whd_...", "delivered": false, "status_code": 500, "error": "The endpoint answered 500 Internal Server Error.", "duration_ms": 87 }
```

### POST /v1/webhooks/{id}/rotate-secret

Answers with the webhook and its new `secret`, shown once.
The old secret keeps signing beside the new one for 24 hours, so you can switch your server over without refusing events.

Webhooks belong to the account's owner, because they carry every post in it, across every profile: a team workspace key is refused with `403 workspace_forbidden`.

## Plans

| Plan | Price | Posts | X posts / month | X posts with a link / month | Accounts | Profiles |
| --- | --- | --- | --- | --- | --- | --- |
| Starter | $15/mo or $150/yr | Unlimited | 300 | 30 | 5 | 2 |
| Growth | $39/mo or $390/yr | Unlimited | 1,500 | 80 | 15 | 5 |
| Scale | $99/mo or $990/yr | Unlimited | 4,000 | 200 | 50 | 20 |

Prices are fixed: nothing is billed beyond the plan, monthly or yearly.
