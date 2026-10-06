package session

import (
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// discover.go finds the Codex conversation a pane is running, from /proc.
//
// Codex cannot be handed an id at start, and no rollout file exists until the
// first prompt. After that the native codex process holds its rollout open, so
// the link under /proc/<pid>/fd names the conversation (research.md M6, D7).
// The walk is read-only and never opens a rollout file. Every read is bounded:
// a pane whose process tree is larger than the caps is an error, not a partial
// answer, so no id is recorded from a walk that did not finish.

var (
	ErrAmbiguousConversation = errors.New("more than one Codex conversation is open under this pane")
	ErrDiscoveryBounds       = errors.New("the process tree under this pane exceeds what discovery will read")
)

const (
	discoverMaxDepth     = 6
	discoverMaxProcs     = 64
	discoverMaxFDs       = 4096     // per process
	discoverChildrenRead = 64 << 10 // bytes read from one children file
)

// DiscoverCodexConversation returns the conversation id of the Codex rollout held open by
// panePID or a descendant, "" when none is open.
func DiscoverCodexConversation(procRoot string, panePID int, sessionsDir string) (string, error) {
	if panePID <= 0 || sessionsDir == "" {
		return "", nil
	}
	// /proc/<pid>/fd links name the resolved path of an open file, so a matcher
	// built from a symlinked $CODEX_HOME would never match. A directory that
	// cannot be resolved is a sweep that records nothing.
	resolved, err := filepath.EvalSymlinks(sessionsDir)
	if err != nil {
		return "", nil //nolint:nilerr // no sessions directory yet, or one that cannot be trusted: nothing to find
	}
	rollout := regexp.MustCompile(`^` + regexp.QuoteMeta(filepath.Clean(resolved)) +
		`/\d{4}/\d{2}/\d{2}/rollout-[^/]*-([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})\.jsonl$`)

	type node struct{ pid, depth int }
	seen := map[int]bool{panePID: true}
	queue := []node{{panePID, 0}}
	ids := map[string]bool{}
	for len(queue) > 0 {
		n := queue[0]
		queue = queue[1:]

		if n.depth < discoverMaxDepth {
			kids, err := discoverChildren(procRoot, n.pid)
			if err != nil {
				return "", err
			}
			for _, k := range kids {
				if seen[k] {
					continue
				}
				if len(seen) >= discoverMaxProcs {
					return "", ErrDiscoveryBounds
				}
				seen[k] = true
				queue = append(queue, node{k, n.depth + 1})
			}
		}

		if err := discoverRollouts(procRoot, n.pid, rollout, ids); err != nil {
			return "", err
		}
	}

	switch len(ids) {
	case 0:
		return "", nil
	case 1:
		for id := range ids {
			return id, nil
		}
	}
	return "", ErrAmbiguousConversation
}

// discoverChildren reads one pid's children file. A missing file is no children.
func discoverChildren(procRoot string, pid int) ([]int, error) {
	p := filepath.Join(procRoot, strconv.Itoa(pid), "task", strconv.Itoa(pid), "children")
	f, err := os.Open(p) //nolint:gosec // path is built from an integer pid under the proc root
	if err != nil {
		return nil, nil //nolint:nilerr // an exited or unreadable process has no children to follow
	}
	defer f.Close() //nolint:errcheck // read-only
	b, err := io.ReadAll(io.LimitReader(f, discoverChildrenRead+1))
	if err != nil {
		return nil, nil //nolint:nilerr // same as an open error
	}
	if len(b) > discoverChildrenRead {
		return nil, ErrDiscoveryBounds
	}
	var kids []int
	for _, field := range strings.Fields(string(b)) {
		k, err := strconv.Atoi(field)
		if err != nil || k <= 0 {
			continue
		}
		kids = append(kids, k)
	}
	return kids, nil
}

// discoverRollouts adds the id of every rollout link under pid's fd directory to ids.
func discoverRollouts(procRoot string, pid int, rollout *regexp.Regexp, ids map[string]bool) error {
	dir := filepath.Join(procRoot, strconv.Itoa(pid), "fd")
	f, err := os.Open(dir) //nolint:gosec // path is built from an integer pid under the proc root
	if err != nil {
		return nil //nolint:nilerr // a process that cannot be read is skipped
	}
	defer f.Close() //nolint:errcheck // read-only
	entries, err := f.ReadDir(discoverMaxFDs + 1)
	if err != nil && !errors.Is(err, io.EOF) {
		return nil //nolint:nilerr // same as an open error
	}
	if len(entries) > discoverMaxFDs {
		return fmt.Errorf("pid %d: %w", pid, ErrDiscoveryBounds)
	}
	for _, e := range entries {
		target, err := os.Readlink(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		if m := rollout.FindStringSubmatch(target); m != nil {
			ids[m[1]] = true
		}
	}
	return nil
}
