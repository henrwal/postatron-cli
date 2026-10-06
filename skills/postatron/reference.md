# Postatron REST API, for agents without the MCP tools or the CLI

Base URL `https://api.postatron.com`, JSON over HTTPS, `Authorization: Bearer $POSTATRON_API_KEY`.
Full reference: https://github.com/henrwal/postatron-cli/blob/main/docs/api.md

## Calls

```bash
auth=(-H "Authorization: Bearer $POSTATRON_API_KEY" -H "Content-Type: application/json")
api=https://api.postatron.com

curl -s "${auth[@]}" "$api/v1/profiles"                          # profiles and their accounts
curl -s "${auth[@]}" "$api/v1/accounts"                          # accounts, each with profile_id and profile_name
curl -s "${auth[@]}" "$api/v1/posts?status=scheduled&profile=Acme"
curl -s "${auth[@]}" "$api/v1/posts/POST_ID"
curl -s "${auth[@]}" -X DELETE "$api/v1/posts/POST_ID"           # cancels if still scheduled
curl -s "${auth[@]}" "$api/v1/usage"
curl -s "${auth[@]}" "$api/v1/analytics?range=30d&profile=Acme"

curl -s "${auth[@]}" -X POST "$api/v1/posts" -d '{
  "content": "Launch day!",
  "platforms": ["linkedin", "x"],
  "profile": "Acme",
  "media_urls": ["https://example.com/launch.jpg"],
  "scheduled_at": "2026-10-07T09:00:00+01:00"
}'

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

## Errors

Every error has the shape `{"error": {"code": "...", "message": "...", "details": {...}}}`.

| HTTP | `code` |
| --- | --- |
| 400 | `validation_error` for a media URL that cannot be fetched |
| 401 | `unauthorized` |
| 402 | `quota_exceeded` |
| 403 | `insufficient_scope`, `subscription_required` |
| 404 | `not_found` |
| 422 | `validation_error` |
| 429 | `rate_limited`, with a `Retry-After` header in seconds |

Rate limits per key: 30 writes, 60 reads and 120 upload links a minute.
