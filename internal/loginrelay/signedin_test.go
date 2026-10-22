package loginrelay

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
)

// TestSignedInRunsTheResolvedProgram holds that `auth status` is run from the
// path the resolver found and not from the bare name, which exec would look up
// on the daemon's own PATH, the one without ~/.local/bin.
//
// **Must fail when** SignedIn goes back to running r.binary directly: the fake
// claude lives in a directory on no PATH and is reachable only through resolve.
func TestSignedInRunsTheResolvedProgram(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	script := func(name, body string) string {
		p := filepath.Join(dir, name)
		if err := os.WriteFile(p, []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil { //nolint:gosec // G306: stands in for an installed program and must be executable.
			t.Fatal(err)
		}
		return p
	}
	signedIn := script("in-claude", `echo '{"loggedIn":true}'`)
	signedOut := script("out-claude", `echo '{"loggedIn":false}'; exit 1`)

	tests := map[string]struct {
		resolve func(string) (string, error)
		want    bool
		wantErr bool
	}{
		"signed in":  {resolve: func(string) (string, error) { return signedIn, nil }, want: true},
		"signed out": {resolve: func(string) (string, error) { return signedOut, nil }, want: false},
		"unresolvable": {
			resolve: func(string) (string, error) { return "", errors.New("not found") },
			wantErr: true,
		},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r, err := New(tmuxctl.NewFake(), "claude --dangerously-skip-permissions", t.TempDir(), []string{"HOME=" + t.TempDir()})
			if err != nil {
				t.Fatal(err)
			}
			var asked string
			r.resolve = func(name string) (string, error) {
				asked = name
				return tc.resolve(name)
			}
			got, err := r.SignedIn(context.Background())
			if (err != nil) != tc.wantErr {
				t.Fatalf("SignedIn error = %v, wantErr %v", err, tc.wantErr)
			}
			if got != tc.want {
				t.Errorf("SignedIn = %v, want %v", got, tc.want)
			}
			if asked != "claude" {
				t.Errorf("resolver asked for %q, want claude", asked)
			}
		})
	}
}
