---
name: postatron
description: Schedule, publish and manage social media posts through Postatron on X, LinkedIn, Instagram, Facebook, Threads, Bluesky, TikTok and YouTube, and read how they performed. Use when someone asks to post or schedule something to social media ("schedule this to LinkedIn and X tomorrow at 9am"), to see or cancel scheduled posts, to check whether a post went out, or to look at engagement, reach or plan usage.
---

# Postatron

Postatron publishes one post to many social accounts, now or at a set time.
Accounts are grouped into **profiles**, usually one per brand or client, with at most one account per platform in each.

## Pick a way in

Use the first of these that works, and stay with it for the whole task.

1. **MCP tools.** If tools such as `list_profiles` and `create_post` are available (from `https://api.postatron.com/mcp` or the local `postatron-mcp`), use them.
2. **CLI.** If `command -v postatron` succeeds and `POSTATRON_API_KEY` is set, use `postatron`, always with `--json` so you read the API's own response.
3. **REST.** Otherwise call `https://api.postatron.com/v1/...` with `Authorization: Bearer $POSTATRON_API_KEY`. See [reference.md](reference.md).

If none is set up, tell the person how to connect and stop:

- Claude: add a custom connector with the URL `https://api.postatron.com/mcp` and sign in.
- Claude Code: `/plugin install postatron --marketplace henrwal/postatron-cli` (or `claude mcp add --transport http postatron https://api.postatron.com/mcp`), then `/mcp` to sign in.
- Terminal: create a key at https://postatron.com/dashboard/api, `export POSTATRON_API_KEY=...`, and install the CLI with `go install github.com/henrwal/postatron-cli/cmd/postatron@latest` or a binary from https://github.com/henrwal/postatron-cli/releases.

Never ask for the API key in the conversation, and never print it.

## Creating a post

Work through these in order.

1. **Find the accounts.** `list_profiles` (CLI `postatron list-profiles --json`) shows each profile with its accounts.
2. **Choose the profile.**
   - The person named a brand or client: pass it as `profile` (its name or id).
   - A requested platform has an account in more than one profile and nobody said which: ask. Do not guess; the API refuses the post anyway.
   - Only one profile has the platforms: no `profile` needed.
3. **Work out the time.**
   - "Now", or no time given: omit `scheduled_at` and the post publishes immediately.
   - Otherwise build an RFC 3339 timestamp **with the person's UTC offset**, e.g. `2026-10-07T09:00:00+01:00`; the API converts it.
     Take the offset from the conversation, or from `date +%z` on their machine. If you cannot tell their timezone, ask.
   - "Tomorrow" means tomorrow in their timezone, not in UTC.
   - It must be at least 5 minutes ahead.
4. **Check the content.** See the limits below. Instagram and TikTok need an image or video, YouTube a video.
5. **Attach media** if there is any; see Media.
6. **Confirm when anything was inferred.** If you chose the profile, account or time yourself, or the post goes out immediately, show the text, the accounts as `@handle on Platform`, and the time in the person's timezone, then wait for a yes. A fully specified request to schedule can go straight through.
7. **Create it**, then report the post id, the accounts, the time in their timezone and its status. Mention that a scheduled post can be cancelled until it publishes.

### Example: "schedule this to LinkedIn and X tomorrow at 9am"

1. `list_profiles`: LinkedIn and X are both in Default only.
2. The person is in London, on BST (+01:00), and today is 6 October 2026, so the time is `2026-10-07T09:00:00+01:00`.
3. The text is 214 characters: under X's 280.
4. Everything was specified, so create it:

```json
{ "content": "...", "platforms": ["linkedin", "x"], "scheduled_at": "2026-10-07T09:00:00+01:00" }
```

CLI:

```bash
postatron create-post --json --content "..." --platforms linkedin,x --scheduled-at 2026-10-07T09:00:00+01:00
```

5. Reply: "Scheduled for 9:00 tomorrow (Wed 7 Oct) on LinkedIn (@henry-wallis) and X (@henrwalli). Post id `0AbC...`; say if you want it cancelled."

## Content limits

| Platform | Characters | Media |
| --- | --- | --- |
| X | 280 (25,000 for X Premium accounts) | optional |
| Bluesky | 300 | optional |
| Threads | 500 | optional |
| LinkedIn | 3,000 | optional |
| Facebook | 63,206 | optional |
| Instagram | 2,200 | an image or a video, required |
| TikTok | 150 | an image or a video, required |
| YouTube | | a video, required |

When one text is too long for one platform, say which and offer a shorter version for it as a separate post, rather than cutting it silently.

## Media

- **A public https URL** (a page the person linked, a CDN file): pass it in `media_urls`. Postatron downloads it.
- **A file on the person's device**, or one only you can see: you cannot send the bytes. Call `create_upload` (CLI `postatron create-upload --purpose "..."`), give the person the `upload_url`, and wait for them to say it is done. Then `get_upload` must show `READY`; pass its id in `media_ids`. Ask them to tell you when they have uploaded rather than polling.
- Up to four items per post, all images or one video; never a mix.

## Everything else

| Task | MCP tool | CLI |
| --- | --- | --- |
| What is connected | `list_profiles`, `list_accounts` | `list-profiles`, `list-accounts` |
| What is scheduled | `list_posts` with `status: scheduled` | `list-posts --status scheduled` |
| Did it go out | `get_post` | `get-post <id>` |
| Change a draft or scheduled post (text, time, add or drop accounts) | `update_post` | `update-post <id>` |
| Their drafts | `list_posts` with `status: draft` | `list-posts --status draft` |
| Send a draft out | `update_post` with `publish_now: true` or `scheduled_at` | `update-post <id> --publish-now` |
| Cancel a scheduled post | `delete_post` | `delete-post <id>` |
| Connect a social account | `connect_account` | `connect-account <platform>` |
| How posts are doing | `get_analytics` (`range` 7d, 30d or 90d; optional `profile`, `platform`) | `get-analytics` |
| Quota left this month | `get_usage` | `get-usage` |

- A post's `deliveries` hold one entry per account: `published` with a `url`, or `failed` with an `error`. Report failures account by account; a post can be `partial`.
- When the person adds to a post they already scheduled ("put it on Instagram too"), use `update_post` with `add_platforms`, not a second `create_post`: it stays one post, edited and cancelled as one.
- "Submit my draft to LinkedIn": find it with `list_posts` and `status: draft`, then `update_post` with `add_platforms: ["linkedin"]` and `publish_now: true` (or `scheduled_at` for later). If more than one draft could be the one, show them and ask. Confirm before publishing now: it goes to their real account.
- When a platform they ask for is not connected, call `connect_account` and give them the `connect_url`. Nothing is connected until they finish signing in there, so wait for them to say so, then check with `list_accounts`.
- `delete_post` only cancels posts that are still scheduled. Once published, it reports `cancelled: false`, and the post has to be removed on the platform itself.
- Analytics has no X numbers: X charges per read, so Postatron does not fetch them. A metric a platform does not report is `null`; say "not reported", not zero.

## Errors

| Code | What to do |
| --- | --- |
| `validation_error` | Read the message; it names the field. Fix it and retry, or ask. A message listing profiles means pick one, or ask. |
| `quota_exceeded` | Stop and tell the person: they are at their plan's monthly limit. Do not retry. If `details.resource` is `x_link_posts`, only X posts with a link are capped: offer to post to X without the link, or to the other platforms. |
| `rate_limited` | Wait for the seconds in `retry_after` (or the `Retry-After` header), then retry once. |
| `unauthorized`, `insufficient_scope` | The key or connection is missing a permission. Tell the person; do not retry. |
| `subscription_required` | Postatron's API needs a paid plan. Tell the person. |

## Do not

- Post to test anything. Every post is real and public on the person's accounts.
- Retry a `create_post` that may have succeeded (a timeout, for example). Check `list_posts` first, or you publish twice.
- Invent account ids or profile names. Use what `list_profiles` returns.
