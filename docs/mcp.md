# Postatron MCP server

Postatron lets an AI agent post, schedule, attach media, check delivery and read analytics through the Model Context Protocol.
There are two ways to connect, with the same tools:

- **Remote** (recommended): `https://api.postatron.com/mcp`, signed in with OAuth. Nothing to install and no key to manage.
- **Local**: `postatron-mcp`, a binary that speaks MCP over stdio and calls the API with your API key, for clients that only launch local servers.

| Tool | Endpoint | Arguments |
| --- | --- | --- |
| `create_post` | `POST /v1/posts` | `content` (required), `platforms` or `account_ids`, `profile`, `media_urls`, `media_ids`, `scheduled_at` or `queue`, `x`, `instagram`, `tiktok` |
| `list_posts` | `GET /v1/posts` | `status`, `platform`, `profile`, `from`, `to`, `limit`, `cursor` |
| `get_post` | `GET /v1/posts/{id}` | post id |
| `update_post` | `PATCH /v1/posts/{id}` | `post_id` (required), `content`, `scheduled_at`, `add_platforms`, `add_account_ids`, `remove_account_ids`, `profile`, `publish_now`, `x`, `instagram`, `tiktok` |
| `delete_post` | `DELETE /v1/posts/{id}` | post id |
| `delete_posts` | `POST /v1/posts/bulk-delete` | `post_ids` (required, up to 100) |
| `list_accounts` | `GET /v1/accounts` | none |
| `connect_account` | `POST /v1/accounts/connect` | `platform` (required), `profile` |
| `list_profiles` | `GET /v1/profiles` | none |
| `list_queues` | `GET /v1/queues` | none |
| `create_queue` | `POST /v1/queues` | `name`, `timezone` (required), `profile`, `slots`, `paused` |
| `update_queue` | `PATCH /v1/queues/{id}` | `queue` (required, name or id), `name`, `profile`, `timezone`, `slots`, `active` |
| `delete_queue` | `DELETE /v1/queues/{id}` | `queue` (required, name or id) |
| `get_usage` | `GET /v1/usage` | none |
| `get_analytics` | `GET /v1/analytics` | `range`, `platform`, `account_id`, `profile`, `source` |
| `create_upload` | `POST /v1/media/uploads` | `purpose` |
| `get_upload` | `GET /v1/media/uploads/{id}` | `upload_id` |

The post id argument is `post_id` on the remote server and `id` on the local one.
Tool calls are metered and rate limited exactly like the API (see `docs/api.md`).
API errors come back as tool errors with the API's `code` and message, so the model can explain a quota or scope problem, or ask which profile you meant, instead of failing silently.

## Remote

Claude: **Settings → Connectors → Add custom connector**, URL `https://api.postatron.com/mcp`, then sign in to Postatron and approve the connection.

Claude Code: the plugin connects the server and adds the Postatron skill, which teaches the agent the workflow (profiles, timezones, platform limits, confirming before publishing).

```
/plugin install postatron --marketplace henrwal/postatron-cli
```

Run it inside Claude Code in a terminal (v2.1.275 or later); it asks to add the Postatron marketplace, then installs.
The Claude desktop app has no `/plugin` command: there, add the connector URL above instead.

Or connect the server alone:

```bash
claude mcp add --transport http postatron https://api.postatron.com/mcp
```

Either way, run `/mcp` in a session to sign in.

Any other client that supports remote MCP servers with OAuth 2.1 and dynamic client registration connects the same way.

## Local

### Install

Install from source with Go 1.23 or later:

```bash
go install github.com/henrwal/postatron-cli/cmd/postatron-mcp@latest
```

The binary lands in `$(go env GOPATH)/bin`; make sure that directory is on your `PATH`.
Or build from a checkout: `make build` produces `bin/postatron-mcp` and `bin/postatron`.

### Create an API key

1. Sign in at `https://postatron.com/dashboard/api`.
2. **New key**, name it (for example `Claude Desktop`), keep all four scopes or drop `posts:write` for a read-only agent.
3. Copy the key. It is shown once.

### Claude Desktop

1. Open Claude Desktop, then **Settings → Developer → Edit Config**.
   This opens `claude_desktop_config.json` (macOS: `~/Library/Application Support/Claude/claude_desktop_config.json`).
2. Add the server:

   ```json
   {
     "mcpServers": {
       "postatron": {
         "command": "postatron-mcp",
         "env": {
           "POSTATRON_API_KEY": "ptn_YOUR_KEY"
         }
       }
     }
   }
   ```

   If `postatron-mcp` is not on Claude's `PATH`, use the absolute path, for example `/Users/you/go/bin/postatron-mcp`.
3. Restart Claude Desktop.
   The tools icon shows `postatron` with ten tools.

### Claude Code

```bash
claude mcp add postatron -e POSTATRON_API_KEY=ptn_YOUR_KEY -- postatron-mcp
```

`claude mcp list` shows the server and `claude mcp remove postatron` drops it.

### Other clients

Any client that launches stdio MCP servers works the same way (Cursor, Windsurf, VS Code, Zed): command `postatron-mcp`, environment `POSTATRON_API_KEY`.

### Environment variables

| Variable | Purpose |
| --- | --- |
| `POSTATRON_API_KEY` | Required. The API key. |
| `POSTATRON_API_URL` | Optional. Overrides `https://api.postatron.com` (useful against a staging stack). |

Logs go to stderr; stdout carries the protocol and must stay clean.

## Worked example

> **You:** What have I got connected, and how much of my quota is left?
>
> **Claude** calls `list_profiles` and `get_usage`, then answers: "Your Default profile has X (@henry) and LinkedIn; Acme has X (@acmehq) and Instagram. This month you've used 42 of 300 destination-posts."
>
> **You:** Schedule "New season, new shoes." with this photo to Acme's Instagram and X for Wednesday at 9am UK time.
>
> **Claude** calls `create_upload`, gives you the link, and once you say the photo is up, calls `create_post` with `{"content": "...", "platforms": ["instagram", "x"], "profile": "Acme", "media_ids": ["u_..."], "scheduled_at": "2026-09-09T08:00:00Z"}`.
>
> **You:** Actually cancel that.
>
> **Claude** calls `delete_post` with the id and reports `cancelled: true`.

Claude asks before running write tools (`create_post`, `update_post`, `delete_post`, `delete_posts`, `delete_queue`) unless you have allowed them for the session.

Deleting a post that has already been published only removes it from Postatron: it stays live on the platforms, and the tool's answer says where.

## Team workspaces

On a team, the sign-in page asks whether to connect the agent to your own account or to the team's workspace.
In the workspace it sees only the profiles you were given, posts on the owner's plan, and what it does appears in the team's activity under your name.
It stops working if you leave the team; reconnect it to your own account then.

## Security notes

- With the remote server, no long-lived secret sits in a config file: access tokens expire and refresh tokens rotate on every use.
- With the local server, the key never leaves your machine except in the `Authorization` header to `api.postatron.com`.
- Give agents the narrowest scopes they need; a research agent only needs `posts:read`, `accounts:read` and `usage:read`.
- Revoke keys from the dashboard the moment a laptop or config file is lost.
- Scheduled posts need `scheduled_at` at least five minutes ahead; an agent that keeps proposing "now" gets a `validation_error` it can read and fix.
