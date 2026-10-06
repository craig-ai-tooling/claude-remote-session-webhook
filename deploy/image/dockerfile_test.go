package image

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

const dockerfilePath = "Dockerfile"

// instructions returns the Dockerfile's instructions with comments removed and
// continuation lines joined, in order of appearance.
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

// TestCrswdImageDockerfileShape pins what the image is for: a static, non-root
// binary on a distroless base that fetches nothing and holds no credential. None
// of it is reachable from another test, and each is a way to ship a daemon image
// that is wrong while everything stays green.
func TestCrswdImageDockerfileShape(t *testing.T) {
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

	if from := byWord("FROM"); len(from) != 1 || !regexp.MustCompile(`^FROM gcr\.io/distroless/static-debian12:nonroot@sha256:[0-9a-f]{64}$`).MatchString(from[0]) {
		t.Errorf("FROM = %q, want the one distroless static nonroot line pinned tag@sha256:<digest>", from)
	}
	if copies := byWord("COPY"); len(copies) != 1 || copies[0] != "COPY --chmod=0755 crswd-${TARGETARCH} /usr/local/bin/crswd" {
		t.Errorf("COPY = %q, want exactly the one per-arch binary", copies)
	}
	for _, word := range []string{"RUN", "ADD", "ENV"} {
		if got := byWord(word); len(got) != 0 {
			t.Errorf("%s = %q; this image copies a binary and nothing else", word, got)
		}
	}
	if user := byWord("USER"); len(user) == 0 || user[len(user)-1] != "USER 65532:65532" {
		t.Errorf("USER = %q, want the last to be numeric 65532:65532", user)
	}
	if entry := byWord("ENTRYPOINT"); len(entry) != 1 || entry[0] != `ENTRYPOINT ["/usr/local/bin/crswd"]` {
		t.Errorf("ENTRYPOINT = %q, want the crswd binary alone", entry)
	}
	for _, s := range steps {
		if word, _, _ := strings.Cut(s, " "); strings.EqualFold(word, "ARG") && s != "ARG TARGETARCH" {
			t.Errorf("instruction %q is an ARG other than TARGETARCH", s)
		}
	}

	if !strings.Contains(raw, "docker.io/nctiggy/crswd:2.0.0-dev.<sha7>") {
		t.Error("the Dockerfile does not document the tag scheme docker.io/nctiggy/crswd:2.0.0-dev.<sha7>")
	}
}
