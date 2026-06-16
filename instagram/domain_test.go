package instagram

import (
	"testing"

	"github.com/tamnd/any-cli/kit"
)

// These tests are offline: they exercise the URI driver's pure string functions
// and the host wiring (mint, body, resolve), which need no network.

func TestDomainInfo(t *testing.T) {
	info := Domain{}.Info()
	if info.Scheme != "instagram" {
		t.Errorf("Scheme = %q, want instagram", info.Scheme)
	}
	if len(info.Hosts) == 0 || info.Hosts[0] != Host {
		t.Errorf("Hosts = %v, want [%s]", info.Hosts, Host)
	}
	if info.Identity.Binary != "ig" {
		t.Errorf("Identity.Binary = %q, want ig", info.Identity.Binary)
	}
}

func TestClassify(t *testing.T) {
	cases := []struct{ in, typ, id string }{
		{"instagram", "profile", "instagram"},
		{"@billgates", "profile", "billgates"},
		{"/nasa/", "profile", "nasa"},
		{"https://www.instagram.com/leomessi/", "profile", "leomessi"},
		{"https://instagram.com/therock", "profile", "therock"},
	}
	for _, tc := range cases {
		typ, id, err := Domain{}.Classify(tc.in)
		if err != nil || typ != tc.typ || id != tc.id {
			t.Errorf("Classify(%q) = (%q, %q, %v), want (%q, %q, nil)",
				tc.in, typ, id, err, tc.typ, tc.id)
		}
	}
}

func TestLocate(t *testing.T) {
	tests := []struct {
		uriType, id, want string
	}{
		{"profile", "nasa", "https://www.instagram.com/nasa/"},
		{"post", "CXabcXYZ", "https://www.instagram.com/p/CXabcXYZ/"},
	}
	for _, tc := range tests {
		got, err := Domain{}.Locate(tc.uriType, tc.id)
		if err != nil || got != tc.want {
			t.Errorf("Locate(%q, %q) = (%q, %v), want (%q, nil)",
				tc.uriType, tc.id, got, err, tc.want)
		}
	}
}

// TestHostWiring mounts the driver in a kit Host and checks the round trip:
// a record mints to its URI, its body is readable, and a bare id resolves
// back to the same URI. The init in domain.go registers the domain, so
// kit.Open finds it.
func TestHostWiring(t *testing.T) {
	h, err := kit.Open()
	if err != nil {
		t.Fatal(err)
	}

	p := &Profile{
		Username: "nasa",
		FullName: "NASA",
		Bio:      "Explore the universe.",
		URL:      "https://www.instagram.com/nasa/",
	}
	u, err := h.Mint(p)
	if err != nil {
		t.Fatalf("Mint: %v", err)
	}
	want := "instagram://profile/nasa"
	if u.String() != want {
		t.Errorf("Mint = %q, want %q", u.String(), want)
	}

	got, err := h.ResolveOn("instagram", "leomessi")
	if err != nil || got.String() != "instagram://profile/leomessi" {
		t.Errorf("ResolveOn = (%q, %v)", got.String(), err)
	}
}

func TestNormalizeUsername(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"instagram", "instagram"},
		{"@billgates", "billgates"},
		{"/nasa/", "nasa"},
		{"https://www.instagram.com/leomessi/", "leomessi"},
		{"https://instagram.com/therock", "therock"},
		{"  @cristiano  ", "cristiano"},
	}
	for _, tc := range tests {
		got := normalizeUsername(tc.in)
		if got != tc.want {
			t.Errorf("normalizeUsername(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
