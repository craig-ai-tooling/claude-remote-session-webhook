package session

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

// trust.go pre-accepts Claude Code's workspace-trust dialog for a session's
// working directory, on disk, before the start command is typed.
//
// # Why this exists
//
// --dangerously-skip-permissions turns off tool-call prompts and nothing else.
// Workspace trust is a second gate, and a session started in a directory
// nobody ever trusted sits on "Is this a project you created or one you
// trust?" with no one at the keyboard. dialog.go can see that and the card says
// `blocked`, but seeing it is all the daemon could do: an operator still had to
// open a terminal and press a key, which is the one thing this daemon exists to
// remove. Found on 2026-10-05 on a session created in ~/code/ai-influencer.
//
// It is not typed into the dialog. The daemon never types into a working
// session (AGENTS.md), and a keystroke aimed at a TUI prompt is the race
// dialog.go's header describes. The decision lives in Claude Code's own config
// file, at projects[<dir>].hasTrustDialogAccepted, and Claude Code's own error
// for a headless session in an untrusted folder names that key as the fix.
//
// # Why it is safe to grant
//
// Only a directory ResolveWorkDir has already accepted reaches here, so the
// directory is inside the operator's allowed_roots. A session there runs with
// tool approval switched off: whatever a repo's hooks could do, the session
// could already do. Trusting the folder grants nothing allowed_roots did not.
//
// # How it writes
//
// The common case writes nothing: a directory already trusted is read and left
// alone. Otherwise the file is rewritten under the same flock ai-lawnmower's
// loop/trust-seed.sh takes (<file>.lock), through a temporary file and a rename,
// so a reader sees the old file or the new one and never half of either. Every
// other value is carried through as raw JSON. A missing file is left missing:
// Claude Code creates it on first run, and a file this daemon invented would be
// one Claude Code had never agreed to read.

// trustKey is Claude Code's own name for the persisted decision.
const trustKey = "hasTrustDialogAccepted"

// ClaudeConfigFile is where Claude Code keeps its per-project state for a
// process with this environment: $CLAUDE_CONFIG_DIR/.claude.json when that is
// set, $HOME/.claude.json otherwise. That is Claude Code's resolution, not this
// daemon's. It returns "" when neither is an absolute path.
func ClaudeConfigFile(env []string) string {
	lookup := func(key string) string {
		v := ""
		for _, kv := range env {
			if k, val, ok := strings.Cut(kv, "="); ok && k == key {
				v = val
			}
		}
		return strings.TrimSpace(v)
	}
	if dir := lookup("CLAUDE_CONFIG_DIR"); filepath.IsAbs(dir) {
		return filepath.Join(dir, ".claude.json")
	}
	if home := lookup("HOME"); filepath.IsAbs(home) {
		return filepath.Join(home, ".claude.json")
	}
	return ""
}

// SeedTrust records dir as trusted in the Claude Code config file at path. An
// empty path, a missing file, or a directory already trusted changes nothing.
func SeedTrust(path, dir string) (err error) {
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("seed workspace trust: %q is not an absolute directory", dir)
	}
	// Unlocked first, because nearly every create lands here and has nothing to
	// write. Reading without the lock is safe: Claude Code replaces the file by
	// rename, so this sees a whole file, and a stale answer only means the
	// locked pass below runs and finds the same thing.
	raw, err := os.ReadFile(path) //nolint:gosec // G304: path is Claude Code's config file, resolved from the daemon's own environment, never from a request.
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("seed workspace trust: read %s: %w", path, err)
	}
	if trusted, err := isTrusted(raw, dir); err != nil {
		return fmt.Errorf("seed workspace trust: %s: %w", path, err)
	} else if trusted {
		return nil
	}

	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // G304: path is Claude Code's config file, resolved from the daemon's own environment, never from a request.
	if err != nil {
		return fmt.Errorf("seed workspace trust: open lock: %w", err)
	}
	// Closing the descriptor releases the flock.
	defer func() { err = errors.Join(err, lock.Close()) }()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("seed workspace trust: lock %s: %w", path, err)
	}

	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("seed workspace trust: stat %s: %w", path, err)
	}
	raw, err = os.ReadFile(path) //nolint:gosec // G304: path is Claude Code's config file, resolved from the daemon's own environment, never from a request.
	if err != nil {
		return fmt.Errorf("seed workspace trust: read %s: %w", path, err)
	}
	updated, changed, err := withTrust(raw, dir)
	if err != nil {
		return fmt.Errorf("seed workspace trust: %s: %w", path, err)
	}
	if !changed {
		return nil
	}
	return replaceFile(path, updated, info.Mode().Perm(), ".claude.json.crswd-*")
}

// isTrusted reports whether raw already records dir as trusted.
func isTrusted(raw []byte, dir string) (bool, error) {
	var doc struct {
		Projects map[string]map[string]json.RawMessage `json:"projects"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		return false, fmt.Errorf("not a JSON object with a projects map: %w", err)
	}
	return bytes.Equal(bytes.TrimSpace(doc.Projects[dir][trustKey]), []byte("true")), nil
}

// withTrust returns raw with projects[dir].hasTrustDialogAccepted set to true.
// Values are decoded only as far as the path to that key; everything else is
// carried as raw JSON, so numbers, nesting and unknown fields come back as they
// went in.
func withTrust(raw []byte, dir string) ([]byte, bool, error) {
	var top map[string]json.RawMessage
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, false, fmt.Errorf("not a JSON object: %w", err)
	}
	if top == nil {
		return nil, false, errors.New("not a JSON object")
	}
	projects := map[string]json.RawMessage{}
	if p, ok := top["projects"]; ok && string(bytes.TrimSpace(p)) != "null" {
		if err := json.Unmarshal(p, &projects); err != nil {
			return nil, false, fmt.Errorf("projects is not an object: %w", err)
		}
	}
	entry := map[string]json.RawMessage{}
	if e, ok := projects[dir]; ok && string(bytes.TrimSpace(e)) != "null" {
		if err := json.Unmarshal(e, &entry); err != nil {
			return nil, false, fmt.Errorf("the entry for the session directory is not an object: %w", err)
		}
	}
	if bytes.Equal(bytes.TrimSpace(entry[trustKey]), []byte("true")) {
		return raw, false, nil
	}
	entry[trustKey] = json.RawMessage("true")

	var err error
	if projects[dir], err = json.Marshal(entry); err != nil {
		return nil, false, err
	}
	if top["projects"], err = json.Marshal(projects); err != nil {
		return nil, false, err
	}
	out, err := json.MarshalIndent(top, "", "  ")
	if err != nil {
		return nil, false, err
	}
	return append(out, '\n'), true, nil
}

// replaceFile writes data beside path and renames it over path, so the file is
// never seen half-written. A failure removes the temporary file and reports
// both errors if that fails too.
func replaceFile(path string, data []byte, mode fs.FileMode, prefix string) (err error) {
	tmp, err := os.CreateTemp(filepath.Dir(path), prefix)
	if err != nil {
		return fmt.Errorf("seed workspace trust: create temporary file: %w", err)
	}
	name := tmp.Name()
	closed := false
	defer func() {
		if err == nil {
			return
		}
		if !closed {
			err = errors.Join(err, tmp.Close())
		}
		if rmErr := os.Remove(name); rmErr != nil && !errors.Is(rmErr, fs.ErrNotExist) {
			err = errors.Join(err, rmErr)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		return fmt.Errorf("seed workspace trust: write temporary file: %w", err)
	}
	if err := tmp.Chmod(mode); err != nil {
		return fmt.Errorf("seed workspace trust: chmod temporary file: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		return fmt.Errorf("seed workspace trust: sync temporary file: %w", err)
	}
	closed = true
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("seed workspace trust: close temporary file: %w", err)
	}
	if err := os.Rename(name, path); err != nil {
		return fmt.Errorf("seed workspace trust: replace %s: %w", path, err)
	}
	return nil
}
