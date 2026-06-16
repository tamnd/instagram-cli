package instagram

import (
	"encoding/json"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// isBlocked reports whether a raw body is one of Instagram's refusal signals:
// a JSON body carrying "login_required" or "checkpoint_url".
func isBlocked(body []byte) bool {
	s := string(body)
	return strings.Contains(s, `"login_required"`) ||
		strings.Contains(s, `"checkpoint_url"`) ||
		strings.Contains(s, `"require_login"`)
}

// parseProfile decodes the web_profile_info API response into a Profile.
// It returns ErrBlocked when the body is a refusal message and ErrNotFound
// when the user object is absent.
func parseProfile(body []byte) (*Profile, error) {
	if isBlocked(body) {
		return nil, ErrBlocked
	}
	var resp rawResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if resp.Message == "login_required" {
		return nil, ErrBlocked
	}
	u := resp.Data.User
	if u == nil {
		return nil, ErrNotFound
	}
	return profileFromRaw(u), nil
}

// profileFromRaw maps a raw API user object to a Profile.
func profileFromRaw(u *rawUser) *Profile {
	return &Profile{
		Username:   u.Username,
		FullName:   u.FullName,
		Bio:        strings.TrimSpace(u.Biography),
		Website:    u.ExternalURL,
		Followers:  u.EdgeFollowedBy.Count,
		Following:  u.EdgeFollow.Count,
		Posts:      u.EdgeMedia.Count,
		IsPrivate:  u.IsPrivate,
		IsVerified: u.IsVerified,
		ProfilePic: u.ProfilePicURL,
		URL:        BaseURL + "/" + u.Username + "/",
		FetchedAt:  time.Now(),
	}
}

// parsePosts decodes recent posts from the web_profile_info API response.
// It returns ErrBlocked when the body is a refusal and returns up to limit
// posts (0 = no limit).
func parsePosts(body []byte, limit int) ([]Post, error) {
	if isBlocked(body) {
		return nil, ErrBlocked
	}
	var resp rawResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	if resp.Message == "login_required" {
		return nil, ErrBlocked
	}
	u := resp.Data.User
	if u == nil {
		return nil, ErrNotFound
	}
	edges := u.EdgeMedia.Edges
	if limit > 0 && limit < len(edges) {
		edges = edges[:limit]
	}
	out := make([]Post, 0, len(edges))
	now := time.Now()
	for _, e := range edges {
		n := e.Node
		if n.ID == "" {
			continue
		}
		p := Post{
			ID:        n.ID,
			ShortCode: n.ShortCode,
			URL:       BaseURL + "/p/" + n.ShortCode + "/",
			Type:      n.TypeName,
			Thumbnail: bestImage(n),
			Likes:     n.LikeCount.Count,
			Comments:  n.CommentCount.Count,
			IsVideo:   n.IsVideo,
			VideoURL:  n.VideoURL,
			FetchedAt: now,
		}
		if len(n.Caption.Edges) > 0 {
			p.Caption = strings.TrimSpace(n.Caption.Edges[0].Node.Text)
		}
		if n.TakenAt > 0 {
			p.Timestamp = time.Unix(n.TakenAt, 0).UTC().Format(time.RFC3339)
		}
		out = append(out, p)
	}
	return out, nil
}

// bestImage returns the best available image URL from a media node.
func bestImage(n rawMediaNode) string {
	if n.DisplayURL != "" {
		return n.DisplayURL
	}
	return n.ThumbnailSrc
}

// --- HTML meta tag fallback ---

var (
	reOGTitle  = regexp.MustCompile(`<meta\s+property="og:title"\s+content="([^"]*)"`)
	reOGDesc   = regexp.MustCompile(`<meta\s+property="og:description"\s+content="([^"]*)"`)
	reOGImage  = regexp.MustCompile(`<meta\s+property="og:image"\s+content="([^"]*)"`)
	// "Full Name (@username) • Instagram photos and videos"
	reOGTitleUser = regexp.MustCompile(`\(@([^)]+)\)`)
	reOGTitleName = regexp.MustCompile(`^(.+?)\s+\(`)
)

// parseMetaTags extracts a partial Profile from og: meta tags in the HTML
// page. It is a fallback when the JSON API is unavailable; the follower counts
// and post counts are not available from this path.
func parseMetaTags(body []byte, username string) (*Profile, error) {
	html := string(body)

	ogTitle := metaMatch(reOGTitle, html)
	ogDesc := metaMatch(reOGDesc, html)
	ogImage := metaMatch(reOGImage, html)

	// og:title for a profile is typically "Full Name (@username) • Instagram…"
	name := ""
	if m := reOGTitleName.FindStringSubmatch(ogTitle); m != nil {
		name = strings.TrimSpace(m[1])
	}
	if name == "" {
		name = ogTitle
	}

	if name == "" && ogDesc == "" && ogImage == "" {
		return nil, ErrBlocked
	}

	return &Profile{
		Username:   username,
		FullName:   name,
		Bio:        ogDesc,
		ProfilePic: ogImage,
		URL:        BaseURL + "/" + username + "/",
		FetchedAt:  time.Now(),
	}, nil
}

func metaMatch(re *regexp.Regexp, s string) string {
	m := re.FindStringSubmatch(s)
	if len(m) > 1 {
		return htmlUnescape(strings.TrimSpace(m[1]))
	}
	return ""
}

// htmlUnescape decodes the common HTML entities that appear in og: meta tag values.
func htmlUnescape(s string) string {
	r := strings.NewReplacer(
		"&amp;", "&",
		"&lt;", "<",
		"&gt;", ">",
		"&quot;", `"`,
		"&#39;", "'",
		"&nbsp;", " ",
	)
	return r.Replace(s)
}
