// Package codexauth recognises Codex's own sign-in screens in a pane.
//
// It is the Codex twin of internal/claudeauth and is deliberately a copy of its
// small helpers rather than an import: the two harnesses will change their
// wording independently, and a wording change must stay a one-package edit.
//
// Everything that knows what Codex's login looks like is behind DetectPrompt,
// covered by golden files in testdata. The panes are matched against a
// whitespace-flattened copy so a phrase means the same at any pane width.
package codexauth

import (
	"net/url"
	"regexp"
	"strings"
)

// Kind is which of Codex's sign-in screens a pane is showing.
type Kind string

const (
	// KindSignedOut is the TUI's start screen when no login is stored.
	KindSignedOut Kind = "signed-out"

	// KindDeviceCode is the screen carrying the sign-in link and a one-time code.
	KindDeviceCode Kind = "device-code"
)

// Prompt is what a pane showing a sign-in screen is asking for.
type Prompt struct {
	// Kind is which screen it is.
	Kind Kind

	// URL is the sign-in link, present only on KindDeviceCode and only when the
	// link was on screen.
	URL string

	// Code is the one-time device code. It is a short-lived credential: it must
	// never be logged, audited, stored or put in an error. String omits it so
	// that %v, %s and a wrapped error cannot be the way it leaks.
	Code string
}

// String describes a Prompt without disclosing the code or the link's path.
func (p Prompt) String() string {
	if u, err := url.Parse(p.URL); err == nil && u.Host != "" {
		return "codexauth.Prompt{" + string(p.Kind) + ", " + u.Host + "}"
	}
	return "codexauth.Prompt{" + string(p.Kind) + "}"
}

const (
	signedOutPhrase  = "Sign in with ChatGPT to use Codex as part of your paid plan"
	deviceCodePhrase = "Enter this one-time code"

	// urlPrefix is where the sign-in link starts, matched as a prefix because the
	// path is not this daemon's business.
	urlPrefix = "https://auth.openai.com/"
)

// codePattern is the shape of a device code: two upper-case alphanumeric
// groups joined by a hyphen.
var codePattern = regexp.MustCompile(`^[A-Z0-9]{4,}-[A-Z0-9]{4,}$`)

// DetectPrompt reports whether a pane is showing a Codex sign-in screen.
//
// Device-code wins when both phrases are present: the signed-out menu stays in
// scrollback after the operator picks the device-code option.
func DetectPrompt(pane string) (*Prompt, bool) {
	if pane == "" {
		return nil, false
	}

	flat := flatten(pane)

	if containsUnquoted(flat, deviceCodePhrase) {
		return &Prompt{Kind: KindDeviceCode, URL: signInURL(pane), Code: oneTimeCode(pane)}, true
	}
	if containsUnquoted(flat, signedOutPhrase) {
		return &Prompt{Kind: KindSignedOut}, true
	}
	return nil, false
}

// signInURL returns the first trimmed line that starts with the sign-in host, or
// "" when no link is on screen. The link is 36 characters, so it is not expected
// to wrap.
func signInURL(pane string) string {
	for _, line := range strings.Split(pane, "\n") {
		if trimmed := strings.TrimSpace(line); strings.HasPrefix(trimmed, urlPrefix) {
			return trimmed
		}
	}
	return ""
}

// oneTimeCode returns the first non-empty trimmed line after the one holding the
// device-code phrase, accepted only if it has a device code's shape.
//
// Quoted mentions of the phrase are skipped, so a pane that quotes it above the
// real screen still reads the code from the real one.
func oneTimeCode(pane string) string {
	lines := strings.Split(pane, "\n")
	for i, line := range lines {
		if !containsUnquoted(flatten(line), deviceCodePhrase) {
			continue
		}
		for _, next := range lines[i+1:] {
			trimmed := strings.TrimSpace(next)
			if trimmed == "" {
				continue
			}
			if codePattern.MatchString(trimmed) {
				return trimmed
			}
			return ""
		}
		return ""
	}
	return ""
}

// quotes are the characters that turn an anchor into a mention of an anchor.
// A session reading this repository shows this file's own phrases, and a working
// session must not read as needing sign-in because of it.
const quotes = "\"'`"

// containsUnquoted reports whether phrase appears at least once without a quote
// character immediately on either side of it.
func containsUnquoted(flat, phrase string) bool {
	for at := 0; ; {
		i := strings.Index(flat[at:], phrase)
		if i < 0 {
			return false
		}
		start := at + i
		end := start + len(phrase)

		beforeQuoted := start > 0 && strings.ContainsRune(quotes, rune(flat[start-1]))
		afterQuoted := end < len(flat) && strings.ContainsRune(quotes, rune(flat[end]))
		if !beforeQuoted && !afterQuoted {
			return true
		}
		at = start + 1
	}
}

// flatten collapses every run of whitespace to a single space.
func flatten(pane string) string {
	return strings.Join(strings.Fields(pane), " ")
}
