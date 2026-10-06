package tmuxctl

import "strings"

// Three more exports for the second Controller (spec 017, kubernetes mode), for
// the reason argv.go gives: it lives in another package and these are
// unexported here on purpose.
//
// ParseSessions and NewBufferName are one call each and add nothing. Absent is
// the one with a body, and it is the same answer Exec.Has and noServer give,
// keyed on an exit code instead of an error: a remote exec reports the code of
// the process it ran, not an *exec.ExitError.

// ParseSessions reads the output of the argv ArgvList returns.
func ParseSessions(stdout string) ([]SessionInfo, error) { return parseSessions(stdout) }

// NewBufferName returns a fresh paste-buffer name behind BufferPrefix.
func NewBufferName() (string, error) { return newBufferName() }

// Absent reports whether a tmux command that exited with exitCode said that the
// session, or the whole server, is not there. Exit 1 alone is not enough, for
// the reason msgNoSession's comment gives.
func Absent(exitCode int, stderr string) bool {
	if exitCode != 1 {
		return false
	}
	if strings.Contains(stderr, msgNoSession) || strings.Contains(stderr, msgNoServer) {
		return true
	}
	return strings.Contains(stderr, msgNoSocket) && strings.Contains(stderr, msgNoSuchFile)
}
