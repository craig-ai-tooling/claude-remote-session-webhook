package session

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

const (
	// codexMetaReadLimit bounds the first line of a rollout. Codex writes its base
	// instructions into it, so 30 KB is ordinary; past this the file is not read.
	codexMetaReadLimit = 64 << 10
	// codexScanLimit bounds the files examined per listing, so a long history is
	// not walked on every render of the create form.
	codexScanLimit = 500
	codexListLimit = 50
)

var codexRolloutName = regexp.MustCompile(`^rollout-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}-([0-9a-f-]{36})\.jsonl$`)

// codexConversations lists the Codex conversations recorded for workDir, newest
// first. Like Conversations it never returns an error: every failure reads as
// "none here". Only line 1 of a rollout is ever read, and only the id leaves it.
func codexConversations(sessionsDir, workDir string) []Conversation {
	var out []Conversation
	examined := 0

	for _, y := range sortedSubdirs(sessionsDir) {
		for _, mo := range sortedSubdirs(filepath.Join(sessionsDir, y)) {
			for _, d := range sortedSubdirs(filepath.Join(sessionsDir, y, mo)) {
				day := filepath.Join(sessionsDir, y, mo, d)
				entries, err := os.ReadDir(day)
				if err != nil {
					continue
				}
				slices.SortFunc(entries, func(a, b os.DirEntry) int { return strings.Compare(b.Name(), a.Name()) })
				for _, e := range entries {
					if !e.Type().IsRegular() {
						continue
					}
					m := codexRolloutName.FindStringSubmatch(e.Name())
					if m == nil {
						continue
					}
					if examined >= codexScanLimit {
						return sortConversations(out)
					}
					examined++

					path := filepath.Join(day, e.Name())
					if !codexRollout(sessionsDir, path) {
						continue
					}
					id, cwd, ok := readCodexMeta(path)
					if !ok || id != m[1] || !isConversationID(id) || cwd != workDir {
						continue
					}
					info, err := e.Info()
					if err != nil {
						continue
					}
					out = append(out, Conversation{ID: id, Modified: info.ModTime()})
					if len(out) >= codexListLimit {
						return sortConversations(out)
					}
				}
			}
		}
	}
	return sortConversations(out)
}

// sortedSubdirs names the real directories under dir, descending. A symlinked
// directory has a Type that is not a directory, so it is left out.
func sortedSubdirs(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var names []string
	for _, e := range entries {
		if e.Type().IsDir() {
			names = append(names, e.Name())
		}
	}
	slices.SortFunc(names, func(a, b string) int { return strings.Compare(b, a) })
	return names
}

// sortConversations orders newest first, ties on identifier, as Conversations does.
func sortConversations(c []Conversation) []Conversation {
	slices.SortFunc(c, func(a, b Conversation) int {
		if !a.Modified.Equal(b.Modified) {
			if a.Modified.After(b.Modified) {
				return -1
			}
			return 1
		}
		return strings.Compare(a.ID, b.ID)
	})
	return c
}

// codexHasTranscript reports whether exactly one rollout carries id and it was
// recorded in workDir.
func codexHasTranscript(sessionsDir, id, workDir string) bool {
	if !isConversationID(id) {
		return false
	}
	matches, err := filepath.Glob(sessionsDir + "/*/*/*/rollout-*-" + id + ".jsonl")
	if err != nil || len(matches) != 1 {
		return false
	}
	if !codexRollout(sessionsDir, matches[0]) {
		return false
	}
	_, cwd, ok := readCodexMeta(matches[0])
	return ok && cwd == workDir
}

// readCodexMeta reads line 1 of a rollout, never more than codexMetaReadLimit bytes.
func readCodexMeta(path string) (id, cwd string, ok bool) {
	f, err := os.Open(path) //nolint:gosec // G304: path came from codexRollout, which proved it is a regular file inside the sessions tree.
	if err != nil {
		return "", "", false
	}
	defer f.Close() //nolint:errcheck // Read-only; a close failure says nothing a reader could act on.

	buf, err := io.ReadAll(io.LimitReader(f, codexMetaReadLimit))
	if err != nil {
		return "", "", false
	}
	line, _, found := bytes.Cut(buf, []byte("\n"))
	if !found {
		return "", "", false
	}
	var meta struct {
		Type    string `json:"type"`
		Payload struct {
			ID  string `json:"id"`
			Cwd string `json:"cwd"`
		} `json:"payload"`
	}
	if err := json.Unmarshal(line, &meta); err != nil || meta.Type != "session_meta" {
		return "", "", false
	}
	return meta.Payload.ID, meta.Payload.Cwd, true
}

// codexRollout reports whether path is a regular, non-symlink file whose resolved
// parent lies inside the resolved sessionsDir. Lstat is what refuses a symlink
// that points at another file inside the tree.
func codexRollout(sessionsDir, path string) bool {
	fi, err := os.Lstat(path)
	if err != nil || !fi.Mode().IsRegular() {
		return false
	}
	root, err := filepath.EvalSymlinks(sessionsDir)
	if err != nil {
		return false
	}
	dir, err := filepath.EvalSymlinks(filepath.Dir(path))
	if err != nil {
		return false
	}
	rel, err := filepath.Rel(root, dir)
	if err != nil {
		return false
	}
	_, ok := containedIn(root, filepath.Join(rel, filepath.Base(path)))
	return ok
}
