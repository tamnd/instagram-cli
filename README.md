# ig

A command line for Instagram. `ig` reads public Instagram data and prints clean,
pipeable records. One pure-Go binary, no API key, no login, no cookie.

It reads two public surfaces. A post or reel page is read from the Open Graph
tags it ships to a crawler, so `ig post` and `ig reel` answer from anywhere. A
profile and its recent posts come from the `web_profile_info` endpoint the web
client calls, which answers from a residential connection and is walled from a
datacenter IP. Every request is paced, retried on transient failures, and sent
with an honest User-Agent.

The same package is also a [resource-URI driver](#use-it-as-a-resource-uri-driver),
so a host program like [ant](https://github.com/tamnd/ant) can address Instagram
as `instagram://` URIs.

`ig` is an independent tool. It is not affiliated with, authorized, or endorsed
by Instagram or Meta.

## Install

```bash
go install github.com/tamnd/instagram-cli/cmd/ig@latest
```

Or grab a prebuilt binary from the [releases](https://github.com/tamnd/instagram-cli/releases),
or run the container image:

```bash
docker run --rm ghcr.io/tamnd/ig:latest --help
```

## Usage

```bash
ig profile instagram                     # one profile record
ig posts instagram -n 12                  # the recent posts on a profile
ig post DZf6PYtGyay                        # one post by shortcode
ig post https://www.instagram.com/p/DZf6PYtGyay/   # or by URL
ig reel https://www.instagram.com/reel/DZdQzr7vDa0/
ig raw instagram                           # the profile's upstream JSON
```

Records come out as table, JSON, JSONL, CSV, TSV, url, or raw:

```bash
ig profile nasa -o json
ig posts instagram -o csv --fields shortcode,type,like_count,comment_count
ig posts instagram -o url                  # just the links
ig profile nasa --template '{{.username}} {{.follower_count}}'
```

Every record carries its own `url`, and a `source` field that says where it came
from: `api` for the exact counts on the profile plane, `ssr` for the rounded
counts Instagram prints on a post page.

### Global flags

```
-o, --output      table|json|jsonl|csv|tsv|url|raw   (auto: table on a TTY, jsonl when piped)
    --fields      comma-separated columns to include
    --template    Go text/template applied per record
-n, --limit       max records (0 = command default)
    --user-agent  override the User-Agent
    --timeout     per-request timeout
    --retries     retry attempts on 429/5xx
```

## Two planes, two reliabilities

Instagram serves the same public data through two channels that fail
differently.

The **SSR plane** reads the Open Graph tags a logged-out post or reel page ships
to a crawler: the author, the caption, the display image, the date, and the like
and comment counts (rounded, the way the page prints them). `ig post` and
`ig reel` ride it and answer from anywhere, including a datacenter IP.

The **API plane** calls `/api/v1/users/web_profile_info` with the web app id the
browser sends. It returns the full profile and the recent posts as exact JSON.
From a residential session it answers; from a datacenter IP it is walled.
`ig profile`, `ig posts`, and `ig raw` ride this plane. When the firewall gates a
call, `ig` exits 4 with a clear message instead of pretending it found nothing.

## Exit codes

```
0  success, at least one record
1  error
2  usage error
3  no data (a valid empty result)
4  walled (the surface needs a residential session)
6  not found (the username or shortcode does not exist)
```

## Serve it

The same operations are available over HTTP and as an MCP tool set for agents,
with no extra code:

```bash
ig serve --addr :7777    # GET /v1/profile/<username> returns NDJSON
ig mcp                   # speak MCP over stdio
```

## Use it as a resource-URI driver

`ig` registers an `instagram` domain the way a program registers a database
driver with `database/sql`. A host enables it with one blank import:

```go
import _ "github.com/tamnd/instagram-cli/instagram"
```

Then [ant](https://github.com/tamnd/ant) (or any program that links the package)
dereferences `instagram://` URIs:

```bash
ant get instagram://profile/instagram        # the profile record
ant get instagram://post/DZf6PYtGyay          # one post
ant cat instagram://profile/instagram         # just the biography text
ant url instagram://post/DZf6PYtGyay          # the live https URL
```

## Development

```
cmd/ig/        thin main: hands cli.NewApp to kit.Run
cli/           assembles the kit App from the instagram domain
instagram/     the library: HTTP client, the two parsers, records, and the driver
pkg/igcode/    the shortcode <-> media id codec
docs/          tago documentation site
```

```bash
make build      # ./bin/ig
make test       # go test ./...
make vet        # go vet ./...
```

## Releasing

Push a version tag and GitHub Actions runs GoReleaser, which builds the archives,
Linux packages, the multi-arch GHCR image, checksums, SBOMs, and a cosign
signature:

```bash
git tag v0.1.0
git push --tags
```

The image tag carries no `v` prefix (`ghcr.io/tamnd/ig:0.1.0`). The Homebrew and
Scoop steps self-disable until their tokens exist, so the first release works
with no extra secrets.

## License

Apache-2.0. See [LICENSE](LICENSE).
