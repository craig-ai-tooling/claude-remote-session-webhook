package config

import (
	"errors"
	"fmt"
	"strings"

	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
)

// ErrCodexUpdateCheck is what a Codex start command earns by turning the
// startup update check on.
var ErrCodexUpdateCheck = errors.New("a Codex start command may not turn the startup update check on")

// ErrCodexQuoting is what a Codex start command earns by containing a quote or
// a backslash. The update-check guard reads the command split on whitespace; a
// shell would strip quotes and escapes first, so a quoted assignment reaches
// Codex as the one the guard missed. Refusing the characters is the rule, not
// parsing shell.
var ErrCodexQuoting = errors.New("a Codex start command may not contain a quote or a backslash")

const codexUpdateCheckKey = "check_for_update_on_startup"

// codexUpdateCheckValues returns every value the command assigns to
// check_for_update_on_startup before the first "--" token. Codex applies the
// last -c for a key, so every assignment matters, not just the first.
func codexUpdateCheckValues(command string) []string {
	tokens := strings.Fields(command)
	for i, tok := range tokens {
		if tok == "--" {
			tokens = tokens[:i]
			break
		}
	}

	var values []string
	for i, tok := range tokens {
		var assignment string
		switch {
		case tok == "-c" || tok == "--config":
			if i+1 >= len(tokens) {
				continue
			}
			assignment = tokens[i+1]
		case strings.HasPrefix(tok, "--config="):
			assignment = strings.TrimPrefix(tok, "--config=")
		case strings.HasPrefix(tok, "-c"):
			assignment = strings.TrimPrefix(tok, "-c")
		default:
			continue
		}
		key, value, found := strings.Cut(assignment, "=")
		if !found || key != codexUpdateCheckKey {
			continue
		}
		values = append(values, unquoteOnce(value))
	}
	return values
}

// unquoteOnce removes one pair of surrounding double or single quotes.
func unquoteOnce(s string) string {
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return s[1 : len(s)-1]
	}
	return s
}

// validateCodexUpdateCheck refuses a Codex command that would let the update
// menu open: a typed Enter on it runs an npm install.
func validateCodexUpdateCheck(variable, name, command string) error {
	if harness.Of(command) != harness.Codex {
		return nil
	}
	if strings.ContainsAny(command, "'\"\\") {
		return fmt.Errorf("%s: start command %q: %w; refusing to start", variable, name, ErrCodexQuoting)
	}
	for _, v := range codexUpdateCheckValues(command) {
		if v != "false" {
			return fmt.Errorf("%s: start command %q: %w; refusing to start", variable, name, ErrCodexUpdateCheck)
		}
	}
	return nil
}
