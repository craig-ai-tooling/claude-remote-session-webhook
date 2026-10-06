package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nctiggy/claude-remote-session-webhook/internal/config"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
)

const codexDir = "/home/op/code/repo"

func TestCodexHome(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		env  []string
		want string
	}{
		{name: "codex home wins", env: []string{"HOME=/home/op", "CODEX_HOME=/srv/codex"}, want: "/srv/codex"},
		{name: "home otherwise", env: []string{"HOME=/home/op"}, want: "/home/op/.codex"},
		{name: "relative codex home is ignored", env: []string{"HOME=/home/op", "CODEX_HOME=codex"}, want: "/home/op/.codex"},
		{name: "nothing absolute", env: []string{"HOME=", "CODEX_HOME="}, want: ""},
		{name: "relative home", env: []string{"HOME=home"}, want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := CodexHome(tt.env); got != tt.want {
				t.Errorf("CodexHome() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestWithCodexTrust(t *testing.T) {
	t.Parallel()

	header := "[projects.\"" + codexDir + "\"]"
	block := header + "\ntrust_level = \"trusted\"\n"
	tests := []struct {
		name        string
		in          string
		dir         string
		want        string
		wantChanged bool
		wantErr     error
	}{
		{name: "empty file", in: "", dir: codexDir, want: "\n" + block, wantChanged: true},
		{
			name: "other projects", dir: codexDir, wantChanged: true,
			in:   "[projects.\"/other\"]\ntrust_level = \"trusted\"\n",
			want: "[projects.\"/other\"]\ntrust_level = \"trusted\"\n\n" + block,
		},
		{
			name: "untrusted becomes trusted", dir: codexDir, wantChanged: true,
			in:   header + "\ntrust_level = \"untrusted\"\n",
			want: header + "\ntrust_level = \"trusted\"\n",
		},
		{
			name: "table without trust_level", dir: codexDir, wantChanged: true,
			in:   header + "\nother = 1\n",
			want: header + "\ntrust_level = \"trusted\"\nother = 1\n",
		},
		{
			name: "already trusted with comment", dir: codexDir,
			in:   header + "\ntrust_level = \"trusted\" # ok\n",
			want: header + "\ntrust_level = \"trusted\" # ok\n",
		},
		{
			name: "trust_level in a later table is not ours", dir: codexDir, wantChanged: true,
			in:   header + "\n[other]\ntrust_level = \"trusted\"\n",
			want: header + "\ntrust_level = \"trusted\"\n[other]\ntrust_level = \"trusted\"\n",
		},
		{
			name: "inline table", dir: codexDir, wantErr: ErrCodexConfigShape,
			in: "projects = { \"" + codexDir + "\" = { trust_level = \"trusted\" } }\n",
		},
		{name: "comment naming the dir", dir: codexDir, wantErr: ErrCodexConfigShape, in: "# " + codexDir + "\n"},
		{name: "duplicate header", dir: codexDir, wantErr: ErrCodexConfigShape, in: block + block},
		{
			name: "quoted trust_level key", dir: codexDir, wantErr: ErrCodexConfigShape,
			in: header + "\n\"trust_level\" = \"untrusted\"\n",
		},
		{
			name: "single-quoted trust_level key", dir: codexDir, wantErr: ErrCodexConfigShape,
			in: header + "\n'trust_level' = \"untrusted\"\n",
		},
		{
			name: "escaped trust_level key", dir: codexDir, wantErr: ErrCodexConfigShape,
			in: header + "\n\"trust\\u005flevel\" = \"untrusted\"\n",
		},
		{
			name: "escaped header naming the same dir", dir: codexDir, wantErr: ErrCodexConfigShape,
			in: "[projects.\"/home/op/code/\\u0072epo\"]\ntrust_level = \"untrusted\"\n",
		},
		{name: "invalid utf-8 in dir", dir: "/a\xffb", wantErr: ErrUntrustablePath},
		{name: "quote in dir", dir: "/a\"b", wantErr: ErrUntrustablePath},
		{name: "backslash in dir", dir: "/a\\b", wantErr: ErrUntrustablePath},
		{name: "newline in dir", dir: "/a\nb", wantErr: ErrUntrustablePath},
		{name: "del in dir", dir: "/a\x7fb", wantErr: ErrUntrustablePath},
		{
			name: "crlf untrusted line", dir: codexDir, wantChanged: true,
			in:   "a = 1\r\n" + header + "\r\ntrust_level = \"untrusted\"\r\nb = 2\r\n",
			want: "a = 1\r\n" + header + "\r\ntrust_level = \"trusted\"\r\nb = 2\r\n",
		},
		{
			name: "crlf table without trust_level", dir: codexDir, wantChanged: true,
			in:   header + "\r\nother = 1\r\n",
			want: header + "\r\ntrust_level = \"trusted\"\r\nother = 1\r\n",
		},
		{
			name: "crlf file without the table", dir: codexDir, wantChanged: true,
			in:   "a = 1\r\n",
			want: "a = 1\r\n\n" + block,
		},
		{
			name: "no trailing newline", dir: codexDir, wantChanged: true,
			in:   "a = 1",
			want: "a = 1\n\n" + block,
		},
		{
			name: "unterminated header is the last line", dir: codexDir, wantChanged: true,
			in:   header,
			want: header + "\ntrust_level = \"trusted\"\n",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, changed, err := withCodexTrust([]byte(tt.in), tt.dir)
			if tt.wantErr != nil {
				if !errors.Is(err, tt.wantErr) {
					t.Fatalf("err = %v, want %v", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if changed != tt.wantChanged {
				t.Errorf("changed = %v, want %v", changed, tt.wantChanged)
			}
			if string(got) != tt.want {
				t.Errorf("output = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestSeedCodexTrust(t *testing.T) {
	t.Parallel()

	block := "[projects.\"" + codexDir + "\"]\ntrust_level = \"trusted\"\n"

	t.Run("empty home does nothing", func(t *testing.T) {
		t.Parallel()
		if err := SeedCodexTrust("", codexDir); err != nil {
			t.Fatal(err)
		}
	})

	t.Run("missing home creates nothing", func(t *testing.T) {
		t.Parallel()
		home := filepath.Join(t.TempDir(), "absent")
		if err := SeedCodexTrust(home, codexDir); err != nil {
			t.Fatal(err)
		}
		if _, err := os.Stat(home); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("home was created: %v", err)
		}
	})

	t.Run("no config is created at 0600", func(t *testing.T) {
		t.Parallel()
		home := t.TempDir()
		if err := SeedCodexTrust(home, codexDir); err != nil {
			t.Fatal(err)
		}
		path := filepath.Join(home, "config.toml")
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Errorf("mode = %o, want 600", info.Mode().Perm())
		}
		raw, err := os.ReadFile(path) //nolint:gosec // G304: a path under t.TempDir().
		if err != nil {
			t.Fatal(err)
		}
		if got := string(raw); got != "\n"+block {
			t.Errorf("config = %q, want %q", got, "\n"+block)
		}
	})

	t.Run("existing mode is kept", func(t *testing.T) {
		t.Parallel()
		home := t.TempDir()
		path := filepath.Join(home, "config.toml")
		if err := os.WriteFile(path, []byte("a = 1\n"), 0o644); err != nil { //nolint:gosec // G306: the test asserts a 0644 file keeps its mode.
			t.Fatal(err)
		}
		if err := SeedCodexTrust(home, codexDir); err != nil {
			t.Fatal(err)
		}
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o644 {
			t.Errorf("mode = %o, want 644", info.Mode().Perm())
		}
	})

	t.Run("already trusted is byte identical", func(t *testing.T) {
		t.Parallel()
		home := t.TempDir()
		path := filepath.Join(home, "config.toml")
		want := "a = 1\n\n" + block
		if err := os.WriteFile(path, []byte(want), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := SeedCodexTrust(home, codexDir); err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(path) //nolint:gosec // G304: a path under t.TempDir().
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("config = %q, want %q", got, want)
		}
	})

	t.Run("shape error leaves the file alone", func(t *testing.T) {
		t.Parallel()
		home := t.TempDir()
		path := filepath.Join(home, "config.toml")
		want := "# " + codexDir + "\n"
		if err := os.WriteFile(path, []byte(want), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := SeedCodexTrust(home, codexDir); !errors.Is(err, ErrCodexConfigShape) {
			t.Fatalf("err = %v, want ErrCodexConfigShape", err)
		}
		got, err := os.ReadFile(path) //nolint:gosec // G304: a path under t.TempDir().
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != want {
			t.Errorf("config = %q, want %q", got, want)
		}
	})

	t.Run("relative dir is refused", func(t *testing.T) {
		t.Parallel()
		if err := SeedCodexTrust(t.TempDir(), "repo"); err == nil {
			t.Fatal("a relative directory was accepted")
		}
	})

	t.Run("untrustable dir is refused", func(t *testing.T) {
		t.Parallel()
		if err := SeedCodexTrust(t.TempDir(), "/a\"b"); !errors.Is(err, ErrUntrustablePath) {
			t.Fatalf("err = %v, want ErrUntrustablePath", err)
		}
	})
}

func codexManagerFixture(t *testing.T, contents string) (managerFixture, string) {
	t.Helper()

	home := t.TempDir()
	path := filepath.Join(home, "config.toml")
	if contents != "" {
		if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	f := newManagerFixture(t)
	f.mgr.SetStartCommands(config.NewStartCommands(map[string]string{
		config.DefaultStartCommandName: claudeStartCommand,
		"codex":                        "codex --dangerously-bypass-approvals-and-sandbox",
	}))
	f.mgr.SetCodexHome(home)
	return f, path
}

func TestCreateCodexTrustsTheWorkDir(t *testing.T) {
	t.Parallel()

	f, path := codexManagerFixture(t, "model = \"x\"\n")
	req := f.request()
	req.StartCommand = "codex"

	s, _ := mustCreate(t, f, req)

	got, err := os.ReadFile(path) //nolint:gosec // G304: a path under t.TempDir().
	if err != nil {
		t.Fatal(err)
	}
	want := "[projects.\"" + s.WorkDir + "\"]\ntrust_level = \"trusted\"\n"
	if !strings.Contains(string(got), want) {
		t.Errorf("config.toml after Create = %q, want it to contain %q", got, want)
	}
}

func TestCreateCodexFailsOnShape(t *testing.T) {
	t.Parallel()

	f, path := codexManagerFixture(t, "")
	req := f.request()
	req.StartCommand = "codex"
	probe, _ := mustCreate(t, f, req)
	// A comment naming the directory is a shape crswd refuses to edit.
	if err := os.WriteFile(path, []byte("# "+probe.WorkDir+"\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	before := len(f.tmux.Calls())

	_, _, err := f.mgr.Create(context.Background(), req)
	if !errors.Is(err, ErrCodexConfigShape) {
		t.Fatalf("Create() error = %v, want ErrCodexConfigShape", err)
	}
	for _, c := range f.tmux.Calls()[before:] {
		if c.Op == tmuxctl.OpSendKeys {
			t.Errorf("a send-keys call was made after the seeding failed: %+v", c)
		}
	}
}

func TestCreateClaudeDoesNotTouchCodexConfig(t *testing.T) {
	t.Parallel()

	f, path := codexManagerFixture(t, "model = \"x\"\n")

	mustCreate(t, f, f.request())

	got, err := os.ReadFile(path) //nolint:gosec // G304: a path under t.TempDir().
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "model = \"x\"\n" {
		t.Errorf("config.toml after a Claude create = %q, want it unchanged", got)
	}
}
