---
title: "Troubleshooting"
description: "The handful of things that trip people up, and how to fix each one."
weight: 40
---

Most of these come down to network reality or how Instagram serves its data.

## profile, posts, or raw say "Walled" (exit 4)

`ig profile`, `ig posts`, and `ig raw` call the `web_profile_info` endpoint,
which answers from a residential connection and is walled from a datacenter IP.
If you run them from a cloud box, a CI runner, or a VPS, you get exit 4 and a
clear message:

```
Walled: this surface needs a residential session.
```

That is the firewall gating the call, so `ig` reports it plainly and never
fakes an empty result. Run those commands from a residential connection. `ig post`
and `ig reel` read the post page directly and answer from anywhere, including
the same datacenter box, so you can still pull individual posts and reels there.

## A username or shortcode is not found (exit 6)

A bad username or a shortcode that does not exist gets exit 6, separate from the
walled exit 4. Exit 6 means Instagram answered and the thing is not there. Exit 4
means the surface itself was blocked before any lookup. Check the spelling the
way Instagram uses it before assuming the account or post is gone.

## Requests start failing or returning 429

Instagram rate-limits like any public site. `ig` already paces requests and
retries the transient failures, but a hard limit still means backing off. Raise
`--retries`, lower how fast you loop over many records, and retry later. A burst
of 429 or 5xx responses is the site asking you to slow down.

## The data you want is not a command

v0.1.0 reads profiles, their recent posts, and individual posts and reels.
These surfaces are out of scope today, gated even from a residential session:

- hashtag pages
- search
- comments
- deep post paging past the roughly twelve posts the profile document returns

`ig posts` returns what the profile endpoint ships in one document, so it stops
at about twelve. There is no paging past that yet.

## The binary is not on your PATH

`go install` puts the binary in `$(go env GOPATH)/bin` (usually `~/go/bin`), and
a release archive leaves it wherever you unpacked it. If your shell cannot find
`ig`, add that directory to your `PATH`. See
[installation](/getting-started/installation/).
