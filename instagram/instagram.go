// Package instagram is the library behind the ig command line: the HTTP client,
// the two reading planes, the typed records, and the parsers that fill them.
//
// Instagram serves the same public data through two channels that fail
// differently, and this library is built around the split:
//
//   - The SSR plane reads the Open Graph tags a logged-out post or reel page
//     ships. It needs no key and answers from anywhere, including a datacenter
//     IP. The like and comment counts there are rounded.
//   - The API plane calls /api/v1/users/web_profile_info with the web app id
//     header and returns the full profile plus the recent posts as exact JSON.
//     It answers from a residential session and is walled from a datacenter IP.
//
// Every record carries a `source` field ("api" or "ssr") so a reader always
// knows whether a count is exact or rounded.
package instagram

import (
	"context"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/tamnd/instagram-cli/pkg/igcode"
)

// DefaultUserAgent identifies the client to Instagram on the API plane. A real
// desktop string is what web_profile_info expects from a logged-out web client.
const DefaultUserAgent = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) " +
	"AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36"

// GooglebotUserAgent is sent on the SSR plane. A logged-out post or reel page
// serves its Open Graph tags to a crawler; with a desktop UA it serves the JS
// app shell instead, which carries no og tags. So the post/reel fetch identifies
// as Googlebot, which is honest about what it is.
const GooglebotUserAgent = "Mozilla/5.0 (compatible; Googlebot/2.1; " +
	"+http://www.google.com/bot.html)"

// appID is the public web app id the browser sends on every /api/v1 call. It is
// the one header that makes web_profile_info answer.
const appID = "936619743392459"

// Host is the site this client talks to and the host the URI driver claims.
const Host = "instagram.com"

// BaseURL is the root every request is built from. www is required; the apex
// redirects.
const BaseURL = "https://www." + Host

// ErrWalled means the firewall gated this surface from the caller's IP. The API
// plane answers from a residential session and returns this from a datacenter
// IP. It maps to exit 4 (need auth) in domain.go.
var ErrWalled = fmt.Errorf("walled: this surface needs a residential session")

// ErrNotFound means the username or shortcode does not exist.
var ErrNotFound = fmt.Errorf("not found")

// Client talks to Instagram over HTTP.
type Client struct {
	HTTP      *http.Client
	UserAgent string
	// Rate is the minimum gap between requests. Zero means no pacing.
	Rate    time.Duration
	Retries int

	last time.Time
}

// NewClient returns a Client with sensible defaults: a 30s timeout, a 600ms
// minimum gap between requests, and five retries on transient errors.
func NewClient() *Client {
	return &Client{
		HTTP:      &http.Client{Timeout: 30 * time.Second},
		UserAgent: DefaultUserAgent,
		Rate:      600 * time.Millisecond,
		Retries:   5,
	}
}

// get fetches url with the given extra headers and returns the response body
// and status. It paces and retries transient failures (429 and 5xx). A non-OK
// status that is not transient is returned as an error alongside the status, so
// the caller can tell a wall (302/401/403) from a hard error.
func (c *Client) get(ctx context.Context, url string, headers map[string]string) ([]byte, int, error) {
	var lastErr error
	for attempt := 0; attempt <= c.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, 0, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, status, retry, err := c.do(ctx, url, headers)
		if err == nil {
			return body, status, nil
		}
		lastErr = err
		if !retry {
			return nil, status, err
		}
	}
	return nil, 0, fmt.Errorf("get %s: %w", url, lastErr)
}

func (c *Client) do(ctx context.Context, url string, headers map[string]string) (body []byte, status int, retry bool, err error) {
	c.pace()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, false, err
	}
	req.Header.Set("User-Agent", c.UserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	// Do not follow redirects: a 302 to /accounts/login is a wall, and we want
	// to see it rather than fetch the login page.
	c.HTTP.CheckRedirect = func(*http.Request, []*http.Request) error {
		return http.ErrUseLastResponse
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, 0, true, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusTooManyRequests || resp.StatusCode >= 500 {
		return nil, resp.StatusCode, true, fmt.Errorf("http %d", resp.StatusCode)
	}
	if resp.StatusCode != http.StatusOK {
		// The status is the signal; no body needed.
		return nil, resp.StatusCode, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, true, err
	}
	return b, resp.StatusCode, false, nil
}

func (c *Client) pace() {
	if c.Rate <= 0 {
		return
	}
	if wait := c.Rate - time.Since(c.last); wait > 0 {
		time.Sleep(wait)
	}
	c.last = time.Now()
}

func backoff(attempt int) time.Duration {
	d := time.Duration(attempt) * 500 * time.Millisecond
	if d > 5*time.Second {
		d = 5 * time.Second
	}
	return d
}

// --- records ---

// Profile is a public account, read from the API plane (web_profile_info).
type Profile struct {
	Username    string `json:"username" kit:"id"`
	UserID      string `json:"user_id,omitempty"`
	FullName    string `json:"full_name,omitempty"`
	URL         string `json:"url"`
	Biography   string `json:"biography,omitempty" kit:"body"`
	ExternalURL string `json:"external_url,omitempty"`
	Followers   int64  `json:"follower_count"`
	Following   int64  `json:"following_count"`
	Posts       int64  `json:"post_count"`
	Verified    bool   `json:"is_verified"`
	Private     bool   `json:"is_private"`
	Business    bool   `json:"is_business,omitempty"`
	Category    string `json:"category,omitempty"`
	ProfilePic  string `json:"profile_pic_url,omitempty"`
	Source      string `json:"source"`
}

// Post is one piece of media. The API parser fills exact counts (source "api");
// the SSR parser fills the rounded counts Instagram prints on the page
// (source "ssr"). Source is always set so the precision is never ambiguous.
type Post struct {
	Shortcode  string `json:"shortcode" kit:"id"`
	URL        string `json:"url"`
	Type       string `json:"type"`
	Author     string `json:"author,omitempty"`
	AuthorID   string `json:"author_id,omitempty"`
	Caption    string `json:"caption,omitempty" kit:"body"`
	Likes      int64  `json:"like_count"`
	Comments   int64  `json:"comment_count"`
	Views      int64  `json:"view_count,omitempty"`
	TakenAt    int64  `json:"taken_at,omitempty"`
	Timestamp  string `json:"timestamp,omitempty"`
	DisplayURL string `json:"display_url,omitempty"`
	VideoURL   string `json:"video_url,omitempty"`
	Width      int    `json:"width,omitempty"`
	Height     int    `json:"height,omitempty"`
	Carousel   int    `json:"carousel_count,omitempty"`
	AltText    string `json:"alt_text,omitempty"`
	Source     string `json:"source"`
}

// Raw is the upstream web_profile_info JSON, pretty-printed, the source the
// profile and posts commands read from.
type Raw struct {
	Username string `json:"username" kit:"id"`
	URL      string `json:"url"`
	Body     string `json:"body" kit:"body"`
}

// --- url builders ---

// PostURL builds the canonical permalink for a shortcode. kind is "post" or
// "reel"; a reel gets the /reel/ segment.
func PostURL(shortcode, kind string) string {
	seg := "p"
	if kind == "reel" {
		seg = "reel"
	}
	return BaseURL + "/" + seg + "/" + shortcode + "/"
}

// ProfileURL builds the canonical profile permalink for a username.
func ProfileURL(username string) string {
	return BaseURL + "/" + username + "/"
}

// --- API plane ---

// webProfileInfo fetches the raw web_profile_info JSON for a username. A login
// redirect (302) or a 401/403 is the firewall, returned as ErrWalled; a 404 is
// ErrNotFound.
func (c *Client) webProfileInfo(ctx context.Context, username string) ([]byte, error) {
	url := BaseURL + "/api/v1/users/web_profile_info/?username=" + username
	body, status, err := c.get(ctx, url, map[string]string{
		"X-IG-App-ID":      appID,
		"X-Requested-With": "XMLHttpRequest",
		"Referer":          ProfileURL(username),
	})
	if err != nil {
		switch status {
		case http.StatusFound, http.StatusMovedPermanently, http.StatusUnauthorized, http.StatusForbidden:
			return nil, ErrWalled
		case http.StatusNotFound:
			return nil, ErrNotFound
		}
		return nil, err
	}
	return body, nil
}

// GetProfile reads a profile and the recent posts embedded alongside it.
func (c *Client) GetProfile(ctx context.Context, username string) (*Profile, []*Post, error) {
	body, err := c.webProfileInfo(ctx, username)
	if err != nil {
		return nil, nil, err
	}
	return parseProfile(body)
}

// GetRaw returns the upstream web_profile_info JSON for a username, pretty
// printed, as a Raw record.
func (c *Client) GetRaw(ctx context.Context, username string) (*Raw, error) {
	body, err := c.webProfileInfo(ctx, username)
	if err != nil {
		return nil, err
	}
	var pretty strings.Builder
	var v any
	if json.Unmarshal(body, &v) == nil {
		enc := json.NewEncoder(&pretty)
		enc.SetIndent("", "  ")
		_ = enc.Encode(v)
	} else {
		pretty.Write(body)
	}
	return &Raw{
		Username: username,
		URL:      BaseURL + "/api/v1/users/web_profile_info/?username=" + username,
		Body:     strings.TrimRight(pretty.String(), "\n"),
	}, nil
}

// --- SSR plane ---

// GetPost reads one post or reel from its page. It rides the SSR plane and
// answers from anywhere. kind is "post" or "reel".
func (c *Client) GetPost(ctx context.Context, shortcode, kind string) (*Post, error) {
	url := PostURL(shortcode, kind)
	// Identify as Googlebot so the page serves its og tags. If the caller set a
	// custom User-Agent, honor it instead.
	ua := GooglebotUserAgent
	if c.UserAgent != DefaultUserAgent {
		ua = c.UserAgent
	}
	body, status, err := c.get(ctx, url, map[string]string{"User-Agent": ua})
	if err != nil {
		switch status {
		case http.StatusFound, http.StatusMovedPermanently:
			return nil, ErrWalled
		case http.StatusNotFound:
			return nil, ErrNotFound
		}
		return nil, err
	}
	return parsePostPage(body, shortcode, kind)
}

// --- parsers ---

// wpiEnvelope is the shape of web_profile_info we read. Unread fields are left
// out so the parser stays small and the test fixture can be trimmed.
type wpiEnvelope struct {
	Data struct {
		User *wpiUser `json:"user"`
	} `json:"data"`
}

type wpiCount struct {
	Count int64 `json:"count"`
}

type wpiUser struct {
	ID                string   `json:"id"`
	Username          string   `json:"username"`
	FullName          string   `json:"full_name"`
	Biography         string   `json:"biography"`
	ExternalURL       string   `json:"external_url"`
	IsVerified        bool     `json:"is_verified"`
	IsPrivate         bool     `json:"is_private"`
	IsBusinessAccount bool     `json:"is_business_account"`
	CategoryName      string   `json:"category_name"`
	ProfilePicHD      string   `json:"profile_pic_url_hd"`
	ProfilePic        string   `json:"profile_pic_url"`
	FollowedBy        wpiCount `json:"edge_followed_by"`
	Follow            wpiCount `json:"edge_follow"`
	Timeline          struct {
		Count int64 `json:"count"`
		Edges []struct {
			Node wpiNode `json:"node"`
		} `json:"edges"`
	} `json:"edge_owner_to_timeline_media"`
}

type wpiNode struct {
	Typename   string `json:"__typename"`
	ID         string `json:"id"`
	Shortcode  string `json:"shortcode"`
	IsVideo    bool   `json:"is_video"`
	DisplayURL string `json:"display_url"`
	VideoURL   string `json:"video_url"`
	Dimensions struct {
		Width  int `json:"width"`
		Height int `json:"height"`
	} `json:"dimensions"`
	AccessibilityCaption string `json:"accessibility_caption"`
	TakenAt              int64  `json:"taken_at_timestamp"`
	Owner                struct {
		ID       string `json:"id"`
		Username string `json:"username"`
	} `json:"owner"`
	Caption struct {
		Edges []struct {
			Node struct {
				Text string `json:"text"`
			} `json:"node"`
		} `json:"edges"`
	} `json:"edge_media_to_caption"`
	Comments    wpiCount `json:"edge_media_to_comment"`
	LikedBy     wpiCount `json:"edge_liked_by"`
	PreviewLike wpiCount `json:"edge_media_preview_like"`
	VideoViews  int64    `json:"video_view_count"`
	Sidecar     struct {
		Edges []json.RawMessage `json:"edges"`
	} `json:"edge_sidecar_to_children"`
}

// parseProfile reads web_profile_info into a Profile and the recent posts.
func parseProfile(body []byte) (*Profile, []*Post, error) {
	var env wpiEnvelope
	if err := json.Unmarshal(body, &env); err != nil {
		return nil, nil, fmt.Errorf("decode web_profile_info: %w", err)
	}
	u := env.Data.User
	if u == nil || u.Username == "" {
		return nil, nil, ErrNotFound
	}
	pic := u.ProfilePicHD
	if pic == "" {
		pic = u.ProfilePic
	}
	p := &Profile{
		Username:    u.Username,
		UserID:      u.ID,
		FullName:    u.FullName,
		URL:         ProfileURL(u.Username),
		Biography:   u.Biography,
		ExternalURL: u.ExternalURL,
		Followers:   u.FollowedBy.Count,
		Following:   u.Follow.Count,
		Posts:       u.Timeline.Count,
		Verified:    u.IsVerified,
		Private:     u.IsPrivate,
		Business:    u.IsBusinessAccount,
		Category:    u.CategoryName,
		ProfilePic:  pic,
		Source:      "api",
	}
	var posts []*Post
	for i := range u.Timeline.Edges {
		posts = append(posts, parsePostNode(&u.Timeline.Edges[i].Node))
	}
	return p, posts, nil
}

// parsePostNode turns one timeline node into a Post with exact counts.
func parsePostNode(n *wpiNode) *Post {
	likes := n.LikedBy.Count
	if likes == 0 {
		likes = n.PreviewLike.Count
	}
	var caption string
	if len(n.Caption.Edges) > 0 {
		caption = n.Caption.Edges[0].Node.Text
	}
	post := &Post{
		Shortcode:  n.Shortcode,
		URL:        PostURL(n.Shortcode, "post"),
		Type:       nodeType(n),
		Author:     n.Owner.Username,
		AuthorID:   n.Owner.ID,
		Caption:    caption,
		Likes:      likes,
		Comments:   n.Comments.Count,
		Views:      n.VideoViews,
		TakenAt:    n.TakenAt,
		DisplayURL: n.DisplayURL,
		VideoURL:   n.VideoURL,
		Width:      n.Dimensions.Width,
		Height:     n.Dimensions.Height,
		Carousel:   len(n.Sidecar.Edges),
		AltText:    n.AccessibilityCaption,
		Source:     "api",
	}
	if n.TakenAt > 0 {
		post.Timestamp = time.Unix(n.TakenAt, 0).UTC().Format(time.RFC3339)
	}
	return post
}

func nodeType(n *wpiNode) string {
	switch n.Typename {
	case "GraphSidecar", "XDTGraphSidecar":
		return "carousel"
	case "GraphVideo", "XDTGraphVideo":
		return "video"
	default:
		if n.IsVideo {
			return "video"
		}
		return "image"
	}
}

var (
	ogRE      = regexp.MustCompile(`<meta property="og:(\w+)" content="([^"]*)"`)
	ogDescRE  = regexp.MustCompile(`(?s)^([\d.,]+[KMB]?)\s+likes?,\s*([\d.,]+[KMB]?)\s+comments?\s*-\s*(\S+)\s+on\s+([^:]+):\s*(.*)$`)
	loginWall = regexp.MustCompile(`(?i)/accounts/login|loginForm|"viewerId":null,"challenge"`)
)

// parsePostPage reads a post or reel page's Open Graph tags into a Post. The
// counts here are the rounded values Instagram prints (source "ssr").
func parsePostPage(htmlBody []byte, shortcode, kind string) (*Post, error) {
	og := map[string]string{}
	for _, m := range ogRE.FindAllSubmatch(htmlBody, -1) {
		og[string(m[1])] = html.UnescapeString(string(m[2]))
	}
	// A logged-out post page always carries og:description. Its absence is a
	// login wall or a page that does not exist.
	if og["description"] == "" {
		if loginWall.Match(htmlBody) {
			return nil, ErrWalled
		}
		return nil, ErrNotFound
	}

	kindLabel := "post"
	if kind == "reel" {
		kindLabel = "reel"
	}
	post := &Post{
		Shortcode:  shortcode,
		URL:        PostURL(shortcode, kind),
		Type:       kindLabel,
		DisplayURL: og["image"],
		Source:     "ssr",
	}
	// og:url names the author handle: .../{author}/p/{shortcode}/
	if a := authorFromOGURL(og["url"]); a != "" {
		post.Author = a
	}
	// og:description: "585K likes, 43K comments - author on June 12, 2026: \"caption\""
	if m := ogDescRE.FindStringSubmatch(og["description"]); m != nil {
		post.Likes = parseCount(m[1])
		post.Comments = parseCount(m[2])
		if post.Author == "" {
			post.Author = strings.TrimSpace(m[3])
		}
		if ts := parseOGDate(strings.TrimSpace(m[4])); ts > 0 {
			post.TakenAt = ts
			post.Timestamp = time.Unix(ts, 0).UTC().Format(time.RFC3339)
		}
		post.Caption = cleanCaption(m[5])
	}
	if post.Caption == "" {
		post.Caption = cleanCaption(captionFromOGTitle(og["title"]))
	}
	return post, nil
}

// authorFromOGURL pulls the handle out of an og:url like
// https://www.instagram.com/instagram/p/DZf6PYtGyay/
func authorFromOGURL(u string) string {
	u = strings.TrimPrefix(u, BaseURL+"/")
	u = strings.TrimPrefix(u, "https://www.instagram.com/")
	u = strings.TrimPrefix(u, "https://instagram.com/")
	seg := strings.SplitN(u, "/", 2)
	if len(seg) > 0 {
		s := seg[0]
		if s != "p" && s != "reel" && s != "" {
			return s
		}
	}
	return ""
}

// captionFromOGTitle strips the "Author on Instagram: " prefix an og:title
// carries and returns the caption portion.
func captionFromOGTitle(title string) string {
	if i := strings.Index(title, ": "); i >= 0 {
		return title[i+2:]
	}
	return ""
}

// cleanCaption trims the surrounding quotes Instagram wraps a caption in and
// the whitespace around it.
func cleanCaption(s string) string {
	s = strings.TrimSpace(s)
	s = strings.Trim(s, "“”\"")
	return strings.TrimSpace(s)
}

// parseCount turns "585K", "43K", "1.2M", or "1,234" into an integer. The
// abbreviated forms are approximate, which is the documented price of the SSR
// plane.
func parseCount(s string) int64 {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0
	}
	mult := 1.0
	switch s[len(s)-1] {
	case 'K', 'k':
		mult, s = 1e3, s[:len(s)-1]
	case 'M', 'm':
		mult, s = 1e6, s[:len(s)-1]
	case 'B', 'b':
		mult, s = 1e9, s[:len(s)-1]
	}
	s = strings.ReplaceAll(s, ",", "")
	f, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return 0
	}
	return int64(f * mult)
}

// ogDateLayouts are the date shapes Instagram prints in og:description.
var ogDateLayouts = []string{"January 2, 2006", "January 2 2006", "Jan 2, 2006"}

func parseOGDate(s string) int64 {
	for _, layout := range ogDateLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Unix()
		}
	}
	return 0
}

// CanonicalShortcode validates and returns a shortcode, accepting a bare code
// or a numeric media id (which it encodes). It is the network-free half of the
// post/reel resolver.
func CanonicalShortcode(ref string) (string, bool) {
	ref = strings.TrimSpace(ref)
	if ref == "" {
		return "", false
	}
	if id, err := strconv.ParseUint(ref, 10, 64); err == nil {
		return igcode.IDToShortcode(id), true
	}
	if igcode.IsShortcode(ref) {
		return ref, true
	}
	return "", false
}
