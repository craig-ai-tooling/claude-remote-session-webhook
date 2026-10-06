package tmuxctl

import (
	"reflect"
	"strings"
	"testing"
)

func TestExportParseSessionsEqualsParser(t *testing.T) {
	t.Parallel()

	// A row of exactly listFieldCount fields parses; the creation time must be a number.
	parts := make([]string, listFieldCount)
	parts[0] = "crswd-x"
	parts[1] = "1785706480"
	stdout := strings.Join(parts, "|") + "\n"

	want, wantErr := parseSessions(stdout)
	got, gotErr := ParseSessions(stdout)
	if wantErr != nil || gotErr != nil {
		t.Fatalf("parseSessions err = %v, ParseSessions err = %v, want both nil", wantErr, gotErr)
	}
	if len(got) != 1 || got[0].Name != "crswd-x" {
		t.Fatalf("ParseSessions = %#v, want one session named crswd-x", got)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ParseSessions = %#v, parseSessions = %#v", got, want)
	}

	if _, err := ParseSessions("not|a|row\n"); err == nil {
		t.Fatal("ParseSessions accepted a short row")
	}
}

func TestExportNewBufferNamePrefix(t *testing.T) {
	t.Parallel()

	a, err := NewBufferName()
	if err != nil {
		t.Fatalf("NewBufferName: %v", err)
	}
	b, err := NewBufferName()
	if err != nil {
		t.Fatalf("NewBufferName: %v", err)
	}
	for _, name := range []string{a, b} {
		if !strings.HasPrefix(name, BufferPrefix) {
			t.Errorf("%q does not start with %q", name, BufferPrefix)
		}
		hex := strings.TrimPrefix(name, BufferPrefix)
		if len(hex) != 16 || strings.Trim(hex, "0123456789abcdef") != "" {
			t.Errorf("%q: want 16 hex digits after the prefix, got %q", name, hex)
		}
	}
	if a == b {
		t.Errorf("two calls returned the same name %q", a)
	}
}

func TestExportAbsent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name   string
		code   int
		stderr string
		want   bool
	}{
		{"no session", 1, "can't find session: crswd-x", true},
		{"no server", 1, "no server running on /tmp/x", true},
		{"no socket", 1, "error connecting to /tmp/x (No such file or directory)", true},
		{"socket unreachable", 1, "error connecting to /tmp/x (Permission denied)", false},
		{"success", 0, "", false},
		{"other exit code", 2, "can't find session", false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			if got := Absent(tc.code, tc.stderr); got != tc.want {
				t.Errorf("Absent(%d, %q) = %v, want %v", tc.code, tc.stderr, got, tc.want)
			}
		})
	}
}
