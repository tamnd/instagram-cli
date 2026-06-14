---
title: "Add a command"
description: "Model a real instagram record and expose it as a command, a route, and a tool at once."
weight: 10
---

v0.1.0 ships `profile`, `posts`, `post`, `reel`, and `raw`. Adding another
operation, or another record type, means modelling it in two files, and every
surface updates itself. The walkthrough below uses a small generic `item` to
show the wiring; the real records live in `instagram/instagram.go`.

## 1. Model the record

In `instagram/instagram.go`, add a struct for the thing you are fetching
and a client method that returns it. The `kit` struct tags decide how a host
addresses the record:

```go
type Item struct {
    ID    string `json:"id"    kit:"id"`              // the URI id
    Title string `json:"title"`
    Body  string `json:"body"  kit:"body"`            // what cat and Markdown print
    Owner string `json:"owner" kit:"link,kind=instagram/user"` // an edge to another record
}

func (c *Client) GetItem(ctx context.Context, id string) (*Item, error) {
    body, err := c.Get(ctx, BaseURL+"/items/"+id+".json")
    if err != nil {
        return nil, err
    }
    // decode body into an Item ...
    return item, nil
}
```

- `kit:"id"` marks the field that becomes the URI id.
- `kit:"body"` marks the prose that `cat` and the Markdown export render.
- `kit:"link,kind=<scheme>/<type>"` marks an outbound edge. It can point at
  another instagram type or at another site entirely, which is what lets a
  host walk the graph across tools.

## 2. Declare the operation

In `instagram/domain.go`, add an input struct and a handler, then register
it in `Register`:

```go
type itemRef struct {
    Ref    string  `kit:"arg" help:"item id or URL"`
    Client *Client `kit:"inject"`
}

func getItem(ctx context.Context, in itemRef, emit func(*Item) error) error {
    it, err := in.Client.GetItem(ctx, in.Ref)
    if err != nil {
        return mapErr(err)
    }
    return emit(it)
}

// inside Register(app):
kit.Handle(app, kit.OpMeta{Name: "item", Group: "read", Single: true,
    Summary: "Fetch an item by id or URL", URIType: "item", Resolver: true,
    Args: []kit.Arg{{Name: "ref", Help: "item id or URL"}}}, getItem)
```

That is the whole change. `kit.Handle` reflects the input for flags and the
output for the record shape, so the operation immediately becomes:

```bash
ig item <id>                 # the command
curl 'localhost:7777/v1/item/<id>'      # the route, under serve
ant get instagram://item/<id>       # the URI dereference, via a host
```

## Resolver ops and list ops

Two flags shape how a host treats an operation:

- **`Single: true`** with **`Resolver: true`** marks the canonical one-record
  fetch for a `URIType`. It answers `ant get`.
- **`List: true`** marks a member-lister for a parent resource. It answers
  `ant ls`. A list op should emit records that are themselves addressable
  (often a lightweight stub of a resolver type), so every member is a URI a host
  can follow. The `posts` op does this: it lists a profile's recent posts, and
  each one is an addressable `instagram://post/` record.

## Map errors to exit codes

Return the `errs` kinds from `mapErr` so every surface reports the same outcome
with the same exit code:

```go
case errors.Is(err, ErrNotFound):
    return errs.NotFound("%s", err.Error())
case errors.Is(err, ErrRateLimited):
    return errs.RateLimited("%s", err.Error())
```

See [output formats](/reference/output/) for how records render, and
[resource URIs](/guides/resource-uris/) for how a host addresses them.
