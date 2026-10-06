package session

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

// trust_codex.go is trust.go for Codex: it records a session's working
// directory as trusted in <codex home>/config.toml before the start command is
// typed, so Codex does not stop on its "Do you trust the contents of this
// directory?" prompt with nobody at the keyboard.
//
// The root module has no dependencies (docs/security.md §5), so there is no TOML
// library. The edit understands exactly the shape Codex writes itself, a
// [projects."<dir>"] header with a trust_level line under it, and refuses any
// other file that mentions the directory rather than guess at it. The lock file,
// the atomic replace and the mode handling are trust.go's.

var (
	// ErrCodexConfigShape means the config mentions the directory in a form this
	// file does not edit: an inline table, a dotted key, a comment, a duplicate.
	ErrCodexConfigShape = errors.New("the Codex config names this directory in a shape crswd does not edit")
	// ErrUntrustablePath means the directory holds a byte that cannot sit inside
	// a TOML basic-string table header without escaping.
	ErrUntrustablePath = errors.New("the directory cannot be written into a Codex config table header")
)

const codexTrustLine = `trust_level = "trusted"`

var (
	trustedLine   = regexp.MustCompile(`^\s*trust_level\s*=\s*"trusted"\s*(#.*)?$`)
	trustLevelKey = regexp.MustCompile(`^\s*trust_level\s*=`)
	// projectsHeader matches a [projects...] table header in any spelling.
	projectsHeader = regexp.MustCompile(`^\s*\[\s*projects\s*\.`)
)

// CodexHome is where Codex keeps its config for a process with this
// environment: $CODEX_HOME when that is absolute, else $HOME/.codex when HOME
// is absolute. It returns "" when neither is.
func CodexHome(env []string) string {
	lookup := func(key string) string {
		v := ""
		for _, kv := range env {
			if k, val, ok := strings.Cut(kv, "="); ok && k == key {
				v = val
			}
		}
		return strings.TrimSpace(v)
	}
	if dir := lookup("CODEX_HOME"); filepath.IsAbs(dir) {
		return dir
	}
	if home := lookup("HOME"); filepath.IsAbs(home) {
		return filepath.Join(home, ".codex")
	}
	return ""
}

// SeedCodexTrust records dir as trusted in <codexHome>/config.toml. An empty
// home, a home that does not exist (Codex has never run) or a directory already
// trusted writes nothing. A missing config.toml under an existing home is
// created, because Codex's own acceptance creates the same table in that file.
func SeedCodexTrust(codexHome, dir string) (err error) {
	if codexHome == "" {
		return nil
	}
	if _, err := os.Stat(codexHome); errors.Is(err, fs.ErrNotExist) {
		return nil
	} else if err != nil {
		return fmt.Errorf("seed codex trust: stat %s: %w", codexHome, err)
	}
	if !filepath.IsAbs(dir) {
		return fmt.Errorf("seed codex trust: %q is not an absolute directory", dir)
	}
	if !trustablePath(dir) {
		return fmt.Errorf("seed codex trust: %w", ErrUntrustablePath)
	}

	path := filepath.Join(codexHome, "config.toml")
	// Unlocked first, as SeedTrust does: most creates find the directory trusted
	// and write nothing, and Codex replaces the file by rename.
	raw, err := readCodexConfig(path)
	if err != nil {
		return err
	}
	if _, changed, err := withCodexTrust(raw, dir); err != nil {
		return fmt.Errorf("seed codex trust: %s: %w", path, err)
	} else if !changed {
		return nil
	}

	lock, err := os.OpenFile(path+".lock", os.O_CREATE|os.O_RDWR, 0o600) //nolint:gosec // G304: path is Codex's config file, resolved from the daemon's own environment, never from a request.
	if err != nil {
		return fmt.Errorf("seed codex trust: open lock: %w", err)
	}
	// Closing the descriptor releases the flock.
	defer func() { err = errors.Join(err, lock.Close()) }()
	if err := syscall.Flock(int(lock.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("seed codex trust: lock %s: %w", path, err)
	}

	mode := fs.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	} else if !errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("seed codex trust: stat %s: %w", path, err)
	}
	raw, err = readCodexConfig(path)
	if err != nil {
		return err
	}
	updated, changed, err := withCodexTrust(raw, dir)
	if err != nil {
		return fmt.Errorf("seed codex trust: %s: %w", path, err)
	}
	if !changed {
		return nil
	}
	return replaceFile(path, updated, mode, ".config.toml.crswd-*")
}

// readCodexConfig reads path, treating a missing file as an empty one.
func readCodexConfig(path string) ([]byte, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // G304: path is Codex's config file, resolved from the daemon's own environment, never from a request.
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("seed codex trust: read %s: %w", path, err)
	}
	return raw, nil
}

// trustablePath reports whether dir can sit inside a TOML basic string as is.
func trustablePath(dir string) bool {
	for i := 0; i < len(dir); i++ {
		if c := dir[i]; c == '"' || c == '\\' || c < 0x20 || c == 0x7f {
			return false
		}
	}
	return true
}

// lineEnding returns the ending of line: "\r\n", "\n", or "" on a final
// unterminated line.
func lineEnding(line string) string {
	switch {
	case strings.HasSuffix(line, "\r\n"):
		return "\r\n"
	case strings.HasSuffix(line, "\n"):
		return "\n"
	}
	return ""
}

// withCodexTrust returns raw with dir trusted. It rewrites only the
// trust_level line under dir's table header, adds that line when the table has
// none, or appends the table when the file has no header for dir. Every other
// byte comes back as it went in. changed is false when dir is already trusted.
func withCodexTrust(raw []byte, dir string) ([]byte, bool, error) {
	if !trustablePath(dir) {
		return nil, false, ErrUntrustablePath
	}
	header := `[projects."` + dir + `"]`
	lines := strings.SplitAfter(string(raw), "\n")
	if n := len(lines); n > 0 && lines[n-1] == "" {
		lines = lines[:n-1]
	}

	headerAt := -1
	for i, line := range lines {
		body := strings.TrimRight(line, "\r\n")
		if strings.TrimSpace(body) == header {
			if headerAt >= 0 {
				return nil, false, ErrCodexConfigShape
			}
			headerAt = i
			continue
		}
		if strings.Contains(body, dir) {
			return nil, false, ErrCodexConfigShape
		}
		// An escaped header can name dir without spelling it ("\u0072epo"), so a
		// second table would be written for the same directory. TOML's escapes
		// are not decoded here; a projects header holding one is refused.
		if projectsHeader.MatchString(body) && strings.Contains(body, `\`) {
			return nil, false, ErrCodexConfigShape
		}
	}

	if headerAt < 0 {
		out := string(raw)
		if out != "" && !strings.HasSuffix(out, "\n") {
			out += "\n"
		}
		return []byte(out + "\n" + header + "\n" + codexTrustLine + "\n"), true, nil
	}

	for i := headerAt + 1; i < len(lines); i++ {
		body := strings.TrimRight(lines[i], "\r\n")
		if strings.HasPrefix(strings.TrimSpace(body), "[") {
			break
		}
		// A quoted key ("trust_level" = ...) is the same key spelled so the
		// bare-key match above cannot see it, and adding a bare one would make
		// the table invalid. Quoted keys in this table are refused outright.
		if t := strings.TrimSpace(body); strings.HasPrefix(t, `"`) || strings.HasPrefix(t, `'`) {
			return nil, false, ErrCodexConfigShape
		}
		if trustedLine.MatchString(body) {
			return raw, false, nil
		}
		if trustLevelKey.MatchString(body) {
			lines[i] = codexTrustLine + lineEnding(lines[i])
			return []byte(strings.Join(lines, "")), true, nil
		}
	}

	end := lineEnding(lines[headerAt])
	if end == "" {
		lines[headerAt] += "\n"
		end = "\n"
	}
	inserted := append([]string{}, lines[:headerAt+1]...)
	inserted = append(inserted, codexTrustLine+end)
	inserted = append(inserted, lines[headerAt+1:]...)
	return []byte(strings.Join(inserted, "")), true, nil
}
