# Postatron CLI

`postatron` is a single binary with six commands, one per API endpoint.

```bash
go install github.com/henrwal/postatron-cli/cmd/postatron@latest
```

Or from a checkout: `cd backend && make cli` builds `bin/postatron` and `bin/postatron-mcp`.

## Authentication

Set the key once:

```bash
export POSTATRON_API_KEY=ptn_YOUR_KEY
```

or pass `--api-key ptn_...` on any command.
Create keys at `https://postatron.com/dashboard/api`.
`--api-url` (or `POSTATRON_API_URL`) points the CLI at another stack.

## Commands

| Command | Endpoint |
| --- | --- |
| `postatron create-post` | `POST /v1/posts` |
| `postatron list-posts` | `GET /v1/posts` |
| `postatron get-post <id>` | `GET /v1/posts/{id}` |
| `postatron delete-post <id>` | `DELETE /v1/posts/{id}` |
| `postatron list-accounts` | `GET /v1/accounts` |
| `postatron get-usage` | `GET /v1/usage` |

Add `--json` to any command to print the raw API response, which is the recommended mode for scripts.

### create-post

```bash
postatron create-post --content "Launch day!" --platforms x,linkedin
postatron create-post --content "Tomorrow 9am" --platforms bluesky --scheduled-at 2026-09-10T09:00:00Z
postatron create-post --content "Only this account" --account-ids 1234567890
```

Flags: `--content` (required), `--platforms` or `--account-ids` (one of them), `--scheduled-at` (RFC 3339, at least 5 minutes ahead).

### list-posts

```bash
postatron list-posts --status scheduled
postatron list-posts --platform x --from 2026-09-01T00:00:00Z --to 2026-09-30T23:59:59Z --limit 50
postatron list-posts --cursor eyJvIjo1MH0
```

### get-post, delete-post

```bash
postatron get-post 0AbCdEfGhIjKlMnOpQrS
postatron delete-post 0AbCdEfGhIjKlMnOpQrS
```

`delete-post` cancels a scheduled post; for anything already published it prints `Not cancelled` and exits 0.

### list-accounts, get-usage

```bash
postatron list-accounts
postatron get-usage
```

`get-usage` output:

```text
Plan: Starter (month, active)  Period: 2026-09-01 to 2026-09-30
Destination-posts: 12 / 300 (288 remaining)
X link posts:      1 / 10 (9 remaining)
Link posts (all):  4
Accounts:          2 / 5
Rate limits:       60 writes/hour, 60 reads/minute

Posts per platform:
  linkedin  5
  x         7
```

## Exit codes

| Code | Meaning |
| --- | --- |
| 0 | Success |
| 1 | API or network error |
| 2 | Usage error (bad flags or timestamps) |
| 3 | Authentication: missing key, invalid key, wrong scope, no subscription |
| 4 | Quota exceeded or rate limited (the message includes when to retry) |

Errors are printed to stderr as `postatron: <code>: <message>`.

## Scripting example

Schedule a week of posts from a CSV with `content,when` columns:

```bash
tail -n +2 posts.csv | while IFS=, read -r content when; do
  postatron create-post --json --content "$content" --platforms x,linkedin --scheduled-at "$when" \
    | jq -r '"\(.id) \(.status)"'
done
```

Rate limits apply per key (60 writes per hour on Starter), so batch runs larger than that should sleep on exit code 4 and honour the `Retry-After` shown in the error.
