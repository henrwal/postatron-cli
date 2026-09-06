# postatron-cli

Command line client and [MCP](https://modelcontextprotocol.io) server for
[Postatron](https://postatron.com), a social media scheduler that publishes one
post to every platform you have connected.

This repository holds everything you need to drive Postatron from a terminal or
from an AI agent. The service itself is closed source.

- `cmd/postatron` - the CLI, six commands.
- `cmd/postatron-mcp` - an MCP server, six tools, so Claude can post for you.
- `apiv1` - a Go client for the REST API, six endpoints.

All three cover the same six operations, one for one.

## Install

Download a binary for your platform from
[Releases](https://github.com/henrwal/postatron-cli/releases), or build from
source with Go 1.23 or newer:

```
go install github.com/henrwal/postatron-cli/cmd/postatron@latest
go install github.com/henrwal/postatron-cli/cmd/postatron-mcp@latest
```

## Get an API key

Create one at [postatron.com/dashboard/api](https://postatron.com/dashboard/api).
Keys are scoped, and the key is shown once at creation. The API, the MCP server
and the CLI are included on every paid plan.

```
export POSTATRON_API_KEY=ptn_your_key_here
```

`POSTATRON_API_URL` overrides the API host if you need it to.

## Use the CLI

```
postatron create-post --content "Shipping today." --platforms x,linkedin
postatron create-post --content "Out on Friday." --platforms x --scheduled-at 2026-09-12T09:00:00Z
postatron list-posts --status scheduled
postatron get-post <id>
postatron delete-post <id>
postatron list-accounts
postatron get-usage
```

Add `--json` to any command for the raw API response. Full reference in
[docs/cli.md](docs/cli.md).

## Connect Claude

Claude Code:

```
claude mcp add postatron -e POSTATRON_API_KEY=$POSTATRON_API_KEY -- postatron-mcp
```

Claude Desktop, in `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "postatron": {
      "command": "postatron-mcp",
      "env": { "POSTATRON_API_KEY": "ptn_your_key_here" }
    }
  }
}
```

Claude then has `create_post`, `list_posts`, `get_post`, `delete_post`,
`list_accounts` and `get_usage`. Worked example in [docs/mcp.md](docs/mcp.md).

ChatGPT custom connectors are not supported, because they require OAuth 2.1 with
dynamic client registration and reject bearer keys.

## Documentation

- [docs/api.md](docs/api.md) - REST API reference, auth, rate limits, errors.
- [docs/mcp.md](docs/mcp.md) - MCP tools and client setup.
- [docs/cli.md](docs/cli.md) - every command and flag.

## Licence

MIT. See [LICENSE](LICENSE).
