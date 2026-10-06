package session

import (
	"errors"
	"strings"
	"testing"
)

func TestParseKeyAcceptsExactlyTheAllowlist(t *testing.T) {
	t.Parallel()

	for _, k := range Keys() {
		got, err := ParseKey(string(k))
		if err != nil || got != k {
			t.Errorf("ParseKey(%q) = %q, %v; want it back", k, got, err)
		}
	}

	for _, in := range []string{"Escape", " enter", "", "enter ", "ENTER", "C-c", "Enter", "f1"} {
		if _, err := ParseKey(in); !errors.Is(err, ErrUnknownKey) {
			t.Errorf("ParseKey(%q) err = %v; want ErrUnknownKey", in, err)
		}
	}
}

func TestKeysMapToMeasuredTmuxNames(t *testing.T) {
	t.Parallel()

	want := map[Key]string{
		"enter": "Enter", "escape": "Escape", "interrupt": "C-c", "tab": "Tab",
		"backtab": "BTab", "up": "Up", "down": "Down", "left": "Left",
		"right": "Right", "pageup": "PageUp", "pagedown": "PageDown", "backspace": "BSpace",
	}
	if len(tmuxKeys) != len(want) {
		t.Fatalf("tmuxKeys has %d entries; want %d", len(tmuxKeys), len(want))
	}
	for k, name := range want {
		if tmuxKeys[k] != name {
			t.Errorf("tmuxKeys[%q] = %q; want %q", k, tmuxKeys[k], name)
		}
	}
	if len(Keys()) != len(want) {
		t.Errorf("Keys() has %d entries; want %d", len(Keys()), len(want))
	}
}

func TestValidateTyped(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		in   string
		want error
	}{
		{"empty", "", ErrEmptyPrompt},
		{"plain", "hello", nil},
		{"tab and newline", "a\tb\nc", nil},
		{"multibyte", "é", nil},
		{"exactly the bound", strings.Repeat("a", MaxTypeBytes), nil},
		{"one past the bound", strings.Repeat("a", MaxTypeBytes+1), ErrInputTooLong},
		{"carriage return", "\r", ErrInputInvalid},
		{"paste terminator", "\x1b[201~", ErrInputInvalid},
		{"invalid utf-8", "\xff", ErrInputInvalid},
		{"nul", "a\x00b", ErrInputInvalid},
		{"delete", "a\x7fb", ErrInputInvalid},
		{"too long and invalid reports length", strings.Repeat("\xff", MaxTypeBytes+1), ErrInputTooLong},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			err := ValidateTyped(tc.in)
			if tc.want == nil {
				if err != nil {
					t.Fatalf("ValidateTyped = %v; want nil", err)
				}
				return
			}
			if !errors.Is(err, tc.want) {
				t.Fatalf("ValidateTyped = %v; want %v", err, tc.want)
			}
		})
	}
}
