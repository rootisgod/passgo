// utils.go - Utility functions
package main

import (
	"math/rand"
	"unicode/utf8"
)

// truncateToRunes truncates s to at most maxRunes runes, appending "…" if truncated.
// Avoids slicing UTF-8 strings by byte index (which can cut mid-rune).
func truncateToRunes(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	n := 0
	for i := range s {
		if n == maxRunes {
			return s[:i] + "…"
		}
		n++
	}
	return s + "…"
}

// randomString generates random VM names like "VM-a1b2"
func randomString(length int) string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
