# postatron-cli

Command line client and [MCP](https://modelcontextprotocol.io) server for [Postatron](https://postatron.com), a social media scheduler that publishes one post to every platform you have connected.

This repository holds everything you need to drive Postatron from a terminal or from an AI agent.
The service itself is closed source.

- `cmd/postatron` - the CLI.
- `cmd/postatron-mcp` - a local MCP server over stdio, for clients that cannot reach a remote one.
- `apiv1` - a Go client for the REST API, and the request and response types the API itself is built on.

## Connect Claude (no install)

Postatron hosts its MCP server at:

```
https://api.postatron.com/mcp
```

Add it as a custom connector in Claude (Settings → Connectors → Add custom connector) and sign in when asked.

Claude Code: install the plugin, which connects the server and adds the Postatron skill in one step.

```
/plugin install postatron --marketplace henrwal/postatron-cli
```

Run it inside Claude Code in a terminal (v2.1.275 or later); it asks to add the Postatron marketplace, then installs.
The Claude desktop app has no `/plugin` command: there, add the connector URL above instead.

Then run `/mcp` and sign in to Postatron.
To connect the server alone, without the skill:

```
claude mcp add --transport http postatron https://api.postatron.com/mcp
```

Any client that supports remote MCP servers with OAuth 2.1 and dynamic client registration works the same way.
There is no key to copy: the client registers itself and you approve it on a Postatron consent page.

## Agent skill

```
npx skills add henrwal/postatron-cli
```

Installs the `postatron` skill into Claude Code, Codex, Cursor and the other agents the [skills](https://skills.sh) CLI supports.
In Claude Code the plugin above already includes it.
It teaches the agent the workflow behind a request such as "schedule this to LinkedIn and X tomorrow at 9am": find the right profile, work out the time in your timezone, respect each platform's limits, attach media and confirm before anything is published.
It works through the MCP tools when they are connected, and through the CLI or REST otherwise.

## Install the CLI

Download a binary for your platform from [Releases](https://github.com/henrwal/postatron-cli/releases), or build from source with Go 1.23 or newer:

```
go install github.com/henrwal/postatron-cli/cmd/postatron@latest
go install github.com/henrwal/postatron-cli/cmd/postatron-mcp@latest
```

## Get an API key

Create one at [postatron.com/dashboard/api](https://postatron.com/dashboard/api).
Keys are scoped, and the key is shown once at creation.
The API, the MCP server and the CLI are included on every paid plan.

```
export POSTATRON_API_KEY=ptn_your_key_here
```

`POSTATRON_API_URL` overrides the API host if you need it to.

## Use the CLI

```
postatron create-post --content "Shipping today." --platforms x,linkedin
postatron create-post --content "Out on Friday." --platforms x --scheduled-at 2026-09-12T09:00:00Z
postatron create-post --content "New in store" --platforms instagram --profile Acme --media-urls https://example.com/shoe.jpg
postatron list-posts --status scheduled
postatron get-post <id>
postatron delete-post <id>
postatron delete-posts <id> <id>
postatron list-accounts
postatron list-profiles
postatron list-queues
postatron create-queue --name "Weekday mornings" --timezone Europe/London --slots "mon 09:00,wed 09:00"
postatron create-post --content "Whenever there's room" --platforms x --queue "Weekday mornings"
postatron get-usage
postatron get-analytics --range 7d
postatron create-upload --purpose "photo for Tuesday"
postatron get-upload <id>
```

Add `--json` to any command for the raw API response.
Full reference in [docs/cli.md](docs/cli.md).

## Profiles

A profile groups connected accounts, usually one per brand or client, and holds at most one account per platform.
Everyone has a Default profile; paid plans can add more.
Pass `--profile` (CLI) or `profile` (API and MCP) to post as one brand.
When a platform has accounts in more than one profile, posting by platform without a profile is refused rather than sent to every brand at once.

## Local MCP server

For clients that only launch local servers, `postatron-mcp` exposes the same tools over stdio with an API key.
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

Details in [docs/mcp.md](docs/mcp.md).

## Documentation

- [docs/api.md](docs/api.md) - REST API reference, auth, rate limits, errors.
- [docs/mcp.md](docs/mcp.md) - MCP tools and client setup.
- [docs/cli.md](docs/cli.md) - every command and flag.

## Licence

MIT. See [LICENSE](LICENSE).
