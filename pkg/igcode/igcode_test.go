package igcode

import "testing"

func TestKnownPairs(t *testing.T) {
	cases := []struct {
		shortcode string
		id        uint64
	}{
		{"DZf6PYtGyay", 3918106344851973810},
		{"B", 1},
		{"A", 0},
	}
	for _, tc := range cases {
		got, err := ShortcodeToID(tc.shortcode)
		if err != nil || got != tc.id {
			t.Errorf("ShortcodeToID(%q) = (%d, %v), want %d", tc.shortcode, got, err, tc.id)
		}
		if sc := IDToShortcode(tc.id); tc.id != 0 && sc != tc.shortcode {
			t.Errorf("IDToShortcode(%d) = %q, want %q", tc.id, sc, tc.shortcode)
		}
	}
}

func TestRoundTrip(t *testing.T) {
	ids := []uint64{1, 63, 64, 65, 1000, 1<<32 + 7, 3918106344851973810, 1<<63 - 1}
	for _, id := range ids {
		sc := IDToShortcode(id)
		back, err := ShortcodeToID(sc)
		if err != nil {
			t.Fatalf("ShortcodeToID(%q): %v", sc, err)
		}
		if back != id {
			t.Errorf("round trip %d -> %q -> %d", id, sc, back)
		}
	}
}

func TestInvalid(t *testing.T) {
	if _, err := ShortcodeToID(""); err == nil {
		t.Error("empty shortcode should error")
	}
	if _, err := ShortcodeToID("has space"); err == nil {
		t.Error("shortcode with a space should error")
	}
}

func TestIsShortcode(t *testing.T) {
	if !IsShortcode("DZf6PYtGyay") {
		t.Error("valid shortcode rejected")
	}
	if IsShortcode("has space") || IsShortcode("") {
		t.Error("invalid shortcode accepted")
	}
}
