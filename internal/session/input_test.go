package session

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/internal/auth"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
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

// canary is text no error string, record or argv element may ever carry.
const typedCanary = "canary-7c1e-typed-text"

func TestTypeDeliversByBracketedPasteThenEnter(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	s, _ := mustCreate(t, f, f.request())
	before := len(f.tmux.Calls())

	if err := f.mgr.Type(context.Background(), *s, "line one\nline two", true); err != nil {
		t.Fatalf("Type() unexpected error: %v", err)
	}

	got := f.tmux.Calls()[before:]
	wantOps := []tmuxctl.Op{tmuxctl.OpPasteBracketed, tmuxctl.OpPasteBracketed, tmuxctl.OpSendKeys}
	if len(got) != len(wantOps) {
		t.Fatalf("Type() ran %d tmux commands, want %d: %v", len(got), len(wantOps), got)
	}
	for i, op := range wantOps {
		if got[i].Op != op {
			t.Errorf("command %d is %s, want %s", i, got[i].Op, op)
		}
	}
	if !bytes.Equal(got[0].Stdin, []byte("line one\nline two")) {
		t.Errorf("stdin = %q, want the text", got[0].Stdin)
	}
	if !slices.Contains(got[1].Argv, "-p") {
		t.Errorf("paste argv %q lacks -p; a plain paste submits line one early", got[1].Argv)
	}
	if last := got[2].Argv[len(got[2].Argv)-1]; last != "Enter" {
		t.Errorf("send-keys ends in %q, want Enter", last)
	}
	for i, c := range got {
		if slices.ContainsFunc(c.Argv, func(a string) bool { return strings.Contains(a, "line one") }) {
			t.Errorf("command %d put the text on the command line: %q", i, c.Argv)
		}
	}
}

func TestTypeWithoutSubmitPressesNothing(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	s, _ := mustCreate(t, f, f.request())
	before := len(f.tmux.Calls())

	if err := f.mgr.Type(context.Background(), *s, "draft", false); err != nil {
		t.Fatalf("Type() unexpected error: %v", err)
	}

	got := f.tmux.Calls()[before:]
	if len(got) != 2 {
		t.Fatalf("Type(submit=false) ran %d commands, want 2: %v", len(got), got)
	}
	for _, c := range got {
		if c.Op == tmuxctl.OpSendKeys {
			t.Errorf("Type(submit=false) pressed a key: %q", c.Argv)
		}
	}
}

func TestTypeRecordsTheDriving(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	s, _ := mustCreate(t, f, f.request())

	later := f.now.Add(30 * time.Minute)
	mgr := f.managerAt(t, f.store, later)

	if err := mgr.Type(context.Background(), *s, "hello", true); err != nil {
		t.Fatalf("Type() unexpected error: %v", err)
	}
	if got := mustStored(t, f, s.ID).LastActivity; !got.Equal(later) {
		t.Errorf("Type left the clock at %v, want %v", got, later)
	}

	if err := mgr.PressKey(context.Background(), *s, KeyTab); err != nil {
		t.Fatalf("PressKey() unexpected error: %v", err)
	}
	later2 := later.Add(time.Hour)
	if err := f.managerAt(t, f.store, later2).PressKey(context.Background(), *s, KeyTab); err != nil {
		t.Fatalf("PressKey() unexpected error: %v", err)
	}
	if got := mustStored(t, f, s.ID).LastActivity; !got.Equal(later2) {
		t.Errorf("PressKey left the clock at %v, want %v", got, later2)
	}
}

func TestTypeRefusesWhatItCannotDeliver(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	live, _ := mustCreate(t, f, f.request())
	dead := *live
	dead.State = StateDead

	cases := map[string]struct {
		session Session
		text    string
		want    error
	}{
		"empty text":          {*live, "", ErrEmptyPrompt},
		"too long":            {*live, strings.Repeat("a", MaxTypeBytes+1), ErrInputTooLong},
		"control character":   {*live, "a\x1b[201~b", ErrInputInvalid},
		"a dead session":      {dead, "hello", ErrSessionDead},
		"a record with no id": {Session{Owner: auth.CallerOperator, State: StateStarting}, "hello", ErrSessionNotFound},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			before := len(f.tmux.Calls())
			stored := mustStored(t, f, live.ID)

			if err := f.mgr.Type(context.Background(), c.session, c.text, true); !errors.Is(err, c.want) {
				t.Fatalf("Type() = %v, want %v", err, c.want)
			}
			if after := len(f.tmux.Calls()); after != before {
				t.Errorf("the refused Type ran %v; a refusal must cost no tmux command", f.tmux.Calls()[before:after])
			}
			if after := mustStored(t, f, live.ID); after != stored {
				t.Errorf("the refused Type left %+v, want %+v unchanged", after, stored)
			}
		})
	}
}

func TestTypeNamesNoTextInItsError(t *testing.T) {
	t.Parallel()

	for _, op := range []tmuxctl.Op{tmuxctl.OpPasteBracketed, tmuxctl.OpSendKeys} {
		t.Run(string(op), func(t *testing.T) {
			t.Parallel()

			f := newManagerFixture(t)
			s, _ := mustCreate(t, f, f.request())
			boom := errors.New("boom")
			f.tmux.FailOp(op, boom)
			before := len(f.tmux.Calls())

			err := f.mgr.Type(context.Background(), *s, typedCanary, true)
			if !errors.Is(err, boom) {
				t.Fatalf("Type() = %v, want it to wrap the tmux failure", err)
			}
			if strings.Contains(err.Error(), typedCanary) {
				t.Errorf("the error names the typed text: %v", err)
			}
			if op == tmuxctl.OpPasteBracketed {
				for _, c := range f.tmux.Calls()[before:] {
					if c.Op == tmuxctl.OpSendKeys {
						t.Errorf("Enter was sent after a failed paste: %q", c.Argv)
					}
				}
			}
		})
	}
}

func TestPressKeySendsTheMappedConstant(t *testing.T) {
	t.Parallel()

	for _, k := range Keys() {
		t.Run(string(k), func(t *testing.T) {
			t.Parallel()

			f := newManagerFixture(t)
			s, _ := mustCreate(t, f, f.request())
			before := len(f.tmux.Calls())

			if err := f.mgr.PressKey(context.Background(), *s, k); err != nil {
				t.Fatalf("PressKey(%q) unexpected error: %v", k, err)
			}
			got := f.tmux.Calls()[before:]
			if len(got) != 1 || got[0].Op != tmuxctl.OpSendKeys {
				t.Fatalf("PressKey(%q) ran %v, want one send-keys", k, got)
			}
			if last := got[0].Argv[len(got[0].Argv)-1]; last != tmuxKeys[k] {
				t.Errorf("PressKey(%q) sent %q, want %q", k, last, tmuxKeys[k])
			}
		})
	}
}

func TestPressKeyRefusesAnUnknownKey(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	s, _ := mustCreate(t, f, f.request())
	before := len(f.tmux.Calls())
	stored := mustStored(t, f, s.ID)

	for _, k := range []Key{"", "Enter", "C-c", "f1"} {
		if err := f.mgr.PressKey(context.Background(), *s, k); !errors.Is(err, ErrUnknownKey) {
			t.Errorf("PressKey(%q) = %v, want ErrUnknownKey", k, err)
		}
	}
	if after := len(f.tmux.Calls()); after != before {
		t.Errorf("a refused key ran %v", f.tmux.Calls()[before:after])
	}
	if after := mustStored(t, f, s.ID); after != stored {
		t.Errorf("a refused key moved the record: %+v, want %+v", after, stored)
	}
}
