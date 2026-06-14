---
title: "Resource URIs"
description: "Use ig as a database/sql-style driver so a host program can address instagram as instagram:// URIs."
weight: 20
---

`ig` is a command line, and the `instagram` Go package is also a
small driver that makes Instagram addressable as a resource URI. A host
program registers it the way a program registers a database driver with
`database/sql`, then dereferences `instagram://` URIs without knowing
anything about how Instagram is fetched.

The host that does this today is [ant](https://github.com/tamnd/ant), a single
binary that puts one URI namespace over a family of site tools. The examples
below use `ant`; any program that links the package gets the same behaviour.

## Mounting the driver

A host enables the driver with one blank import, exactly like `import _
"github.com/lib/pq"`:

```go
import _ "github.com/tamnd/instagram-cli/instagram"
```

The package's `init` registers a domain with the scheme `instagram` for the
host `instagram.com`. The standalone `ig` binary does not change.

## Addressing records

A URI is `scheme://authority/id`. The driver exposes three types:

| URI                                  | What it is                                  |
| ------------------------------------ | ------------------------------------------- |
| `instagram://profile/<username>`     | a profile, keyed by username                |
| `instagram://post/<shortcode>`       | a post, keyed by its shortcode              |
| `instagram://reel/<shortcode>`       | a reel, keyed by its shortcode              |

```bash
ant get instagram://profile/instagram    # the profile record
ant get instagram://post/DZf6PYtGyay      # one post
ant cat instagram://profile/instagram     # just the biography text
ant url instagram://post/DZf6PYtGyay      # the live https URL
```

`ant cat` prints the `body` of a record, which is the biography for a profile
and the caption for a post or reel. `ant url` returns the live https URL each
record carries.

## Walking the graph

`ls` lists the recent posts on a profile, and every member is itself an
addressable URI, so a host can follow the graph and write it to disk:

```bash
ant ls     instagram://profile/instagram             # the recent posts
ant export instagram://profile/instagram --follow 1 --to ./data
```

Each listed post is an `instagram://post/<shortcode>` URI in its own right, so
`ant get`, `ant cat`, and `ant url` all work on it. Because `ig profile` and
`ig posts` ride the API plane, these resolve from a residential connection and
are walled from a datacenter IP, the same as the commands.

## Why this is the same code

The driver and the binary share one definition per operation. A resolver op
answers both `ig profile` on the command line and `ant get
instagram://profile/...` through a host, from the same handler and the same
client. There is no second implementation to keep in step.
