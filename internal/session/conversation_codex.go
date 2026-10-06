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
	// codexEntryCap is the most directory entries one listing or lookup reads,
	// of every kind and at every level. The 500-file limit above counts rollouts
	// only, so it bounds work on a tidy tree and nothing on a hostile one.
	codexEntryCap = 10000
	// codexReadChunk is how many entries one ReadDir call returns.
	codexReadChunk = 256
)

var codexRolloutName = regexp.MustCompile(`^rollout-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}-([0-9a-f-]{36})\.jsonl$`)

// codexConversations lists the Codex conversations recorded for workDir, newest
// first. Like Conversations it never returns an error: every failure reads as
// "none here". Only line 1 of a rollout is ever read, and only the id leaves it.
func codexConversations(sessionsDir, workDir string) []Conversation {
	var out []Conversation
	examined := 0

	// A walk that runs out of its entry budget keeps what it has found.
	_ = walkCodexRollouts(sessionsDir, func(day string, e os.DirEntry, nameID string) bool {
		if examined >= codexScanLimit {
			return true
		}
		examined++

		path := filepath.Join(day, e.Name())
		if !codexRollout(sessionsDir, path) {
			return false
		}
		id, cwd, ok := readCodexMeta(path)
		if !ok || id != nameID || !isConversationID(id) || cwd != workDir {
			return false
		}
		info, err := e.Info()
		if err != nil {
			return false
		}
		out = append(out, Conversation{ID: id, Modified: info.ModTime()})
		return len(out) >= codexListLimit
	})
	return sortConversations(out)
}

// walkCodexRollouts visits the rollout files under sessionsDir, newest day
// first, and stops when visit returns true. Every directory entry read, of any
// kind, spends from one budget of codexEntryCap, so a sessions tree someone else
// can write to costs bounded memory and time; running out returns
// ErrDiscoveryBounds. Directories are read in chunks, never whole.
func walkCodexRollouts(sessionsDir string, visit func(day string, e os.DirEntry, nameID string) bool) error {
	budget := codexEntryCap
	years, err := codexDateDirs(sessionsDir, 4, &budget)
	if err != nil {
		return err
	}
	for _, y := range years {
		months, err := codexDateDirs(filepath.Join(sessionsDir, y), 2, &budget)
		if err != nil {
			return err
		}
		for _, mo := range months {
			days, err := codexDateDirs(filepath.Join(sessionsDir, y, mo), 2, &budget)
			if err != nil {
				return err
			}
			for _, d := range days {
				day := filepath.Join(sessionsDir, y, mo, d)
				files, err := codexDayFiles(day, &budget)
				if err != nil {
					return err
				}
				for _, e := range files {
					m := codexRolloutName.FindStringSubmatch(e.Name())
					if m != nil && visit(day, e, m[1]) {
						return nil
					}
				}
			}
		}
	}
	return nil
}

// readCodexDir reads dir in chunks of 256, spending one budget unit per entry
// and handing each to keep. An unreadable directory is an empty one.
func readCodexDir(dir string, budget *int, keep func(os.DirEntry)) error {
	f, err := os.Open(dir) //nolint:gosec // G304: dir is the sessions tree or a name this walker built from digits.
	if err != nil {
		return nil //nolint:nilerr // an unreadable directory has nothing to list
	}
	defer f.Close() //nolint:errcheck // read-only
	for {
		entries, err := f.ReadDir(codexReadChunk)
		for _, e := range entries {
			if *budget--; *budget < 0 {
				return ErrDiscoveryBounds
			}
			keep(e)
		}
		if err != nil {
			return nil //nolint:nilerr // io.EOF ends the directory, any other error ends the read
		}
	}
}

// codexDateDirs names the real directories under dir whose names are exactly
// width ASCII digits, descending. A symlinked directory has a Type that is not
// a directory, so it is left out.
func codexDateDirs(dir string, width int, budget *int) ([]string, error) {
	var names []string
	err := readCodexDir(dir, budget, func(e os.DirEntry) {
		if e.Type().IsDir() && asciiDigits(e.Name(), width) {
			names = append(names, e.Name())
		}
	})
	slices.SortFunc(names, func(a, b string) int { return strings.Compare(b, a) })
	return names, err
}

// codexDayFiles returns the regular files in a day directory that are named
// like a rollout, newest name first.
func codexDayFiles(dir string, budget *int) ([]os.DirEntry, error) {
	var files []os.DirEntry
	err := readCodexDir(dir, budget, func(e os.DirEntry) {
		if e.Type().IsRegular() && codexRolloutName.MatchString(e.Name()) {
			files = append(files, e)
		}
	})
	slices.SortFunc(files, func(a, b os.DirEntry) int { return strings.Compare(b.Name(), a.Name()) })
	return files, err
}

func asciiDigits(s string, width int) bool {
	if len(s) != width {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
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
	var matches []string
	err := walkCodexRollouts(sessionsDir, func(day string, e os.DirEntry, nameID string) bool {
		if nameID == id {
			matches = append(matches, filepath.Join(day, e.Name()))
		}
		return len(matches) > 1
	})
	if err != nil || len(matches) != 1 {
		return false
	}
	if !codexRollout(sessionsDir, matches[0]) {
		return false
	}
	metaID, cwd, ok := readCodexMeta(matches[0])
	return ok && metaID == id && cwd == workDir
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
