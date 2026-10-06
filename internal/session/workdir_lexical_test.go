package session

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
)

func TestLexicalWorkDir(t *testing.T) {
	t.Parallel()

	// Nothing here exists on disk: the function under test must answer from the
	// string alone, which is the only thing a daemon that cannot see the
	// session's filesystem has.
	root := filepath.Join(string(filepath.Separator), "srv", "code")
	roots := []config.ApprovedRoot{{Path: root}}

	tests := []struct {
		name    string
		in      string
		roots   []config.ApprovedRoot
		want    string
		wantErr error
	}{
		{name: "empty path", in: "", roots: roots, wantErr: ErrInvalidWorkDir},
		{name: "relative path", in: "repo", roots: roots, wantErr: ErrWorkDirNotAbsolute},
		{name: "dotdot escape out of the root", in: root + "/repo/../..", roots: roots, wantErr: ErrWorkDirOutsideRoots},
		{name: "boundary lookalike of the root", in: root + "EVIL", roots: roots, wantErr: ErrWorkDirOutsideRoots},
		{name: "outside every root", in: "/etc", roots: roots, wantErr: ErrWorkDirOutsideRoots},
		{name: "no roots at all", in: root, roots: nil, wantErr: ErrWorkDirOutsideRoots},
		{name: "the root itself", in: root, roots: roots, want: root},
		{name: "non-existent dir under the root", in: root + "/absent/deeper", roots: roots, want: root + "/absent/deeper"},
		{name: "unclean path under the root is cleaned", in: root + "/a/../b/", roots: roots, want: root + "/b"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := LexicalWorkDir(tt.in, tt.roots)
			if tt.wantErr != nil {
				if !errors.Is(err, ErrInvalidWorkDir) || !errors.Is(err, tt.wantErr) {
					t.Fatalf("LexicalWorkDir(%q) error = %v, want one wrapping ErrInvalidWorkDir and %v", tt.in, err, tt.wantErr)
				}
				if got != "" {
					t.Errorf("LexicalWorkDir(%q) = %q alongside an error, want empty", tt.in, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("LexicalWorkDir(%q) error = %v, want nil", tt.in, err)
			}
			if got != tt.want {
				t.Errorf("LexicalWorkDir(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}

// The caller's path is attacker text on its way to an audit record, so a
// refusal must never carry it.
func TestLexicalWorkDirErrorOmitsThePath(t *testing.T) {
	t.Parallel()

	const probe = "/marker-xyzzy/../../plugh"
	_, err := LexicalWorkDir(probe, []config.ApprovedRoot{{Path: "/srv/code"}})
	if err == nil {
		t.Fatal("LexicalWorkDir admitted a path outside the roots")
	}
	for _, needle := range []string{"xyzzy", "plugh"} {
		if strings.Contains(err.Error(), needle) {
			t.Errorf("error %q carries part of the caller's path", err)
		}
	}
}

// UnderAnyRoot is exported so internal/admit and the pod share one containment
// rule with the host path rather than restating it.
func TestUnderAnyRootExported(t *testing.T) {
	t.Parallel()

	roots := []config.ApprovedRoot{{Path: "/srv/code"}}
	if !UnderAnyRoot("/srv/code/repo", roots) {
		t.Error("UnderAnyRoot refused a child of a root")
	}
	if UnderAnyRoot("/srv/codeEVIL", roots) {
		t.Error("UnderAnyRoot admitted a string-prefix lookalike")
	}
	if UnderAnyRoot("/srv/code", nil) {
		t.Error("UnderAnyRoot admitted a path against an empty list")
	}
}
