package util

// Random utility functions that are useful in various places.

import (
	"strings"
	"rand"
	"sync"
)

func RemovePrefixedNick(text, nick string) (string, bool) {
	if HasPrefixedNick(text, nick) {
		text = strings.TrimSpace(text[len(nick)+1:])
		return text, true
	}
	return text, false
}

func HasPrefixedNick(text, nick string) bool {
	prefixed := false
	if strings.HasPrefix(strings.ToLower(text), strings.ToLower(nick)) {
		switch text[len(nick)] {
		// This is nicer than an if statement :-)
		// We only cut off the nick if it's followed by one of these chars
		// and an optional space, to indicate that it was prefixed to the text.
		case ':', ';', ',', '>', '-':
			prefixed = true
		}
	}
	return prefixed
}

// Removes mIRC-style colours from a string.
// These colours match the following BNF notation:
//   colour ::= idchar | idchar colnum | idchar colnum "," colnum
//   idchar ::= "\003"
//   colnum ::= digit | digit digit
//   digit  ::= "0" "1" "2" "3" "4" "5" "6" "7" "8" "9"
func RemoveColours(s string) string {
	for {
		i := strings.Index(s, "\003")
		if i == -1 {
			break
		}
		j := i + 1  // end of colour sequence
		c := -1     // comma position, if found
	L:
		for {
			// Who needs regex anyway.
			switch {
			case c != -1 && (j-c) > 2:
				break L
			case s[j] == ',':
				c = j
				j++
			case c == -1 && (j-i) > 2:
				break L
			case s[j] >= '0' && s[j] <= '9':
				j++
			default:
				break L
			}
		}
		s = s[:i] + s[j:]
	}
	return s
}

func RemoveFormatting(s string) string {
	return strings.Map(func(c int) int {
		switch c {
		case '\002', '\025':
			// \002 == bold, \025 == underline
			return -1
		}
		return c
	}, s)
}

// Does this string look like a URL to you?
// This should be fairly conservative, I hope:
//   s starts with http:// or https:// and contains no spaces
func LooksURLish(s string) bool {
	return ((strings.HasPrefix(s, "http://") ||
		strings.HasPrefix(s, "https://")) &&
		strings.Index(s, " ") == -1)
}

// Gratuitously stolen from pkg/rand, cos they aren't usable externally.
type lockedSource struct {
	sync.Mutex
	rand.Source
}

func (r *lockedSource) Int63() (n int64) {
	r.Lock()
	defer r.Unlock()
	return r.Source.Int63()
}

func (r *lockedSource) Seed(seed int64) {
	r.Lock()
	r.Source.Seed(seed)
	r.Unlock()
}

func NewRand(seed int64) *rand.Rand {
	return rand.New(&lockedSource{Source: rand.NewSource(seed)})
}
