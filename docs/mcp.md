# Postatron MCP server

`postatron-mcp` lets an AI agent post, schedule and check delivery through the Model Context Protocol.
It runs locally over stdio, talks to the REST API with your API key, and exposes exactly six tools, one per endpoint:

| Tool | Endpoint | Arguments |
| --- | --- | --- |
| `create_post` | `POST /v1/posts` | `content` (required), `platforms` or `account_ids`, `scheduled_at` |
| `list_posts` | `GET /v1/posts` | `status`, `platform`, `from`, `to`, `limit`, `cursor` |
| `get_post` | `GET /v1/posts/{id}` | `id` |
| `delete_post` | `DELETE /v1/posts/{id}` | `id` |
| `list_accounts` | `GET /v1/accounts` | none |
| `get_usage` | `GET /v1/usage` | none |

Tool calls are metered and rate limited exactly like the API (see `docs/api.md`).
API errors come back as tool errors with the API's `code` and message, so the model can explain a quota or scope problem instead of failing silently.

## Install

Prebuilt binaries are not published yet, so install from source with Go 1.23 or later:

```bash
go install github.com/henrwal/postatron-cli/cmd/postatron-mcp@latest
```

The binary lands in `$(go env GOPATH)/bin`; make sure that directory is on your `PATH`.
Or build from a checkout: `cd backend && make cli` produces `bin/postatron-mcp` and `bin/postatron`.

## Create an API key

1. Sign in at `https://postatron.com/dashboard/api`.
2. **New key**, name it (for example `Claude Desktop`), keep all four scopes or drop `posts:write` for a read-only agent.
3. Copy the key. It is shown once.

## Worked example: Claude Desktop

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

   If `postatron-mcp` is not on Claude's `PATH`, use the absolute path, for example `/Users/henry/go/bin/postatron-mcp`.
3. Restart Claude Desktop.
   The tools icon shows `postatron` with six tools.
4. Try it:

   > **You:** What have I got connected, and how much of my quota is left?
   >
   > **Claude** calls `list_accounts` and `get_usage`, then answers: "You have X (@henry), LinkedIn and Bluesky connected. This month you've used 42 of 300 destination-posts and 1 of 10 X link posts."
   >
   > **You:** Schedule "Shipping the Postatron API today. Docs in the thread." to X and LinkedIn for Wednesday at 9am UK time.
   >
   > **Claude** converts the time to UTC and calls `create_post` with `{"content": "...", "platforms": ["x", "linkedin"], "scheduled_at": "2026-09-09T08:00:00Z"}`, then confirms the post id and that two destination-posts were reserved.
   >
   > **You:** Actually cancel that.
   >
   > **Claude** calls `delete_post` with the id and reports `cancelled: true`.

Claude will ask before running write tools (`create_post`, `delete_post`) unless you have allowed them for the session.

## Claude Code

```bash
claude mcp add postatron -e POSTATRON_API_KEY=ptn_YOUR_KEY -- postatron-mcp
```

Then in a session: "use postatron to list my scheduled posts for next week".
`claude mcp list` shows the server and `claude mcp remove postatron` drops it.

## Other clients

Any client that launches stdio MCP servers works the same way (Cursor, Windsurf, VS Code, Zed): command `postatron-mcp`, environment `POSTATRON_API_KEY`.

ChatGPT is different: its custom connectors only accept remote MCP servers protected by OAuth 2.1 with dynamic client registration, and reject API keys.
Postatron does not host a remote MCP endpoint or an OAuth server in this release, so ChatGPT cannot connect yet.
The CLI and the REST API are the alternatives for ChatGPT-driven automations (for example through an Action or a custom GPT calling the API).

## Environment variables

| Variable | Purpose |
| --- | --- |
| `POSTATRON_API_KEY` | Required. The API key. |
| `POSTATRON_API_URL` | Optional. Overrides `https://api.postatron.com` (useful against a staging stack). |

Logs go to stderr; stdout carries the protocol and must stay clean.

## Security notes

- The key never leaves your machine except in the `Authorization` header to `api.postatron.com`.
- Give agents the narrowest scopes they need; a research agent only needs `posts:read`, `accounts:read` and `usage:read`.
- Revoke keys from the dashboard the moment a laptop or config file is lost.
- Scheduled posts need `scheduled_at` at least five minutes ahead; an agent that keeps proposing "now" gets a `validation_error` it can read and fix.
