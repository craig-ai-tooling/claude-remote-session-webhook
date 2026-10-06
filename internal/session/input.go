package session

import (
	"errors"
	"unicode/utf8"
)

// Key is a symbolic key name from the closed set the session page offers.
type Key string

const (
	KeyEnter     Key = "enter"
	KeyEscape    Key = "escape"
	KeyInterrupt Key = "interrupt"
	KeyTab       Key = "tab"
	KeyBackTab   Key = "backtab"
	KeyUp        Key = "up"
	KeyDown      Key = "down"
	KeyLeft      Key = "left"
	KeyRight     Key = "right"
	KeyPageUp    Key = "pageup"
	KeyPageDown  Key = "pagedown"
	KeyBackspace Key = "backspace"
)

// tmuxKeys is the whole allowlist. Each mapping was measured against tmux 3.4
// (research.md R3). It is unexported so that a tmux key name is reachable only
// through PressKey, never from a caller's string.
var tmuxKeys = map[Key]string{
	KeyEnter: "Enter", KeyEscape: "Escape", KeyInterrupt: "C-c", KeyTab: "Tab",
	KeyBackTab: "BTab", KeyUp: "Up", KeyDown: "Down", KeyLeft: "Left",
	KeyRight: "Right", KeyPageUp: "PageUp", KeyPageDown: "PageDown", KeyBackspace: "BSpace",
}

// Keys returns the allowlist in key-bar order.
func Keys() []Key {
	return []Key{
		KeyEnter, KeyEscape, KeyInterrupt, KeyTab, KeyBackTab, KeyUp,
		KeyDown, KeyLeft, KeyRight, KeyPageUp, KeyPageDown, KeyBackspace,
	}
}

// ParseKey maps a caller's string to a Key by exact, case-sensitive lookup.
// Trimming or folding here would widen a closed set into a fuzzy one.
func ParseKey(name string) (Key, error) {
	k := Key(name)
	if _, ok := tmuxKeys[k]; !ok {
		return "", ErrUnknownKey
	}
	return k, nil
}

// MaxTypeBytes bounds one typed message (spec FR-004).
const MaxTypeBytes = 16384

var (
	ErrUnknownKey   = errors.New("that is not a key the session page offers")
	ErrInputTooLong = errors.New("typed text is longer than the bound")
	ErrInputInvalid = errors.New("typed text holds a control character or is not UTF-8")
)

// ValidateTyped applies FR-004 to text the caller has already CRLF-normalised.
// The control-character refusal is what keeps a pasted "\x1b[201~" from ending
// the bracketed paste early and letting the remainder run as keystrokes.
func ValidateTyped(text string) error {
	if text == "" {
		return ErrEmptyPrompt
	}
	if len(text) > MaxTypeBytes {
		return ErrInputTooLong
	}
	if !utf8.ValidString(text) {
		return ErrInputInvalid
	}
	for i := 0; i < len(text); i++ {
		b := text[i]
		if (b < 0x20 && b != '\t' && b != '\n') || b == 0x7f {
			return ErrInputInvalid
		}
	}
	return nil
}
