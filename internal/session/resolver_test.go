package session

import (
	"context"
	"errors"
	"path/filepath"
	"testing"

	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
)

// A daemon that cannot see the session's filesystem has no directory to resolve,
// so the resolver is the seam where it answers from the string alone.
func TestWorkDirResolverAdmitsAMissingDirUnderTheRoot(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	f.mgr.SetWorkDirResolver(LexicalWorkDir)

	req := f.request()
	req.WorkDir = filepath.Join(f.root, "absent")
	s, _, err := f.mgr.Create(context.Background(), req)
	if err != nil {
		t.Fatalf("Create() with a lexical resolver refused a missing dir under the root: %v", err)
	}
	if s == nil {
		t.Fatal("Create() returned no session alongside a nil error")
	}
}

func TestWorkDirResolverRefusesADirOutsideTheRoot(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	f.mgr.SetWorkDirResolver(LexicalWorkDir)

	req := f.request()
	req.WorkDir = filepath.Join(f.root, "..", "elsewhere")
	_, _, err := f.mgr.Create(context.Background(), req)
	if !errors.Is(err, ErrWorkDirOutsideRoots) {
		t.Fatalf("Create() error = %v, want one wrapping ErrWorkDirOutsideRoots", err)
	}
}

// Without a resolver the host's behaviour is untouched: a directory that does
// not exist is still refused, because EvalSymlinks has nothing to resolve.
func TestWorkDirResolverUnsetStillRefusesAMissingDir(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)

	req := f.request()
	req.WorkDir = filepath.Join(f.root, "absent")
	_, _, err := f.mgr.Create(context.Background(), req)
	if !errors.Is(err, ErrInvalidWorkDir) {
		t.Fatalf("Create() error = %v, want one wrapping ErrInvalidWorkDir", err)
	}
}

// The resolver must reach the manager's own roots, not a copy.
func TestWorkDirResolverReceivesTheManagersRoots(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	var got []config.ApprovedRoot
	f.mgr.SetWorkDirResolver(func(p string, roots []config.ApprovedRoot) (string, error) {
		got = roots
		return LexicalWorkDir(p, roots)
	})

	req := f.request()
	req.WorkDir = filepath.Join(f.root, "absent")
	if _, _, err := f.mgr.Create(context.Background(), req); err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}
	if len(got) == 0 || got[0].Path != f.roots()[0].Path {
		t.Errorf("resolver saw roots %v, want %v", got, f.roots())
	}
}
