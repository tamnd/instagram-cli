# ig

A command line for Instagram.

`ig` is a single pure-Go binary. It reads public instagram data
over plain HTTPS, shapes it into clean records, and prints output that pipes
into the rest of your tools. No API key, nothing to run alongside it.

The same package is also a [resource-URI driver](#use-it-as-a-resource-uri-driver),
so a host program like [ant](https://github.com/tamnd/ant) can address
instagram as `instagram://` URIs.

## Install

```bash
go install github.com/tamnd/instagram-cli/cmd/ig@latest
```

Or grab a prebuilt binary from the [releases](https://github.com/tamnd/instagram-cli/releases), or run
the container image:

```bash
docker run --rm ghcr.io/tamnd/ig:latest --help
```

## Usage

```bash
ig page <path>                      # fetch one page as a record
ig page <path> -o json              # as JSON, ready for jq
ig page <path> --template '{{.Body}}'  # just the readable body text
ig links <path>                     # the pages it links to, one per line
ig --help                           # the whole command tree
```

Every command shares one output contract: `-o table|json|jsonl|csv|tsv|url|raw`,
`--fields` to pick columns, `--template` for a custom line, and `-n` to limit.
The default adapts to where output goes (a table on a terminal, JSONL in a
pipe), so the same command reads well by hand and parses cleanly downstream.

This is a fresh scaffold. It ships one example resource type, `page`, wired end
to end. Model the real instagram records in `instagram/` and declare their
operations in `instagram/domain.go`; each one becomes a command, an HTTP
route, and an MCP tool at once.

## Serve it

The same operations are available over HTTP and as an MCP tool set for agents,
with no extra code:

```bash
ig serve --addr :7777    # GET /v1/page/<path>  returns NDJSON
ig mcp                   # speak MCP over stdio
```

## Use it as a resource-URI driver

`ig` registers a `instagram` domain the way a program registers a
database driver with `database/sql`. A host enables it with one blank import:

```go
import _ "github.com/tamnd/instagram-cli/instagram"
```

Then [ant](https://github.com/tamnd/ant) (or any program that links the package)
dereferences `instagram://` URIs without knowing anything about instagram:

```bash
ant get instagram://page/<path>   # fetch the record
ant cat instagram://page/<path>   # just the body text
ant ls  instagram://page/<path>   # the pages it links to, each addressable
ant url instagram://page/<path>   # the live https URL
```

## Development

```
cmd/ig/   thin main: hands cli.NewApp to kit.Run
cli/                 assembles the kit App from the instagram domain
instagram/                the library: HTTP client, data models, and domain.go (the driver)
docs/                tago documentation site
```

```bash
make build      # ./bin/ig
make test       # go test ./...
make vet        # go vet ./...
```

## Releasing

Push a version tag and GitHub Actions runs GoReleaser, which builds the
archives, Linux packages, the multi-arch GHCR image, checksums, SBOMs, and a
cosign signature:

```bash
git tag v0.1.0
git push --tags
```

The Homebrew and Scoop steps self-disable until their tokens exist, so the first
release works with no extra secrets.

## License

Apache-2.0. See [LICENSE](LICENSE).
