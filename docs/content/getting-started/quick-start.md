---
title: "Quick start"
description: "Fetch your first record with ig."
weight: 30
---

Once `ig` is on your `PATH`, fetch a profile. The argument is a username, an
`@username`, or a profile URL:

```bash
ig profile instagram
```

By default you get an aligned table. Ask for JSON when you want to pipe it:

```bash
$ ig profile instagram -o json
[
  {
    "username": "instagram",
    "user_id": "25025320",
    "full_name": "Instagram",
    "url": "https://www.instagram.com/instagram/",
    "biography": "...",
    "follower_count": 686000000,
    "following_count": 234,
    "post_count": 7800,
    "is_verified": true,
    "source": "api"
  }
]
```

The `source` field says which plane the record came from: `api` for the exact
counts on the profile plane, `ssr` for the rounded counts on a post page.

## Fetch a post

A post comes from its shortcode, a numeric media id, or a `/p/...` or
`/reel/...` URL:

```bash
ig post DZf6PYtGyay
ig post https://www.instagram.com/p/DZf6PYtGyay/
ig reel https://www.instagram.com/reel/DZdQzr7vDa0/
```

## Shape the output

The same flags work on every command:

```bash
ig profile nasa --fields username,follower_count    # keep only these columns
ig profile nasa --template '{{.username}} {{.follower_count}}'
ig posts instagram -o jsonl | jq .url               # one object per line, into jq
```

`-o` takes `table`, `json`, `jsonl`, `csv`, `tsv`, `url`, or `raw`. Left to
`auto`, it prints a table to a terminal and JSONL into a pipe, so the same
command reads well by hand and parses cleanly downstream. See
[output formats](/reference/output/) for the full contract.

## List the recent posts

`posts` returns the roughly twelve most recent posts on a profile, each one its
own addressable record:

```bash
ig posts instagram -n 12              # the recent posts
ig posts instagram -o url             # just the post URLs
ig posts instagram -o csv --fields shortcode,type,like_count,comment_count
```

## The two planes

`ig post` and `ig reel` read the Open Graph tags a post page ships to a
crawler, so they answer from anywhere, including a datacenter IP. `ig profile`,
`ig posts`, and `ig raw` call the profile endpoint, which answers from a
residential connection and is walled from a datacenter IP. When the firewall
gates a call, `ig` exits 4 with a clear message:

```
Walled: this surface needs a residential session.
```

That is not a bug. The post and reel commands still work from the same machine.

## Serve it instead

The same operations are available over HTTP and to agents over MCP:

```bash
ig serve --addr :7777 &
curl -s 'localhost:7777/v1/profile/instagram'    # NDJSON, one record per line
ig mcp                                            # MCP over stdio
```

## What to do next

The [CLI reference](/reference/cli/) lists every command and flag. The
[guides](/guides/) cover using `ig` as a resource-URI driver. The
[troubleshooting](/reference/troubleshooting/) page explains the walled exit and
the surfaces that are out of scope today.
