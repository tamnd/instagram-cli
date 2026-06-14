package instagram

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring, which need no network. The client's HTTP behaviour and
// the parsers are covered in instagram_test.go.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "instagram" {
		t.Errorf("Scheme = %q, want instagram", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s ...]", info.Hosts, Host)
	}
	if info.Identity.Binary != "ig" {
		t.Errorf("Identity.Binary = %q, want ig", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct{ in, typ, id string }{
		{"instagram", "profile", "instagram"},
		{"@instagram", "profile", "instagram"},
		{"https://www.instagram.com/instagram/", "profile", "instagram"},
		{"https://www.instagram.com/p/DZf6PYtGyay/", "post", "DZf6PYtGyay"},
		{"https://www.instagram.com/reel/DCxqp8/", "reel", "DCxqp8"},
		{"https://www.instagram.com/instagram/p/DZf6PYtGyay/", "post", "DZf6PYtGyay"},
		{"https://www.instagram.com/instagram/reel/DCxqp8/", "reel", "DCxqp8"},
		{"p/DZf6PYtGyay", "post", "DZf6PYtGyay"},
		{"reel/DCxqp8", "reel", "DCxqp8"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestClassifyReservedPathsAreNotProfiles(t *testing.T) {
	for _, in := range []string{"https://www.instagram.com/explore/", "https://www.instagram.com/accounts/login/"} {
		if _, _, err := (Domain{}).Classify(in); err == nil {
			t.Errorf("Classify(%q) should not resolve a reserved path as a profile", in)
		}
	}
}

func TestLocate(t *testing.T) {
	cases := []struct{ typ, id, want string }{
		{"profile", "instagram", "https://www.instagram.com/instagram/"},
		{"post", "DZf6PYtGyay", "https://www.instagram.com/p/DZf6PYtGyay/"},
		{"reel", "DCxqp8", "https://www.instagram.com/reel/DCxqp8/"},
	}
	for _, tc := range cases {
		got, err := Domain{}.Locate(tc.typ, tc.id)
		if err != nil || got != tc.want {
			t.Errorf("Locate(%q,%q) = (%q, %v), want %q", tc.typ, tc.id, got, err, tc.want)
		}
	}
	if _, err := (Domain{}).Locate("nope", "x"); err == nil {
		t.Error("Locate of an unknown type should error")
	}
}

func TestUsernameOf(t *testing.T) {
	if u, err := usernameOf("@instagram"); err != nil || u != "instagram" {
		t.Errorf("usernameOf(@instagram) = (%q, %v)", u, err)
	}
	if _, err := usernameOf("https://www.instagram.com/p/ABC/"); err == nil {
		t.Error("a post URL is not a username")
	}
}

func TestShortcodeOf(t *testing.T) {
	cases := []struct {
		in, defKind, wantSC, wantKind string
	}{
		{"DZf6PYtGyay", "post", "DZf6PYtGyay", "post"},
		{"DZf6PYtGyay", "reel", "DZf6PYtGyay", "reel"},
		{"3918106344851973810", "post", "DZf6PYtGyay", "post"},
		{"https://www.instagram.com/reel/ABC/", "post", "ABC", "reel"},
		{"https://www.instagram.com/p/XYZ/", "reel", "XYZ", "post"},
	}
	for _, tc := range cases {
		sc, kind, err := shortcodeOf(tc.in, tc.defKind)
		if err != nil || sc != tc.wantSC || kind != tc.wantKind {
			t.Errorf("shortcodeOf(%q,%q) = (%q,%q,%v), want (%q,%q)",
				tc.in, tc.defKind, sc, kind, err, tc.wantSC, tc.wantKind)
		}
	}
}

// TestHostWiring mounts the driver in a kit Host and checks the round trip: a
// record mints to its URI, its body is readable, and a bare id resolves back to
// the same URI. The init in domain.go registers the domain.
func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	p := &Post{Shortcode: "DZf6PYtGyay", URL: PostURL("DZf6PYtGyay", "post"), Caption: "hello"}
	u, err := h.Mint(p)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	if want := "instagram://post/DZf6PYtGyay"; u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}
	if body, ok := h.Body(p); !ok || body == "" {
		t.Errorf("Body = (%q, %v), want non-empty", body, ok)
	}

	prof := &Profile{Username: "instagram", URL: ProfileURL("instagram"), Biography: "bio"}
	pu, err := h.Mint(prof)
	if err != nil {
		t.Fatalf("Mint profile: %v", err)
	}
	if want := "instagram://profile/instagram"; pu.String() != want {
		t.Errorf("Mint profile = %q, want %q", pu.String(), want)
	}

	got, err := h.ResolveOn("instagram", "instagram")
	if err != nil || got.String() != "instagram://profile/instagram" {
		t.Errorf("ResolveOn = (%q, %v), want instagram://profile/instagram", got.String(), err)
	}
}
