package instagram

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// fixture is a trimmed copy of the web_profile_info response for a public
// account. The shape matches what Instagram returns to a logged-out browser
// with the X-IG-App-ID header.
const fixtureProfile = `{
  "data": {
    "user": {
      "id": "25025320",
      "username": "instagram",
      "full_name": "Instagram",
      "biography": "Discover what's next.",
      "external_url": "https://www.instagram.com/",
      "is_private": false,
      "is_verified": true,
      "profile_pic_url": "https://example.com/pic.jpg",
      "edge_followed_by": { "count": 672000000 },
      "edge_follow":      { "count": 355 },
      "edge_owner_to_timeline_media": {
        "count": 8128,
        "edges": [
          { "node": {
              "id": "111",
              "shortcode": "CXabcXYZ",
              "__typename": "GraphImage",
              "thumbnail_src": "https://example.com/thumb.jpg",
              "display_url": "https://example.com/display.jpg",
              "is_video": false,
              "taken_at_timestamp": 1700000000,
              "edge_media_to_caption": {
                "edges": [{ "node": { "text": "A beautiful photo." }}]
              },
              "edge_media_preview_like": { "count": 12345 },
              "edge_media_to_comment":   { "count": 789 }
          }},
          { "node": {
              "id": "222",
              "shortcode": "CXdef456",
              "__typename": "GraphVideo",
              "thumbnail_src": "https://example.com/thumb2.jpg",
              "display_url": "https://example.com/display2.jpg",
              "is_video": true,
              "video_url": "https://example.com/video.mp4",
              "taken_at_timestamp": 1699000000,
              "edge_media_to_caption": { "edges": [] },
              "edge_media_preview_like": { "count": 5000 },
              "edge_media_to_comment":   { "count": 200 }
          }}
        ]
      }
    }
  },
  "status": "ok"
}`

// fixtureBlocked is the response Instagram returns when it requires login.
const fixtureBlocked = `{"message":"login_required","status":"fail"}`

// fixtureHTML is a minimal HTML page with og: meta tags, used for the
// fallback path.
const fixtureHTML = `<!DOCTYPE html>
<html>
<head>
<meta property="og:title" content="NASA (@nasa) • Instagram photos and videos">
<meta property="og:description" content="Explore the universe with NASA.">
<meta property="og:image" content="https://example.com/nasa.jpg">
</head>
<body></body>
</html>`

// testClient returns a Client that points at the given test server URL and
// has no pacing (so tests run instantly).
func testClient(baseURL string) *Client {
	cfg := DefaultConfig()
	cfg.BaseURL = baseURL
	cfg.Rate = 0
	cfg.Retries = 1
	return NewClient(cfg)
}

func TestFetchProfile_OK(t *testing.T) {
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if r.Header.Get("X-IG-App-ID") == "" {
			t.Error("X-IG-App-ID header missing")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureProfile))
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	p, err := c.FetchProfile(context.Background(), "instagram")
	if err != nil {
		t.Fatalf("FetchProfile: %v", err)
	}
	if p.Username != "instagram" {
		t.Errorf("Username = %q", p.Username)
	}
	if p.FullName != "Instagram" {
		t.Errorf("FullName = %q", p.FullName)
	}
	if p.Bio != "Discover what's next." {
		t.Errorf("Bio = %q", p.Bio)
	}
	if p.Followers != 672000000 {
		t.Errorf("Followers = %d", p.Followers)
	}
	if p.Posts != 8128 {
		t.Errorf("Posts = %d", p.Posts)
	}
	if !p.IsVerified {
		t.Error("IsVerified = false")
	}
	if p.ProfilePic != "https://example.com/pic.jpg" {
		t.Errorf("ProfilePic = %q", p.ProfilePic)
	}
}

func TestFetchProfile_Blocked401(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(fixtureBlocked))
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	// No HTML fallback since the server returns 401 for everything.
	_, err := c.FetchProfile(context.Background(), "anyone")
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("want ErrBlocked, got %v", err)
	}
}

func TestFetchProfile_Blocked403(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	_, err := c.FetchProfile(context.Background(), "anyone")
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("want ErrBlocked, got %v", err)
	}
}

func TestFetchProfile_BlockedLoginJSON(t *testing.T) {
	// The server returns 200 but with a login_required body (as Instagram
	// sometimes does from certain IPs).
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureBlocked))
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	_, err := c.FetchProfile(context.Background(), "anyone")
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("want ErrBlocked, got %v", err)
	}
}

func TestFetchProfile_LoginRedirect(t *testing.T) {
	// Simulate Instagram redirecting to the login wall.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, "/accounts/login/", http.StatusFound)
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	_, err := c.FetchProfile(context.Background(), "anyone")
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("want ErrBlocked, got %v", err)
	}
}

func TestFetchPosts_OK(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureProfile))
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	posts, err := c.FetchPosts(context.Background(), "instagram", 0)
	if err != nil {
		t.Fatalf("FetchPosts: %v", err)
	}
	if len(posts) != 2 {
		t.Fatalf("posts = %d, want 2", len(posts))
	}
	p0 := posts[0]
	if p0.ID != "111" {
		t.Errorf("post[0].ID = %q", p0.ID)
	}
	if p0.ShortCode != "CXabcXYZ" {
		t.Errorf("post[0].ShortCode = %q", p0.ShortCode)
	}
	if p0.Caption != "A beautiful photo." {
		t.Errorf("post[0].Caption = %q", p0.Caption)
	}
	if p0.Likes != 12345 {
		t.Errorf("post[0].Likes = %d", p0.Likes)
	}
	if p0.IsVideo {
		t.Error("post[0].IsVideo = true, want false")
	}
	wantURL := BaseURL + "/p/CXabcXYZ/"
	if p0.URL != wantURL {
		t.Errorf("post[0].URL = %q, want %q", p0.URL, wantURL)
	}

	p1 := posts[1]
	if !p1.IsVideo {
		t.Error("post[1].IsVideo = false, want true")
	}
	if p1.VideoURL != "https://example.com/video.mp4" {
		t.Errorf("post[1].VideoURL = %q", p1.VideoURL)
	}
}

func TestFetchPosts_Limit(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureProfile))
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	posts, err := c.FetchPosts(context.Background(), "instagram", 1)
	if err != nil {
		t.Fatalf("FetchPosts: %v", err)
	}
	if len(posts) != 1 {
		t.Errorf("posts = %d, want 1 (limit applied)", len(posts))
	}
}

func TestFetchPosts_Blocked(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	_, err := c.FetchPosts(context.Background(), "anyone", 0)
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("want ErrBlocked, got %v", err)
	}
}

func TestHTMLFallback(t *testing.T) {
	// The JSON API returns 401; the HTML endpoint returns the og: meta page.
	apiBlocked := false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "web_profile_info") {
			// First request (JSON API): blocked.
			w.WriteHeader(http.StatusUnauthorized)
			apiBlocked = true
			return
		}
		// Second request (HTML fallback): serve og: meta.
		w.Header().Set("Content-Type", "text/html")
		_, _ = w.Write([]byte(fixtureHTML))
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	p, err := c.FetchProfile(context.Background(), "nasa")
	if err != nil {
		t.Fatalf("FetchProfile (HTML fallback): %v", err)
	}
	if !apiBlocked {
		t.Error("API endpoint was never called")
	}
	if p.FullName != "NASA" {
		t.Errorf("FullName from og:title = %q, want NASA", p.FullName)
	}
	if p.Bio != "Explore the universe with NASA." {
		t.Errorf("Bio from og:description = %q", p.Bio)
	}
	if p.ProfilePic != "https://example.com/nasa.jpg" {
		t.Errorf("ProfilePic from og:image = %q", p.ProfilePic)
	}
}

func TestGet_RetriesOn503(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(fixtureProfile))
	}))
	defer srv.Close()

	cfg := DefaultConfig()
	cfg.BaseURL = srv.URL
	cfg.Rate = 0
	cfg.Retries = 5
	c := NewClient(cfg)

	p, err := c.FetchProfile(context.Background(), "instagram")
	if err != nil {
		t.Fatalf("FetchProfile: %v (after retries)", err)
	}
	if p.Username != "instagram" {
		t.Errorf("Username = %q after retries", p.Username)
	}
	if hits != 3 {
		t.Errorf("server saw %d requests, want 3", hits)
	}
}

func TestGet_NoRetryOn403(t *testing.T) {
	hits := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	c := testClient(srv.URL)
	_, err := c.FetchProfile(context.Background(), "anyone")
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("want ErrBlocked, got %v", err)
	}
	// The JSON API hit + the HTML fallback also hit 403 = 2 total.
	// But with no retry on 403, each path hits only once.
	if hits > 2 {
		t.Errorf("server saw %d hits, want at most 2 (no retry on 403)", hits)
	}
}

func TestParseProfile_NilUser(t *testing.T) {
	body := []byte(`{"data":{},"status":"ok"}`)
	_, err := parseProfile(body)
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("want ErrNotFound for null user, got %v", err)
	}
}

func TestIsBlocked(t *testing.T) {
	cases := []struct {
		body    string
		blocked bool
	}{
		{`{"message":"login_required","status":"fail"}`, true},
		{`{"data":{"user":null},"checkpoint_url":"/challenge/"}`, true},
		{`{"data":{"user":{"username":"x"}}}`, false},
	}
	for _, tc := range cases {
		got := isBlocked([]byte(tc.body))
		if got != tc.blocked {
			t.Errorf("isBlocked(%q) = %v, want %v", tc.body, got, tc.blocked)
		}
	}
}

func TestParseMetaTags_FullNameFromOGTitle(t *testing.T) {
	html := `<html>
<meta property="og:title" content="Elon Musk (@elonmusk) • Instagram">
<meta property="og:description" content="Technoking.">
<meta property="og:image" content="https://example.com/elon.jpg">
</html>`
	p, err := parseMetaTags([]byte(html), "elonmusk")
	if err != nil {
		t.Fatal(err)
	}
	if p.FullName != "Elon Musk" {
		t.Errorf("FullName = %q, want Elon Musk", p.FullName)
	}
	if p.Username != "elonmusk" {
		t.Errorf("Username = %q", p.Username)
	}
}

func TestParseMetaTags_NoData(t *testing.T) {
	html := `<html><body>no meta here</body></html>`
	_, err := parseMetaTags([]byte(html), "nobody")
	if !errors.Is(err, ErrBlocked) {
		t.Errorf("empty meta page want ErrBlocked, got %v", err)
	}
}
