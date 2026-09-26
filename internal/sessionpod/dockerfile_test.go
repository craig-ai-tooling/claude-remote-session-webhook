package sessionpod

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

const dockerfilePath = "../../deploy/session-image/Dockerfile"

// instructions returns the Dockerfile's instructions with comments removed and
// continuation lines joined, keyed in order of appearance. Only what these
// assertions need: a line is an instruction word and its arguments.
func instructions(t *testing.T) (all []string, text string) {
	t.Helper()

	raw, err := os.ReadFile(dockerfilePath)
	if err != nil {
		t.Fatalf("read %s: %v", dockerfilePath, err)
	}
	var current string
	for _, line := range strings.Split(string(raw), "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if rest, ok := strings.CutSuffix(trimmed, "\\"); ok {
			current += rest + " "
			continue
		}
		all = append(all, strings.TrimSpace(current+trimmed))
		current = ""
	}
	return all, string(raw)
}

// TestSessionImageDockerfileShape pins what the image is for, because none of it
// is reachable from a Go test any other way and each is a way to ship a session
// pod that is wrong without anything failing: not amd64, running as root, holding
// a credential, or starting the base image's ralph loop.
func TestSessionImageDockerfileShape(t *testing.T) {
	t.Parallel()

	steps, raw := instructions(t)

	byWord := func(word string) []string {
		var out []string
		for _, s := range steps {
			if strings.HasPrefix(strings.ToUpper(s), word+" ") || strings.EqualFold(s, word) {
				out = append(out, s)
			}
		}
		return out
	}

	if from := byWord("FROM"); len(from) != 1 || from[0] != "FROM --platform=linux/amd64 docker.io/nctiggy/ralph-runner:2.1.246-ci10" {
		t.Errorf("FROM = %q, want the one amd64-pinned ralph-runner 2.1.246-ci10 line", from)
	}
	if user := byWord("USER"); len(user) == 0 || user[len(user)-1] != "USER 10001:10001" {
		t.Errorf("USER = %q, want the last to be numeric 10001:10001", user)
	}
	if copies := byWord("COPY"); len(copies) != 1 || copies[0] != "COPY --chmod=0755 crswd /usr/local/bin/crswd" {
		t.Errorf("COPY = %q, want exactly the one binary", copies)
	}
	if adds := byWord("ADD"); len(adds) != 0 {
		t.Errorf("ADD = %q; this image copies a file and fetches nothing", adds)
	}
	if run := byWord("RUN"); len(run) != 0 {
		t.Errorf("RUN = %q; the base image already has the user, tmux and claude", run)
	}
	if entry := byWord("ENTRYPOINT"); len(entry) != 1 || !strings.Contains(entry[0], "/usr/local/bin/crswd") {
		t.Errorf("ENTRYPOINT = %q, want tini around the crswd binary", entry)
	}
	if cmd := byWord("CMD"); len(cmd) != 1 || cmd[0] != "CMD []" {
		t.Errorf("CMD = %q, want CMD [] so the base image's ralph loop never starts", cmd)
	}

	// Nothing that bakes a credential, in the instructions and not the comments
	// (which say "no ARG, no ENV" on purpose).
	sensitive := regexp.MustCompile(`(?i)token|secret|password|credential|api_?key|\.claude`)
	for _, s := range steps {
		if word, _, _ := strings.Cut(s, " "); strings.EqualFold(word, "ENV") || strings.EqualFold(word, "ARG") || sensitive.MatchString(s) {
			t.Errorf("instruction %q sets a variable or names something credential-shaped", s)
		}
	}

	// The tag scheme the brief fixes is written down where the person building
	// the image will read it.
	if !strings.Contains(raw, "docker.io/nctiggy/crswd-session:2.1.246-<sha7>") {
		t.Error("the Dockerfile does not document the tag scheme docker.io/nctiggy/crswd-session:2.1.246-<sha7>")
	}
}
