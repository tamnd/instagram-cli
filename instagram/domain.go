package instagram

import (
	"context"
	"errors"
	"net/url"
	"strings"

	"github.com/tamnd/any-cli/kit"
	"github.com/tamnd/any-cli/kit/errs"
)

// domain.go exposes instagram as a kit Domain: a driver a multi-domain host
// (ant) enables with a single blank import,
//
//	import _ "github.com/tamnd/instagram-cli/instagram"
//
// exactly as a database/sql program enables a driver. The init below registers
// it; the host then dereferences instagram:// URIs by routing to the operations
// Register installs. The same Domain also builds the standalone ig binary (see
// cli.NewApp), so the binary and a host share one source of truth.
func init() { kit.Register(Domain{}) }

// Domain is the instagram driver. It carries no state; the per-run client is
// built by the factory Register hands kit.
type Domain struct{}

// reserved first-path segments that are never a username.
var reserved = map[string]bool{
	"p": true, "reel": true, "reels": true, "tv": true, "explore": true,
	"stories": true, "accounts": true, "directory": true, "about": true,
	"developer": true, "legal": true, "api": true, "web": true,
	"graphql": true, "challenge": true, "direct": true,
}

// Info describes the scheme, the hostnames a pasted link is matched against, and
// the identity reused for the binary's help and version.
func (Domain) Info() kit.DomainInfo {
	return kit.DomainInfo{
		Scheme: "instagram",
		Hosts:  []string{Host, "www." + Host},
		Identity: kit.Identity{
			Binary: "ig",
			Short:  "A command line for Instagram.",
			Long: `A command line for Instagram.

ig reads public Instagram data over plain HTTPS, shapes it into clean records,
and prints output that pipes into the rest of your tools. No API key, no login,
nothing to run alongside it.

It reads two surfaces. A post or reel page is read from the Open Graph tags it
ships to a logged-out client, so ig post and ig reel answer from anywhere. A
profile and its recent posts come from the web_profile_info endpoint, which
answers from a residential connection and is walled from a datacenter IP; when
it is walled, ig says so and exits 4 instead of pretending it found nothing.`,
			Site: Host,
			Repo: "https://github.com/tamnd/instagram-cli",
		},
	}
}

// Register installs the client factory and every operation onto app.
func (Domain) Register(app *kit.App) {
	app.SetClient(newClient)

	kit.Handle(app, kit.OpMeta{Name: "profile", Group: "read", Single: true,
		Summary: "Fetch a profile by username or URL", URIType: "profile", Resolver: true,
		Args: []kit.Arg{{Name: "ref", Help: "username or profile URL"}}}, getProfile)

	kit.Handle(app, kit.OpMeta{Name: "posts", Group: "read", List: true,
		Summary: "List the recent posts on a profile", URIType: "post",
		Args: []kit.Arg{{Name: "ref", Help: "username or profile URL"}}}, listPosts)

	kit.Handle(app, kit.OpMeta{Name: "post", Group: "read", Single: true,
		Summary: "Fetch one post by shortcode or URL", URIType: "post", Resolver: true,
		Args: []kit.Arg{{Name: "ref", Help: "shortcode or post URL"}}}, getPost)

	kit.Handle(app, kit.OpMeta{Name: "reel", Group: "read", Single: true,
		Summary: "Fetch one reel by shortcode or URL", URIType: "reel", Resolver: true,
		Args: []kit.Arg{{Name: "ref", Help: "shortcode or reel URL"}}}, getReel)

	kit.Handle(app, kit.OpMeta{Name: "raw", Group: "read", Single: true,
		Summary: "Print a profile's upstream web_profile_info JSON", URIType: "profile",
		Args: []kit.Arg{{Name: "ref", Help: "username or profile URL"}}}, getRaw)
}

// newClient builds the client from the host-resolved config.
func newClient(_ context.Context, cfg kit.Config) (any, error) {
	c := NewClient()
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
		c.HTTP.Timeout = cfg.Timeout
	}
	return c, nil
}

// --- inputs ---

type profileRef struct {
	Ref    string  `kit:"arg" help:"username or profile URL"`
	Client *Client `kit:"inject"`
}

type postRef struct {
	Ref    string  `kit:"arg" help:"shortcode or post URL"`
	Client *Client `kit:"inject"`
}

// --- handlers ---

func getProfile(ctx context.Context, in profileRef, emit func(*Profile) error) error {
	username, err := usernameOf(in.Ref)
	if err != nil {
		return err
	}
	p, _, err := in.Client.GetProfile(ctx, username)
	if err != nil {
		return mapErr(err)
	}
	return emit(p)
}

func listPosts(ctx context.Context, in profileRef, emit func(*Post) error) error {
	username, err := usernameOf(in.Ref)
	if err != nil {
		return err
	}
	_, posts, err := in.Client.GetProfile(ctx, username)
	if err != nil {
		return mapErr(err)
	}
	for _, p := range posts {
		if err := emit(p); err != nil {
			return err
		}
	}
	return nil
}

func getPost(ctx context.Context, in postRef, emit func(*Post) error) error {
	return fetchPost(ctx, in, "post", emit)
}

func getReel(ctx context.Context, in postRef, emit func(*Post) error) error {
	return fetchPost(ctx, in, "reel", emit)
}

func fetchPost(ctx context.Context, in postRef, kind string, emit func(*Post) error) error {
	shortcode, k, err := shortcodeOf(in.Ref, kind)
	if err != nil {
		return err
	}
	p, err := in.Client.GetPost(ctx, shortcode, k)
	if err != nil {
		return mapErr(err)
	}
	return emit(p)
}

func getRaw(ctx context.Context, in profileRef, emit func(*Raw) error) error {
	username, err := usernameOf(in.Ref)
	if err != nil {
		return err
	}
	r, err := in.Client.GetRaw(ctx, username)
	if err != nil {
		return mapErr(err)
	}
	return emit(r)
}

// --- Resolver: URI-native string functions, pure and network-free ---

// Classify turns any accepted input into a canonical (type, id). A bare token is
// read as a username, the common case; /p/ and /reel/ links classify to a post
// or reel.
func (Domain) Classify(input string) (uriType, id string, err error) {
	t, id := classify(input)
	if t == "" {
		return "", "", errs.Usage("unrecognized instagram reference: %q", input)
	}
	return t, id, nil
}

// Locate is the inverse: the live https URL for a (type, id).
func (Domain) Locate(uriType, id string) (string, error) {
	switch uriType {
	case "profile":
		return ProfileURL(id), nil
	case "post":
		return PostURL(id, "post"), nil
	case "reel":
		return PostURL(id, "reel"), nil
	default:
		return "", errs.Usage("instagram has no resource type %q", uriType)
	}
}

// --- helpers ---

// classify parses any accepted input into (type, id) with no network call.
func classify(input string) (uriType, id string) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", ""
	}
	// A full URL on this host: read the path segments.
	if u, err := url.Parse(input); err == nil && (u.Scheme == "http" || u.Scheme == "https") {
		segs := pathSegs(u.Path)
		switch {
		case len(segs) >= 2 && segs[0] == "p":
			return "post", segs[1]
		case len(segs) >= 2 && (segs[0] == "reel" || segs[0] == "reels"):
			return "reel", segs[1]
		// .../{author}/p/{code}/ or .../{author}/reel/{code}/
		case len(segs) >= 3 && segs[1] == "p":
			return "post", segs[2]
		case len(segs) >= 3 && (segs[1] == "reel" || segs[1] == "reels"):
			return "reel", segs[2]
		case len(segs) >= 1 && !reserved[segs[0]]:
			return "profile", segs[0]
		}
		return "", ""
	}
	// A bare path like p/CODE or reel/CODE.
	segs := pathSegs(input)
	switch {
	case len(segs) >= 2 && segs[0] == "p":
		return "post", segs[1]
	case len(segs) >= 2 && (segs[0] == "reel" || segs[0] == "reels"):
		return "reel", segs[1]
	case len(segs) == 1 && !reserved[segs[0]]:
		return "profile", strings.TrimPrefix(segs[0], "@")
	}
	return "", ""
}

func pathSegs(p string) []string {
	var out []string
	for _, s := range strings.Split(p, "/") {
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

// usernameOf resolves a ref to a username, accepting a bare handle (with or
// without @) or a profile URL.
func usernameOf(ref string) (string, error) {
	t, id := classify(ref)
	if t == "profile" {
		return id, nil
	}
	// A pasted post/reel link does not name a profile we can fetch directly.
	return "", errs.Usage("expected a username or profile URL, got %q", ref)
}

// shortcodeOf resolves a ref to a (shortcode, kind), accepting a bare shortcode,
// a numeric media id, or a post/reel URL. defaultKind is the command's own kind
// ("post" or "reel"); a URL that names the other kind wins over it.
func shortcodeOf(ref, defaultKind string) (string, string, error) {
	t, id := classify(ref)
	switch t {
	case "post":
		return id, "post", nil
	case "reel":
		return id, "reel", nil
	}
	// Not a URL: accept a bare shortcode or numeric id.
	if sc, ok := CanonicalShortcode(ref); ok {
		return sc, defaultKind, nil
	}
	return "", "", errs.Usage("expected a shortcode or post URL, got %q", ref)
}

// mapErr converts a library error into the kit error kind that carries the
// right exit code, so a host renders the same outcomes the binary does.
func mapErr(err error) error {
	switch {
	case errors.Is(err, ErrWalled):
		return errs.NeedAuth("%s", err.Error())
	case errors.Is(err, ErrNotFound):
		return errs.NotFound("%s", err.Error())
	default:
		return err
	}
}
