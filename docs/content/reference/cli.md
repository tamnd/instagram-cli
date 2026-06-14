---
title: "CLI"
description: "Every command and subcommand, with the flags that matter."
weight: 10
---

```
ig <command> [arguments] [flags]
```

Run `ig <command> --help` for the full flag list on any command. This
page is the map of the whole surface.

## Commands

| Command | Plane | What it does |
|---|---|---|
| `profile <username\|url>` | API | One profile record |
| `posts <username\|url>` | API | The recent posts on a profile (about 12) |
| `post <shortcode\|url>` | SSR | One post by shortcode or URL |
| `reel <shortcode\|url>` | SSR | One reel by shortcode or URL |
| `raw <username\|url>` | API | The profile's upstream `web_profile_info` JSON |
| `serve [--addr]` | | Serve the operations over HTTP as NDJSON |
| `mcp` | | Run as an MCP server over stdio |
| `version` | | Print the version and exit |

The **API** commands call the profile endpoint, which answers from a
residential connection and is walled from a datacenter IP. The **SSR** commands
read the Open Graph tags a post or reel page ships to a crawler, and answer from
anywhere. See [troubleshooting](/reference/troubleshooting/) for what the wall
looks like.

A shortcode is the media id in base64url, so `post` accepts a bare shortcode
like `DZf6PYtGyay`, a numeric media id, or a `/p/...` or `/reel/...` URL. A
profile ref accepts `instagram`, `@instagram`, or a profile URL.

```bash
ig profile instagram
ig posts instagram -n 12
ig post DZf6PYtGyay
ig post https://www.instagram.com/p/DZf6PYtGyay/
ig reel https://www.instagram.com/reel/DZdQzr7vDa0/
ig raw instagram
```

## Global flags

These are shared by every operation, so they work the same on every command.

| Flag | Meaning |
|---|---|
| `-o, --output` | Output format: `auto`, `table`, `json`, `jsonl`, `csv`, `tsv`, `url`, `raw` |
| `--fields` | Comma-separated columns to keep |
| `--template` | Go text/template applied per record |
| `--no-header` | Omit the header row in `table` and `csv` |
| `-n, --limit` | Stop after N records (0 means the command default) |
| `--user-agent` | Override the User-Agent |
| `--timeout` | Per-request timeout |
| `--retries` | Retry attempts on 429 or 5xx |

## Exit codes

| Code | Meaning |
|---|---|
| `0` | Success, at least one record |
| `1` | Error |
| `2` | Usage error |
| `3` | No data (a valid empty result) |
| `4` | Walled (the surface needs a residential session) |
| `6` | Not found (the username or shortcode does not exist) |

See [output formats](/reference/output/) for what `-o`, `--fields`, and
`--template` produce, and [configuration](/reference/configuration/) for
environment variables and defaults.
