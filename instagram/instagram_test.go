package instagram

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestGetRetriesOn503(t *testing.T) {
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if hits < 3 {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte("recovered"))
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	c.Retries = 5

	start := time.Now()
	body, status, err := c.get(context.Background(), srv.URL, nil)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "recovered" {
		t.Errorf("body = %q after retries", body)
	}
	if status != http.StatusOK {
		t.Errorf("status = %d, want 200", status)
	}
	if hits != 3 {
		t.Errorf("server saw %d hits, want 3", hits)
	}
	if time.Since(start) < 500*time.Millisecond {
		t.Error("retries did not back off")
	}
}

func TestGetReportsStatusOnWall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer srv.Close()

	c := NewClient()
	c.Rate = 0
	_, status, err := c.get(context.Background(), srv.URL, nil)
	if err == nil {
		t.Fatal("want an error on 401")
	}
	if status != http.StatusUnauthorized {
		t.Errorf("status = %d, want 401", status)
	}
}

func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestParseProfile(t *testing.T) {
	p, posts, err := parseProfile(readFixture(t, "web_profile_info.json"))
	if err != nil {
		t.Fatal(err)
	}
	if p.Username != "instagram" {
		t.Errorf("Username = %q, want instagram", p.Username)
	}
	if p.Source != "api" {
		t.Errorf("Source = %q, want api", p.Source)
	}
	if !p.Verified {
		t.Error("want Verified true")
	}
	if p.Followers <= 0 || p.Following <= 0 || p.Posts <= 0 {
		t.Errorf("counts not filled: followers=%d following=%d posts=%d", p.Followers, p.Following, p.Posts)
	}
	if p.URL != "https://www.instagram.com/instagram/" {
		t.Errorf("URL = %q", p.URL)
	}
	if len(posts) != 2 {
		t.Fatalf("got %d posts, want 2", len(posts))
	}
}

func TestParsePostNode(t *testing.T) {
	_, posts, err := parseProfile(readFixture(t, "web_profile_info.json"))
	if err != nil {
		t.Fatal(err)
	}
	// First node is a carousel, second is a video (see fixture).
	carousel, video := posts[0], posts[1]
	if carousel.Type != "carousel" {
		t.Errorf("first post Type = %q, want carousel", carousel.Type)
	}
	if carousel.Shortcode == "" || carousel.URL == "" {
		t.Error("carousel missing shortcode/url")
	}
	if carousel.Likes <= 0 || carousel.Comments <= 0 {
		t.Errorf("carousel counts: likes=%d comments=%d", carousel.Likes, carousel.Comments)
	}
	if carousel.Source != "api" {
		t.Errorf("Source = %q, want api", carousel.Source)
	}
	if carousel.Timestamp == "" {
		t.Error("carousel missing RFC3339 timestamp")
	}
	if video.Type != "video" {
		t.Errorf("second post Type = %q, want video", video.Type)
	}
	if video.Views <= 0 {
		t.Errorf("video Views = %d, want > 0", video.Views)
	}
}

func TestParsePostPage(t *testing.T) {
	post, err := parsePostPage(readFixture(t, "post.html"), "DZf6PYtGyay", "post")
	if err != nil {
		t.Fatal(err)
	}
	if post.Source != "ssr" {
		t.Errorf("Source = %q, want ssr", post.Source)
	}
	if post.Author != "instagram" {
		t.Errorf("Author = %q, want instagram", post.Author)
	}
	if post.Type != "post" {
		t.Errorf("Type = %q, want post", post.Type)
	}
	if post.Likes != 585000 {
		t.Errorf("Likes = %d, want 585000 (rounded 585K)", post.Likes)
	}
	if post.Comments != 43000 {
		t.Errorf("Comments = %d, want 43000 (rounded 43K)", post.Comments)
	}
	if post.Caption == "" {
		t.Error("Caption not parsed from og:description")
	}
	if post.DisplayURL == "" {
		t.Error("DisplayURL not parsed from og:image")
	}
	if post.Timestamp == "" {
		t.Error("Timestamp not parsed from og date")
	}
	if post.URL != "https://www.instagram.com/p/DZf6PYtGyay/" {
		t.Errorf("URL = %q", post.URL)
	}
}

func TestParsePostPageReelKind(t *testing.T) {
	post, err := parsePostPage(readFixture(t, "post.html"), "DZf6PYtGyay", "reel")
	if err != nil {
		t.Fatal(err)
	}
	if post.Type != "reel" {
		t.Errorf("Type = %q, want reel", post.Type)
	}
	if post.URL != "https://www.instagram.com/reel/DZf6PYtGyay/" {
		t.Errorf("URL = %q, want /reel/ permalink", post.URL)
	}
}

func TestParsePostPageWalled(t *testing.T) {
	_, err := parsePostPage(readFixture(t, "walled.html"), "ABC", "post")
	if !errors.Is(err, ErrWalled) {
		t.Errorf("err = %v, want ErrWalled", err)
	}
}

func TestParseCount(t *testing.T) {
	cases := []struct {
		in   string
		want int64
	}{
		{"585K", 585000},
		{"43K", 43000},
		{"1.2M", 1200000},
		{"2B", 2000000000},
		{"1,234", 1234},
		{"7", 7},
		{"", 0},
	}
	for _, tc := range cases {
		if got := parseCount(tc.in); got != tc.want {
			t.Errorf("parseCount(%q) = %d, want %d", tc.in, got, tc.want)
		}
	}
}

func TestCanonicalShortcode(t *testing.T) {
	if sc, ok := CanonicalShortcode("DZf6PYtGyay"); !ok || sc != "DZf6PYtGyay" {
		t.Errorf("bare shortcode: got (%q, %v)", sc, ok)
	}
	if sc, ok := CanonicalShortcode("3918106344851973810"); !ok || sc != "DZf6PYtGyay" {
		t.Errorf("numeric id: got (%q, %v), want DZf6PYtGyay", sc, ok)
	}
	if _, ok := CanonicalShortcode(""); ok {
		t.Error("empty should not be a shortcode")
	}
	if _, ok := CanonicalShortcode("has space"); ok {
		t.Error("input with a space is not a shortcode")
	}
}
