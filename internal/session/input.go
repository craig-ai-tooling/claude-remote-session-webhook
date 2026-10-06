package session

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"unicode/utf8"

	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
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

	// ErrSubmitFailed means the text reached the pane and the Enter after it
	// did not. It is not a failure to deliver, and a caller must not offer a
	// retry that types the text a second time (spec 018 review #4).
	ErrSubmitFailed = errors.New("the text was typed but not submitted")
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

// Type delivers operator text into a session by bracketed paste, then presses
// Enter when submit is true. The paste is bracketed for every session whatever
// it runs: research.md R1 measured that a plain paste submits line one early on
// Claude Code, so a per-runtime choice here would break multi-line text there.
//
// Input is never gated on what the pane shows, and the text is never audited,
// logged or named in an error, for the reason Prompt's is not.
func (m *Manager) Type(ctx context.Context, s Session, text string, submit bool) error {
	if err := guardDelivery(s); err != nil {
		return fmt.Errorf("type into session: %w", err)
	}
	if err := ValidateTyped(text); err != nil {
		return fmt.Errorf("type into session %s: %w", s.ID, err)
	}

	// Held across the paste and the Enter, so a key or another message cannot
	// land between them.
	unlock := m.lockSession(s.ID)
	defer unlock()

	if err := m.stillLive(s); err != nil {
		return fmt.Errorf("type into session %s: %w", s.ID, err)
	}
	if err := m.tmux.PasteBracketed(ctx, s.TmuxName(), []byte(text)); err != nil {
		return m.vanished(ctx, s, "type into", err)
	}
	// After the delivery, not before: a session that is not there must not look
	// driven in the fleet.
	if err := m.recordDriving(s); err != nil {
		return fmt.Errorf("type into session %s: %w", s.ID, err)
	}
	if submit {
		if err := m.tmux.SendKeys(ctx, s.TmuxName(), enterKey); err != nil {
			err = m.vanished(ctx, s, "submit typed text in", err)
			if errors.Is(err, ErrSessionDead) {
				return err
			}
			return fmt.Errorf("%w: %w", ErrSubmitFailed, err)
		}
	}
	return nil
}

// PressKey sends one key from the closed set. The tmux name is looked up here
// and nowhere a caller's string can reach.
func (m *Manager) PressKey(ctx context.Context, s Session, key Key) error {
	if err := guardDelivery(s); err != nil {
		return fmt.Errorf("press a key: %w", err)
	}
	name, ok := tmuxKeys[key]
	if !ok {
		return fmt.Errorf("press a key in session %s: %w", s.ID, ErrUnknownKey)
	}

	unlock := m.lockSession(s.ID)
	defer unlock()

	if err := m.stillLive(s); err != nil {
		return fmt.Errorf("press a key in session %s: %w", s.ID, err)
	}
	if err := m.tmux.SendKeys(ctx, s.TmuxName(), name); err != nil {
		return m.vanished(ctx, s, "press a key in", err)
	}
	if err := m.recordDriving(s); err != nil {
		return fmt.Errorf("press a key in session %s: %w", s.ID, err)
	}
	return nil
}

// lockSession takes the session's lifecycle mutex and returns its release.
//
// One mutex serves every operation that delivers into the pane or changes what
// is running in it or whether the record exists: Type, PressKey, Prompt,
// Compact, SetMode, Continue, Destroy and the supervisor's Codex discovery
// (019 core review #2 and #8). It is not reentrant. A method holding it must
// not call another public method that takes it; helpers it calls are the
// unlocked ones (vanished, sendStart, quitStepped, restartInto).
func (m *Manager) lockSession(id string) func() {
	mu, _ := m.inputLocks.LoadOrStore(id, &sync.Mutex{})
	l, ok := mu.(*sync.Mutex)
	if !ok {
		// Unreachable: the map is only ever given a *sync.Mutex.
		l = new(sync.Mutex)
	}
	l.Lock()
	return l.Unlock
}

// stillLive asks the store whether the record is still there, without moving
// its clock. recordDriving's Touch answers the same question but also counts as
// activity, which must wait until something has been delivered.
func (m *Manager) stillLive(s Session) error {
	_, err := m.store.Get(s.ID, s.Owner)
	return err
}

// History returns the session's tmux scrollback, excluding the visible screen,
// stripped. Reading is not driving, so the record is not touched (FR-011).
//
// A refusal for size (ErrHistoryTooLarge) is returned as it is: it is no
// evidence the window died, and treating it as such would let a long scrollback
// end a live session's card. Any other failure asks the host whether the
// session is still there, and a session that is not answers ErrSessionDead.
func (m *Manager) History(ctx context.Context, s Session) (Capture, error) {
	if err := guardDelivery(s); err != nil {
		return Capture{}, fmt.Errorf("capture history: %w", err)
	}

	text, err := m.tmux.CaptureHistory(ctx, s.TmuxName())
	if errors.Is(err, tmuxctl.ErrHistoryTooLarge) {
		return Capture{}, fmt.Errorf("capture history of session %s: %w", s.ID, err)
	}
	if err != nil {
		return Capture{}, m.vanished(ctx, s, "capture history of", err)
	}

	return Capture{Text: tmuxctl.Strip(text), At: m.clock.Now()}, nil
}

// guardDelivery is Compact's two guards: an empty ID would build the bare
// prefix as a tmux target, and a dead session's window is already gone.
func guardDelivery(s Session) error {
	if s.ID == "" {
		return ErrSessionNotFound
	}
	if s.State == StateDead {
		return ErrSessionDead
	}
	return nil
}

// recordDriving moves the clock before the bytes do, as Compact does: Touch is
// the store's own answer to whether the record is still live, so a session the
// reaper collected is refused before anything is delivered.
func (m *Manager) recordDriving(s Session) error {
	now := m.clock.Now()
	displayed := s.DisplayState(now)
	if err := m.store.Touch(s.ID, now); err != nil {
		return err
	}
	s.LastActivity = now
	if after := s.DisplayState(now); after != displayed {
		m.emit(FleetChanged, s)
	}
	return nil
}
