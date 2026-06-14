---
title: "Introduction"
description: "What ig is and how it is put together."
weight: 10
---

A command line for Instagram.

ig is a single binary. It speaks to Instagram over plain HTTPS,
shapes the responses into clean records, and gets out of your way. There is
nothing to sign up for and nothing to run alongside it. No API key, no login,
no cookie.

## How it is built

- A **library package** (`instagram`) holds the HTTP client and the typed
  data models. It paces requests, sets an honest User-Agent, and retries the
  transient failures any public site throws under load.
- A **domain** (`instagram/domain.go`) declares each operation once on the
  [any-cli/kit](https://github.com/tamnd/any-cli) framework. That single
  declaration becomes a CLI command, an HTTP route, an MCP tool, and a
  resource-URI dereference. It is the one place each operation lives.
- A thin **`cmd/ig`** hands the assembled app to `kit.Run`, which
  builds the command tree and the serve and mcp surfaces.

## Two planes

Instagram serves the same public data through two channels that fail
differently, and ig reads both.

The **SSR plane** reads the Open Graph tags a logged-out post or reel page
ships to a crawler: the author, the caption, the display image, the date, and
the like and comment counts rounded the way the page prints them (for example
"585K likes"). `ig post` and `ig reel` ride this plane and answer from anywhere,
including a datacenter IP. This is the reliable floor.

The **API plane** calls `/api/v1/users/web_profile_info` with the web app id
the browser sends. It returns the full profile and the roughly twelve most
recent posts as exact JSON. `ig profile`, `ig posts`, and `ig raw` ride this
plane. From a residential session it answers; from a datacenter IP it is walled,
and ig exits 4 with a clear message so you know the surface was gated.

Every record carries a `source` field that records which plane it came from:
`api` for exact counts, `ssr` for rounded counts.

## One operation, four surfaces

Because an operation is surface-neutral, the same `profile` you run on the
command line is also a route and a tool:

```bash
ig profile instagram                       # the command
ig serve --addr :7777                      # GET /v1/profile/instagram
ig mcp                                     # the profile tool, over stdio
ant get instagram://profile/instagram      # the URI dereference (via a host)
```

You get the same record shape on every surface from one handler.

## Scope

ig is a read-only client over data Instagram already serves
publicly. It reads that data and shapes it for you. That narrow scope keeps it a
single small binary with no database, no daemon, and no setup.

Next: [install it](/getting-started/installation/), then take the
[quick start](/getting-started/quick-start/).
