package instagram

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes instagram as a kit Domain: a driver that a multi-domain
// host (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/instagram-cli/instagram"
//
// exactly as a database/sql program enables a driver with `import _
// "github.com/lib/pq"`. The init below registers it; the host then dereferences
// instagram:// URIs by routing to the operations Register installs. The same
// Domain also builds the standalone ig binary (see cli.NewApp), so the
// binary and a host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the Instagram driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// Info describes the scheme, the hostnames a pasted link is matched against, and
// the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "instagram",
		Hosts:  []string{Host, "instagram.com"},
		Identity: kit.Identity{
			Binary: "ig",
			Short:  "Read public Instagram profiles and posts",
			Long: `ig reads public Instagram data over plain HTTPS the way a logged-out
browser does. It fetches profiles and recent posts via Instagram's
web_profile_info API, falling back to og: meta tag parsing on the
HTML page when the API is unavailable.

Instagram actively blocks requests from datacenter IP addresses. On most
cloud or CI machines every command exits with code 5 (blocked). Running
ig from a residential network usually succeeds for public profiles.

ig is an independent tool and is not affiliated with, endorsed by, or
sponsored by Instagram or Meta Platforms.`,
			Site: "https://www.instagram.com",
			Repo: "https://github.com/tamnd/instagram-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{
		Name:     "profile",
		Group:    "read",
		Single:   true,
		Resolver: true,
		URIType:  "profile",
		Summary:  "Fetch a public user profile",
		Args:     []kit.Arg{{Name: "username", Help: "Instagram username (without @)"}},
	}, getProfile)

	kit.Handle(app, kit.OpMeta{
		Name:    "posts",
		Group:   "read",
		URIType: "post",
		Summary: "List a user's recent public posts",
		Args:    []kit.Arg{{Name: "username", Help: "Instagram username (without @)"}},
	}, listPosts)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := DefaultConfig()
	if cfg.UserAgent != "" {
		c.UserAgent = cfg.UserAgent
	}
	if cfg.Rate > 0 {
		c.Rate = cfg.Rate
	}
	if cfg.Retries > 0 {
		c.Retries = cfg.Retries
	}
	if cfg.Timeout > 0 {
		c.Timeout = cfg.Timeout
	}
	return NewClient(c), nil
}

// --- input structs ---

type profileIn struct {
	Username string  `kit:"arg" help:"Instagram username (without @)"`
	Client   *Client `kit:"inject"`
}

type postsIn struct {
	Username string  `kit:"arg" help:"Instagram username (without @)"`
	Limit    int     `kit:"flag,inherit" help:"max posts" default:"12"`
	Client   *Client `kit:"inject"`
}

// --- handlers ---

func getProfile(ctx context.Context, in profileIn, emit func(*Profile) error) error {
	p, err := in.Client.FetchProfile(ctx, in.Username)
	if err != nil {
		return mapErr(err)
	}
	return emit(p)
}

func listPosts(ctx context.Context, in postsIn, emit func(*Post) error) error {
	limit := in.Limit
	if limit <= 0 {
		limit = 12
	}
	posts, err := in.Client.FetchPosts(ctx, in.Username, limit)
	if err != nil {
		return mapErr(err)
	}
	for i := range posts {
		if err := emit(&posts[i]); err != nil {
			return err
		}
	}
	return nil
}

// --- Resolver ---

// Classify turns any accepted input into the canonical (type, id).
// Accepts a bare username, an @username, an /username/ path, or a full URL.
func (Domain) Classify(input string) (uriType, id string, err error) {
	id = normalizeUsername(input)
	if id == "" {
		return "", "", errs.Usage("unrecognized instagram reference: %q", input)
	}
	return "profile", id, nil
}

// Locate returns the canonical URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "profile":
		return BaseURL + "/" + strings.Trim(id, "/") + "/", nil
	case "post":
		return BaseURL + "/p/" + strings.Trim(id, "/") + "/", nil
	default:
		return "", errs.Usage("instagram has no resource type %q", uriType)
	}
}

// --- helpers ---

// normalizeUsername returns a canonical username from any accepted input.
func normalizeUsername(input string) string {
	input = strings.TrimSpace(input)
	// Full URL: https://www.instagram.com/username/ → username
	if u, err := url.Parse(input); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		seg := strings.Trim(u.Path, "/")
		// Strip path sub-segments like /p/ or /reel/ for non-profile URLs.
		if i := strings.Index(seg, "/"); i >= 0 {
			seg = seg[:i]
		}
		return seg
	}
	// @handle or bare username or /username/ path.
	input = strings.TrimPrefix(input, "@")
	input = strings.Trim(input, "/")
	return input
}

// mapErr converts library errors into kit-typed errors that carry the right
// exit code.
func mapErr(err error) error {
	if err == nil {
		return nil
	}
	switch {
	case errors.Is(err, ErrNotFound):
		return errs.NotFound("%s", err.Error())
	case errors.Is(err, ErrBlocked):
		return errs.NeedAuth("%s", err.Error())
	default:
		return err
	}
}
