package config

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// TestResolveBinary is the daemon's answer to "where is claude", asked of a host
// where the systemd user manager's PATH has no ~/.local/bin and the login
// shell's does.
//
// **Must fail when** the lookup stops at the daemon's own PATH, which is what
// left the sign-in pill at "auth: unknown".
func TestResolveBinary(t *testing.T) {
	t.Parallel()

	localBin := t.TempDir()
	claude := filepath.Join(localBin, "claude")
	if err := os.WriteFile(claude, []byte("#!/bin/sh\n"), 0o755); err != nil { //nolint:gosec // G306: stands in for an installed program and must be executable.
		t.Fatal(err)
	}
	notExec := t.TempDir()
	if err := os.WriteFile(filepath.Join(notExec, "claude"), []byte("data"), 0o644); err != nil { //nolint:gosec // G306: the point is a file that is readable but not executable.
		t.Fatal(err)
	}

	missing := func(string) (string, error) { return "", exec.ErrNotFound }
	shellErr := errors.New("no shell")

	tests := map[string]struct {
		lookPath func(string) (string, error)
		shell    func() (string, error)
		want     string
		wantErr  bool
	}{
		"the daemon's own PATH wins and the shell is never asked": {
			lookPath: func(string) (string, error) { return "/usr/bin/claude", nil },
			shell:    func() (string, error) { t.Error("login shell asked although LookPath answered"); return "", nil },
			want:     "/usr/bin/claude",
		},
		"found only on the login shell's PATH": {
			lookPath: missing,
			shell:    func() (string, error) { return "/usr/bin:" + localBin, nil },
			want:     claude,
		},
		"on neither": {
			lookPath: missing,
			shell:    func() (string, error) { return "/usr/bin:/bin", nil },
			wantErr:  true,
		},
		"a file without an execute bit is not a program": {
			lookPath: missing,
			shell:    func() (string, error) { return notExec, nil },
			wantErr:  true,
		},
		"the login shell cannot be asked": {
			lookPath: missing,
			shell:    func() (string, error) { return "", shellErr },
			wantErr:  true,
		},
		"an empty PATH element is not the working directory": {
			lookPath: missing,
			shell:    func() (string, error) { return ":", nil },
			wantErr:  true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			got, err := resolveBinary("claude", tc.lookPath, tc.shell)
			if (err != nil) != tc.wantErr {
				t.Fatalf("resolveBinary error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("resolveBinary = %q, want %q", got, tc.want)
			}
		})
	}
}
