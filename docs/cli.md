# Postatron CLI

`postatron` is a single binary with one command per API endpoint.

```bash
go install github.com/henrwal/postatron-cli/cmd/postatron@latest
```

Or from a checkout: `make build` produces `bin/postatron` and `bin/postatron-mcp`.

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
| `postatron delete-posts <id>...` | `POST /v1/posts/bulk-delete` |
| `postatron list-accounts` | `GET /v1/accounts` |
| `postatron list-profiles` | `GET /v1/profiles` |
| `postatron list-queues` | `GET /v1/queues` |
| `postatron create-queue` | `POST /v1/queues` |
| `postatron update-queue <queue>` | `PATCH /v1/queues/{id}` |
| `postatron delete-queue <queue>` | `DELETE /v1/queues/{id}` |
| `postatron get-usage` | `GET /v1/usage` |
| `postatron get-analytics` | `GET /v1/analytics` |
| `postatron create-upload` | `POST /v1/media/uploads` |
| `postatron get-upload <id>` | `GET /v1/media/uploads/{id}` |

Add `--json` to any command to print the raw API response, which is the recommended mode for scripts.

### create-post

```bash
postatron create-post --content "Launch day!" --platforms x,linkedin
postatron create-post --content "Tomorrow 9am" --platforms bluesky --scheduled-at 2026-09-10T09:00:00Z
postatron create-post --content "Only this account" --account-ids 1234567890
postatron create-post --content "New in store" --platforms x,instagram --profile Acme --media-urls https://example.com/shoe.jpg
```

| Flag | Meaning |
| --- | --- |
| `--content` | Required. The post text. |
| `--platforms` | Platforms to post to; the account on each in `--profile`. |
| `--account-ids` | Specific accounts from `list-accounts`, instead of `--platforms`. |
| `--profile` | Profile name or id. Needed with `--platforms` once a platform has accounts in more than one profile. |
| `--media-urls` | Public `https` links to images or one video, up to four. Instagram and TikTok need one; YouTube needs a video. |
| `--media-ids` | Upload ids from the API's upload endpoint. |
| `--scheduled-at` | RFC 3339, at least 5 minutes ahead. Omit to publish now. |
| `--queue` | A queue name or id: the post takes its next free slot. |
| `--thread` | An X reply under the first tweet; repeat for each. |
| `--poll` | Make the X post a poll with these 2 to 4 options; `--poll-minutes` for how long. |
| `--reply-settings` | Who can reply on X: `following`, `mentioned_users`, `subscribers` or `verified`. |
| `--community` | An X Community id or link. |
| `--instagram-type` | `auto`, `feed`, `story`, `reel` or `carousel`. |
| `--first-comment` | Instagram first comment. |
| `--options` | Every platform option as JSON, or `@file.json`; see the API reference. The flags above win over it. |

```bash
postatron create-post --content "Tea or coffee?" --platforms x --poll "Tea,Coffee" --reply-settings following
postatron create-post --content "1/ A thread" --platforms x --thread "2/ More" --thread "3/ The end"
postatron create-post --content "New reel" --platforms instagram --media-urls https://example.com/r.mp4 \
  --options '{"instagram":{"post_type":"reel","trial_reel":"manual"}}'
```

`update-post` takes the same platform flags; each replaces that platform's options as a whole.

### list-posts

```bash
postatron list-posts --status scheduled
postatron list-posts --profile Acme --platform instagram
postatron list-posts --platform x --from 2026-09-01T00:00:00Z --to 2026-09-30T23:59:59Z --limit 50
postatron list-posts --cursor eyJvIjo1MH0
```

### get-post, delete-post, delete-posts

```bash
postatron get-post 0AbCdEfGhIjKlMnOpQrS
postatron delete-post 0AbCdEfGhIjKlMnOpQrS
postatron delete-posts 0AbCdEfGhIjKlMnOpQrS 1EfGhIjKlMnOpQrStUvW
```

`get-post` shows who created and last changed the post, and the queue it came from.
`delete-post` cancels a draft or scheduled post.
A published post is only removed from Postatron and stays live on the platforms; the command says where.
A post that is publishing, or due within two minutes, prints `Not deleted` with the reason and exits 0.
`delete-posts` does the same for up to 100 posts and lists any it could not delete.

### list-accounts, list-profiles

```bash
postatron list-accounts
postatron list-accounts --profile Acme
postatron list-profiles
```

```text
ID        NAME     ACCOUNTS
default   Default  x:henry, linkedin:henry-wallis
8f2c1a9e  Acme     x:acmehq, instagram:acme
```

### list-queues

A queue is a profile's weekly posting times.
`create-post --queue <name or id>` puts a post in the queue's next free slot instead of at `--scheduled-at`; its accounts must be in the queue's profile.
Set them up in the dashboard under Queues, or here.

```bash
postatron list-queues
postatron create-queue --name "Weekday mornings" --timezone Europe/London --slots "mon 09:00,tue 09:00,wed 09:00"
postatron update-queue "Weekday mornings" --slots "mon 08:30,fri 08:30"
postatron update-queue "Weekday mornings" --pause
postatron delete-queue "Weekday mornings"
postatron create-post --content "Whenever there's room" --platforms x,linkedin --queue "Weekday mornings"
```

```text
ID          NAME              PROFILE  SLOTS      NEXT SLOT             QUEUED
q7k2m9xw4a  Weekday mornings  Default  5 a week   2026-10-12T08:00:00Z  3
```

### get-usage

```bash
postatron get-usage
```

```text
Plan: Starter (month, active)  Period: 2026-09-01 to 2026-09-30
Destination-posts: 12 / 300 (288 remaining)
X link posts:      1 (no cap)
Link posts (all):  4
Accounts:          2 / 5
Rate limits:       30 writes, 60 reads, 120 uploads per minute

Posts per platform:
  linkedin  5
  x         7
```

### get-analytics

```bash
postatron get-analytics
postatron get-analytics --range 7d --profile Acme --platform instagram
postatron get-analytics --source postatron
```

Flags: `--range` (`7d`, `30d` or `90d`), `--platform`, `--account-id`, `--profile`, `--source` (`all` or `postatron`).
X is not included, because X bills per read.

### create-upload, get-upload

```bash
postatron create-upload --purpose "photo for Tuesday's post"
postatron get-upload u_...
```

For a file that is not on the public internet.
`create-upload` prints a link; the person opens it while signed in to Postatron and picks the file.
Once `get-upload` reports `READY`, pass the id to `create-post --media-ids`.

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

Writes are limited to 30 a minute per key, so batch runs larger than that should sleep on exit code 4 and honour the `Retry-After` shown in the error.
