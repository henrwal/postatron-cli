# Postatron REST API, for agents without the MCP tools or the CLI

Base URL `https://api.postatron.com`, JSON over HTTPS, `Authorization: Bearer $POSTATRON_API_KEY`.
Full reference: https://github.com/henrwal/postatron-cli/blob/main/docs/api.md

## Calls

```bash
auth=(-H "Authorization: Bearer $POSTATRON_API_KEY" -H "Content-Type: application/json")
api=https://api.postatron.com

curl -s "${auth[@]}" "$api/v1/profiles"                          # profiles and their accounts
curl -s "${auth[@]}" "$api/v1/accounts"                          # accounts, each with profile_id, profile_name and status
curl -s "${auth[@]}" "$api/v1/posts?status=scheduled&profile=Acme"
curl -s "${auth[@]}" "$api/v1/posts?status=draft"
curl -s "${auth[@]}" "$api/v1/posts/POST_ID"                     # deliveries: status, url, error, error_code, warning
curl -s "${auth[@]}" -X DELETE "$api/v1/posts/POST_ID"           # cancels a draft or scheduled post; a published one is only removed from Postatron
curl -s "${auth[@]}" -X POST "$api/v1/posts/bulk-delete" -d '{"ids": ["POST_ID", "POST_ID"]}'
curl -s "${auth[@]}" "$api/v1/usage"
curl -s "${auth[@]}" "$api/v1/analytics?range=30d&profile=Acme" # range 7d, 30d, 90d or 365d
curl -s "${auth[@]}" "$api/v1/events?range=24h&status=error"     # the Logs page: what failed and why

curl -s "${auth[@]}" -X POST "$api/v1/posts" -d '{
  "content": "Launch day!",
  "platforms": ["linkedin", "x"],
  "profile": "Acme",
  "media_urls": ["https://example.com/launch.jpg"],
  "scheduled_at": "2026-10-07T09:00:00+01:00"
}'

# Change a draft or scheduled post: add Instagram, move it, or send it now.
curl -s "${auth[@]}" -X PATCH "$api/v1/posts/POST_ID" -d '{"add_platforms": ["instagram"], "profile": "Acme"}'
curl -s "${auth[@]}" -X PATCH "$api/v1/posts/POST_ID" -d '{"scheduled_at": "2026-10-08T09:00:00+01:00"}'
curl -s "${auth[@]}" -X PATCH "$api/v1/posts/POST_ID" -d '{"publish_now": true}'

# Send a failed post again, to the accounts it failed on only (all of them, or those named).
curl -s "${auth[@]}" -X POST "$api/v1/posts/POST_ID/retry"
curl -s "${auth[@]}" -X POST "$api/v1/posts/POST_ID/retry" -d '{"account_ids": ["ACCOUNT_ID"]}'

# Queues: a profile's weekly slots.
curl -s "${auth[@]}" "$api/v1/queues"
curl -s "${auth[@]}" -X POST "$api/v1/queues" -d '{"name": "Weekday mornings", "timezone": "Europe/London", "slots": [{"day": 1, "time": "09:00"}]}'
curl -s "${auth[@]}" -X PATCH "$api/v1/queues/Weekday%20mornings" -d '{"active": false}'
curl -s "${auth[@]}" -X DELETE "$api/v1/queues/Weekday%20mornings"

# A link the person opens to connect an account, or to sign one in again.
curl -s "${auth[@]}" -X POST "$api/v1/accounts/connect" -d '{"platform": "instagram", "profile": "Acme"}'
curl -s "${auth[@]}" -X POST "$api/v1/accounts/connect" -d '{"account_id": "ACCOUNT_ID"}'

curl -s "${auth[@]}" -X POST "$api/v1/media/uploads" -d '{"purpose": "photo for the launch post"}'
curl -s "${auth[@]}" "$api/v1/media/uploads/UPLOAD_ID"            # PENDING, then READY
```

Build JSON bodies with `jq -n --arg content "$text" '{content: $content, ...}'` rather than by hand, so quotes and newlines in the post survive.

## `POST /v1/posts` fields

| Field | Notes |
| --- | --- |
| `content` | Required. |
| `platforms` | `x`, `linkedin`, `bluesky`, `threads`, `facebook`, `instagram`, `tiktok`, `youtube`. The account on each, in `profile`. |
| `account_ids` | Instead of `platforms`: ids from `/v1/accounts`. |
| `profile` | Profile name (any case) or id. Required with `platforms` when a platform has accounts in more than one profile. |
| `media_urls` | Public https links, up to four. |
| `media_ids` | Upload ids that are `READY`. |
| `scheduled_at` | RFC 3339 with an offset, at least 5 minutes ahead. Omit to publish now. |
| `queue` | A queue name or id from `/v1/queues`. The post takes its next free slot; not with `scheduled_at`. |
| `x` | X options: `content`, `thread` (`[{content, media_urls, media_ids}]`), `community`, `share_with_followers`, `reply_settings`, `long_post`, `poll` (`{options, duration_minutes}`). |
| `instagram` | `post_type`, `caption`, `first_comment`, `collaborators`, `user_tags`, `ai_generated`, `trial_reel`. |
| `tiktok` | `title`, `description`, `draft`, `privacy`, `disable_comment`, `disable_duet`, `disable_stitch`, `commercial_content`, `your_brand`, `branded_content`, `music_usage_confirmed`, `auto_add_music`, `ai_generated`. A direct post needs all the required ones; a `draft` (or no block) goes to the creator's inbox. |
| `youtube` | `title`, `description`, `privacy` (`public`, `unlisted`, `private`), `made_for_kids`, `tags`, `category_id`, `thumbnail_url` or `thumbnail_id`. `first_comment` is refused for now. |

## Reading a post

Each delivery has `status` (`draft` or `scheduled` while the post is, then `pending`, `processing`, `published` or `failed`).
A `failed` one has `error` and `error_code`: `reconnect` (connect the account again, then retry), `rate_limited` (retry later), `platform_unavailable` or `postatron` (retry), `media_rejected` or `validation` (a new post with the change), `policy` or `unknown` (read `error`).
A `published` one with `warning: true` went out; its `error` is about something after it, such as a first comment.
A draft with `unpublished: {reason: "no_plan", due_at}` came due without a plan and went back to the drafts.

## Errors

Every error has the shape `{"error": {"code": "...", "message": "...", "details": {...}}}`.

| HTTP | `code` |
| --- | --- |
| 400 | `validation_error` for a media URL that cannot be fetched |
| 401 | `unauthorized` |
| 402 | `quota_exceeded` |
| 403 | `insufficient_scope`, `subscription_required`, `workspace_forbidden` |
| 404 | `not_found` |
| 409 | `conflict`: the post started publishing, the queue is paused or full, or a retry found nothing that failed |
| 422 | `reconnect_required`: an account cannot post until it is signed in again (`details.account_ids`); `validation_error` otherwise |
| 429 | `rate_limited`, with a `Retry-After` header in seconds |

Rate limits per key: 30 writes, 60 reads and 120 upload links a minute.
