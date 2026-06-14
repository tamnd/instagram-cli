---
title: "ig"
description: "ig reads public Instagram data and prints clean, pipeable records. No API key, no login."
heroTitle: "instagram, from the command line"
heroLead: "A command line for Instagram. One pure-Go binary, no API key, output that pipes into the rest of your tools, and a resource-URI driver other programs can address."
heroPrimaryURL: "/getting-started/quick-start/"
heroPrimaryText: "Get started"
---

`ig` reads public Instagram data over plain HTTPS, shapes it into
clean records, and gets out of your way. No API key, no login, no cookie.

```bash
ig profile instagram                # one profile record
ig posts instagram -n 12            # the recent posts on a profile
ig post DZf6PYtGyay                 # one post by shortcode
ig serve --addr :7777               # the same operations over HTTP
```

There is nothing to sign up for and nothing to run alongside it. Output adapts
to where it goes: an aligned table on your terminal, JSONL the moment you pipe
it somewhere. Every record carries its own `url` and a `source` field that says
where it came from: `api` for the exact counts on the profile plane, `ssr` for
the rounded counts Instagram prints on a post page.

## Two ways to use it

- **As a command** for reading Instagram by hand or in a script. Start with
  the [quick start](/getting-started/quick-start/).
- **As a resource-URI driver** so a host like
  [ant](https://github.com/tamnd/ant) can address Instagram as
  `instagram://` URIs and follow links across sites. See
  [resource URIs](/guides/resource-uris/).

Both are the same code: one operation, declared once, is a CLI command, an HTTP
route, an MCP tool, and a URI dereference.

## Where to go next

- New here? Read the [introduction](/getting-started/introduction/), then the
  [quick start](/getting-started/quick-start/).
- Installing? See [installation](/getting-started/installation/).
- Doing a specific job? The [guides](/guides/) are task-first.
- Need every flag? The [CLI reference](/reference/cli/) is the full surface.
