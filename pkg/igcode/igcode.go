// Package igcode converts between an Instagram shortcode and the numeric media
// id it encodes. A shortcode is the media primary key written in base64url over
// the alphabet A-Za-z0-9-_, so the mapping is exact and reversible:
//
//	ShortcodeToID("DZf6PYtGyay") == 3918106344851973810
//	IDToShortcode(3918106344851973810) == "DZf6PYtGyay"
//
// The driver uses this to keep its resolver network-free: a pasted /p/{code}/
// url becomes a (type, id) with no fetch, and a bare media id becomes a
// canonical permalink.
package igcode

import (
	"fmt"
	"strings"
)

const alphabet = "ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz0123456789-_"

// index inverts the alphabet for decoding. -1 marks a byte that is not a
// shortcode character.
var index = func() [256]int {
	var t [256]int
	for i := range t {
		t[i] = -1
	}
	for i := 0; i < len(alphabet); i++ {
		t[alphabet[i]] = i
	}
	return t
}()

// ShortcodeToID decodes a shortcode into its media id. It returns an error if
// the input is empty or carries a character outside the base64url alphabet.
func ShortcodeToID(shortcode string) (uint64, error) {
	if shortcode == "" {
		return 0, fmt.Errorf("empty shortcode")
	}
	var n uint64
	for i := 0; i < len(shortcode); i++ {
		v := index[shortcode[i]]
		if v < 0 {
			return 0, fmt.Errorf("invalid shortcode character %q", shortcode[i])
		}
		n = n*64 + uint64(v)
	}
	return n, nil
}

// IDToShortcode encodes a media id into its shortcode. Zero encodes to "0", the
// first letter of the alphabet's zero digit, matching Instagram's own encoding.
func IDToShortcode(id uint64) string {
	if id == 0 {
		return string(alphabet[0])
	}
	var b strings.Builder
	var rev []byte
	for id > 0 {
		rev = append(rev, alphabet[id%64])
		id /= 64
	}
	for i := len(rev) - 1; i >= 0; i-- {
		b.WriteByte(rev[i])
	}
	return b.String()
}

// IsShortcode reports whether s looks like a shortcode: non-empty and built
// entirely from the base64url alphabet.
func IsShortcode(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if index[s[i]] < 0 {
			return false
		}
	}
	return true
}
