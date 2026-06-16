// Package instagram is the library behind the ig command line: the HTTP
// client, request shaping, blocked-request detection, and the typed data
// models for Instagram.
//
// Instagram is a Tier-C site: most data paths require session cookies from a
// real account, and the service actively detects datacenter IPs. The client
// here tries the JSON API first (web_profile_info) and falls back to og: meta
// tag parsing on the HTML page. Both paths detect the login wall and return
// ErrBlocked, which the CLI maps to exit code 5.
package instagram

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Host is the primary domain this client reads.
const Host = "www.instagram.com"

// BaseURL is the root every request is built from.
const BaseURL = "https://" + Host

// DefaultUserAgent is a real browser UA; a bot-detection string like
// "instagram/dev" is filtered at the edge.
const DefaultUserAgent = "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_15_7) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36"

// DefaultAppID is the Instagram web application ID embedded in the public JS
// bundle. It is required by the web_profile_info endpoint.
const DefaultAppID = "936619743392459"

// profileInfoPath is the JSON endpoint for public profile data.
const profileInfoPath = "/api/v1/users/web_profile_info/"

// ErrBlocked is returned when Instagram refuses the request: a 401/403 status,
// a redirect to the login page, or a body containing a login-required signal.
// The CLI maps this to exit code 5.
var ErrBlocked = errors.New("blocked: Instagram requires login or CAPTCHA")

// ErrNotFound is returned when the API returns a null user or a 404.
var ErrNotFound = errors.New("not found")

// Config holds constructor parameters for a Client.
type Config struct {
	BaseURL   string
	UserAgent string
	AppID     string
	Rate      time.Duration
	Retries   int
	Timeout   time.Duration
}

// DefaultConfig returns polite defaults for a site that actively detects bots.
func DefaultConfig() Config {
	return Config{
		BaseURL:   BaseURL,
		UserAgent: DefaultUserAgent,
		AppID:     DefaultAppID,
		Rate:      500 * time.Millisecond,
		Retries:   3,
		Timeout:   30 * time.Second,
	}
}

// Client is a rate-limited HTTP client for Instagram's public endpoints.
type Client struct {
	cfg  Config
	http *http.Client
	mu   sync.Mutex
	last time.Time
}

// NewClient returns a Client with sensible defaults.
func NewClient(cfg Config) *Client {
	if cfg.BaseURL == "" {
		cfg.BaseURL = BaseURL
	}
	if cfg.UserAgent == "" {
		cfg.UserAgent = DefaultUserAgent
	}
	if cfg.AppID == "" {
		cfg.AppID = DefaultAppID
	}
	if cfg.Rate <= 0 {
		cfg.Rate = 500 * time.Millisecond
	}
	if cfg.Retries <= 0 {
		cfg.Retries = 3
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = 30 * time.Second
	}
	cl := &http.Client{
		Timeout: cfg.Timeout,
		// CheckRedirect intercepts auth-wall redirects before the standard
		// client follows them, so we can return ErrBlocked cleanly instead of
		// following to an infinite-redirect loop or a 200 login page.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if isLoginRedirect(req.URL.String()) {
				return http.ErrUseLastResponse
			}
			if len(via) >= 10 {
				return http.ErrUseLastResponse
			}
			return nil
		},
	}
	return &Client{
		cfg:  cfg,
		http: cl,
	}
}

// FetchProfile fetches a public Instagram profile by username. It tries the
// JSON API first; if that returns a blocked signal it falls back to og: meta
// tags from the HTML page. If both are blocked, it returns ErrBlocked.
func (c *Client) FetchProfile(ctx context.Context, username string) (*Profile, error) {
	username = strings.TrimPrefix(strings.Trim(username, "/"), "@")

	// Try the JSON API.
	body, err := c.getProfileJSON(ctx, username)
	if err != nil {
		if errors.Is(err, ErrBlocked) {
			// Fall back to HTML og: meta parsing.
			return c.fetchHTMLProfile(ctx, username)
		}
		return nil, err
	}
	return parseProfile(body)
}

// FetchPosts fetches up to limit recent public posts for a user. It uses the
// same JSON endpoint as FetchProfile. A limit of 0 returns all available posts
// (up to the API page size, which is 12).
func (c *Client) FetchPosts(ctx context.Context, username string, limit int) ([]Post, error) {
	username = strings.TrimPrefix(strings.Trim(username, "/"), "@")
	body, err := c.getProfileJSON(ctx, username)
	if err != nil {
		return nil, err
	}
	return parsePosts(body, limit)
}

// getProfileJSON fetches the raw JSON from the web_profile_info endpoint.
func (c *Client) getProfileJSON(ctx context.Context, username string) ([]byte, error) {
	url := c.cfg.BaseURL + profileInfoPath + "?username=" + username
	headers := map[string]string{
		"X-IG-App-ID": c.cfg.AppID,
		"Accept":      "application/json",
	}
	return c.get(ctx, url, headers)
}

// fetchHTMLProfile fetches the HTML profile page and parses og: meta tags.
func (c *Client) fetchHTMLProfile(ctx context.Context, username string) (*Profile, error) {
	url := c.cfg.BaseURL + "/" + username + "/"
	body, err := c.get(ctx, url, nil)
	if err != nil {
		return nil, err
	}
	return parseMetaTags(body, username)
}

// get fetches a URL with retry and rate pacing.
func (c *Client) get(ctx context.Context, url string, extraHeaders map[string]string) ([]byte, error) {
	var lastErr error
	for attempt := 0; attempt <= c.cfg.Retries; attempt++ {
		if attempt > 0 {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff(attempt)):
			}
		}
		body, retry, err := c.do(ctx, url, extraHeaders)
		if err == nil {
			return body, nil
		}
		lastErr = err
		if !retry {
			return nil, err
		}
	}
	return nil, fmt.Errorf("get %s: %w", url, lastErr)
}

// do performs one HTTP GET. It returns (body, retry, err).
// retry is true for transient failures (429, 5xx, network errors).
// ErrBlocked is returned immediately without retry.
func (c *Client) do(ctx context.Context, rawURL string, extraHeaders map[string]string) (body []byte, retry bool, err error) {
	c.pace()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, false, err
	}

	req.Header.Set("User-Agent", c.cfg.UserAgent)
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	for k, v := range extraHeaders {
		req.Header.Set(k, v)
	}

	// Instagram reads a self-referencing Referer as a scraping signal; send none.

	resp, err := c.http.Do(req)
	if err != nil {
		// When CheckRedirect returns ErrUseLastResponse and the redirect target
		// is a login page, Do returns the last response alongside the error.
		// We check the response below if available; otherwise surface the error.
		if resp == nil {
			return nil, true, err
		}
		// Fall through: the response holds the redirect target's status.
	}
	if resp == nil {
		return nil, true, fmt.Errorf("nil response")
	}
	defer func() { _ = resp.Body.Close() }()

	// Check for auth-wall redirect (CheckRedirect halted at a login URL).
	final := resp.Request.URL.String()
	if isLoginRedirect(final) {
		return nil, false, fmt.Errorf("%w (redirected to %s)", ErrBlocked, final)
	}
	// A 3xx we stopped following toward a login page.
	if resp.StatusCode >= 300 && resp.StatusCode < 400 {
		loc := resp.Header.Get("Location")
		if isLoginRedirect(loc) {
			return nil, false, fmt.Errorf("%w (redirect to %s)", ErrBlocked, loc)
		}
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// fall through
	case http.StatusUnauthorized, http.StatusForbidden:
		return nil, false, fmt.Errorf("%w (HTTP %d)", ErrBlocked, resp.StatusCode)
	case http.StatusNotFound:
		return nil, false, ErrNotFound
	case http.StatusTooManyRequests:
		return nil, true, fmt.Errorf("http %d", resp.StatusCode)
	default:
		if resp.StatusCode >= 500 {
			return nil, true, fmt.Errorf("http %d", resp.StatusCode)
		}
		return nil, false, fmt.Errorf("http %d", resp.StatusCode)
	}

	b, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, true, err
	}

	// Check body-level blocked signals even on a 200.
	if isBlocked(b) {
		return nil, false, ErrBlocked
	}

	return b, false, nil
}

// isLoginRedirect reports whether a URL is one of Instagram's login gate paths.
func isLoginRedirect(u string) bool {
	for _, p := range []string{"/accounts/login/", "/challenge/", "/checkpoint/"} {
		if strings.Contains(u, p) {
			return true
		}
	}
	return false
}

// pace blocks until at least cfg.Rate has passed since the previous request.
func (c *Client) pace() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cfg.Rate <= 0 {
		c.last = time.Now()
		return
	}
	if wait := c.cfg.Rate - time.Since(c.last); wait > 0 {
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
