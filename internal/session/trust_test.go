package session

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// trustOf reads projects[dir].hasTrustDialogAccepted back out of path.
func trustOf(t *testing.T, path, dir string) any {
	t.Helper()
	raw, err := os.ReadFile(path) //nolint:gosec // G304: a path under t.TempDir().
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	var doc struct {
		Projects map[string]map[string]any `json:"projects"`
	}
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("the rewritten file is not JSON: %v", err)
	}
	return doc.Projects[dir][trustKey]
}

func TestSeedTrust(t *testing.T) {
	t.Parallel()

	const dir = "/home/op/code/repo"
	tests := []struct {
		name      string
		initial   string // "" means no file at all
		wantWrite bool
		wantErr   bool
	}{
		{name: "missing file is left missing", initial: ""},
		{name: "already trusted is not rewritten", initial: `{"projects":{"/home/op/code/repo":{"hasTrustDialogAccepted":true}}}`},
		{name: "untrusted entry is trusted", initial: `{"projects":{"/home/op/code/repo":{"hasTrustDialogAccepted":false,"allowedTools":[]}}}`, wantWrite: true},
		{name: "no entry for the directory gains one", initial: `{"projects":{"/other":{"hasTrustDialogAccepted":true}}}`, wantWrite: true},
		{name: "no projects map gains one", initial: `{"numStartups":3}`, wantWrite: true},
		{name: "malformed file is refused and untouched", initial: `{"projects":`, wantErr: true},
		{name: "projects of the wrong type is refused", initial: `{"projects":[]}`, wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			path := filepath.Join(t.TempDir(), ".claude.json")
			if tt.initial != "" {
				if err := os.WriteFile(path, []byte(tt.initial), 0o600); err != nil {
					t.Fatal(err)
				}
			}

			err := SeedTrust(path, dir)
			if (err != nil) != tt.wantErr {
				t.Fatalf("SeedTrust() error = %v, wantErr %v", err, tt.wantErr)
			}

			after, readErr := os.ReadFile(path) //nolint:gosec // G304: a path under t.TempDir().
			if tt.initial == "" {
				if !os.IsNotExist(readErr) {
					t.Fatalf("SeedTrust() created %s; want it left missing", path)
				}
				return
			}
			if readErr != nil {
				t.Fatal(readErr)
			}
			if got := string(after) != tt.initial; got != tt.wantWrite {
				t.Fatalf("file rewritten = %v, want %v", got, tt.wantWrite)
			}
			if tt.wantWrite {
				if got := trustOf(t, path, dir); got != true {
					t.Errorf("%s = %v, want true", trustKey, got)
				}
				info, err := os.Stat(path)
				if err != nil {
					t.Fatal(err)
				}
				if info.Mode().Perm() != 0o600 {
					t.Errorf("mode = %v, want the original 0600", info.Mode().Perm())
				}
			}
		})
	}
}

// Everything the seeder does not own must come back as it went in: other
// projects, other keys in the same entry, and numbers too large for a float64.
func TestSeedTrustPreservesEverythingElse(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), ".claude.json")
	initial := `{"userID":"abc","firstStartTime":12345678901234567890,` +
		`"projects":{"/home/op/code/repo":{"allowedTools":["Bash"],"lastCost":0.25},` +
		`"/other":{"hasTrustDialogAccepted":true,"x":{"y":[1,2]}}}}`
	if err := os.WriteFile(path, []byte(initial), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SeedTrust(path, "/home/op/code/repo"); err != nil {
		t.Fatalf("SeedTrust() = %v", err)
	}

	raw, err := os.ReadFile(path) //nolint:gosec // G304: a path under t.TempDir().
	if err != nil {
		t.Fatal(err)
	}
	expected := `{"userID":"abc","firstStartTime":12345678901234567890,` +
		`"projects":{"/home/op/code/repo":{"allowedTools":["Bash"],"lastCost":0.25,"hasTrustDialogAccepted":true},` +
		`"/other":{"hasTrustDialogAccepted":true,"x":{"y":[1,2]}}}}`
	decode := func(b []byte) any {
		d := json.NewDecoder(bytes.NewReader(b))
		d.UseNumber()
		var v any
		if err := d.Decode(&v); err != nil {
			t.Fatal(err)
		}
		return v
	}
	if got, want := decode(raw), decode([]byte(expected)); !reflect.DeepEqual(got, want) {
		t.Errorf("rewritten file =\n%s\nwant the original plus the trust key:\n%s", raw, expected)
	}
}

func TestSeedTrustRefusesARelativeDirectory(t *testing.T) {
	t.Parallel()

	path := filepath.Join(t.TempDir(), ".claude.json")
	if err := os.WriteFile(path, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SeedTrust(path, "code/repo"); err == nil {
		t.Fatal("SeedTrust() accepted a relative directory")
	}
}

func TestClaudeConfigFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  []string
		want string
	}{
		{name: "config dir wins", env: []string{"HOME=/home/op", "CLAUDE_CONFIG_DIR=/srv/claude"}, want: "/srv/claude/.claude.json"},
		{name: "home otherwise", env: []string{"HOME=/home/op"}, want: "/home/op/.claude.json"},
		{name: "relative config dir is ignored", env: []string{"HOME=/home/op", "CLAUDE_CONFIG_DIR=claude"}, want: "/home/op/.claude.json"},
		{name: "nothing absolute", env: []string{"HOME="}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ClaudeConfigFile(tt.env); got != tt.want {
				t.Errorf("ClaudeConfigFile() = %q, want %q", got, tt.want)
			}
		})
	}
}

// The regression: a session created in a directory Claude Code had never
// trusted started on the trust dialog with nobody to answer it.
func TestCreateTrustsTheWorkDirBeforeStarting(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	path := filepath.Join(t.TempDir(), ".claude.json")
	if err := os.WriteFile(path, []byte(`{"projects":{}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	f.mgr.SetClaudeConfig(path)

	s, _ := mustCreate(t, f, f.request())

	if got := trustOf(t, path, s.WorkDir); got != true {
		t.Errorf("after Create, %s for %s = %v, want true", trustKey, s.WorkDir, got)
	}
}

// A file Claude Code would not read either must stop the create rather than
// start a session that is going to sit on the dialog.
func TestCreateFailsWhenTrustCannotBeSeeded(t *testing.T) {
	t.Parallel()

	f := newManagerFixture(t)
	path := filepath.Join(t.TempDir(), ".claude.json")
	if err := os.WriteFile(path, []byte(`{"projects":`), 0o600); err != nil {
		t.Fatal(err)
	}
	f.mgr.SetClaudeConfig(path)

	if _, _, err := f.mgr.Create(context.Background(), f.request()); err == nil {
		t.Fatal("Create() succeeded with an unreadable Claude config")
	}
}
