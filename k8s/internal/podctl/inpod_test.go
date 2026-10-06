package podctl

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
)

const testConv = "11111111-2222-3333-4444-555555555555"

func TestCodexConversationArgv(t *testing.T) {
	t.Parallel()
	r := newRig(t, nil)
	want := []string{Binary, "codex-conversation", testName}
	r.rec.on(strings.Join(want, " "), reply{stdout: testConv + "\n"})
	got, err := r.c.CodexConversation(context.Background(), testName)
	if err != nil || got != testConv {
		t.Fatalf("got %q, %v", got, err)
	}
	if a := r.argvs(); len(a) != 1 || !slices.Equal(a[0], want) || r.rec.calls[0].pod != testName {
		t.Fatalf("argv = %v", a)
	}
}

func TestCodexConversationResults(t *testing.T) {
	t.Parallel()
	key := strings.Join([]string{Binary, "codex-conversation", testName}, " ")
	cases := map[string]struct {
		rp      reply
		want    string
		wantErr bool
	}{
		"empty":      {reply{}, "", false},
		"not a uuid": {reply{stdout: "../../etc/passwd\n"}, "", true},
		"exit 2":     {reply{stderr: "boom\nmore", code: 2}, "", true},
		"transport":  {reply{err: errors.New("down")}, "", true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, nil)
			r.rec.on(key, tc.rp)
			got, err := r.c.CodexConversation(context.Background(), testName)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("got %q, %v", got, err)
			}
			if err != nil && strings.Contains(err.Error(), "passwd") {
				t.Fatalf("the output reached the error: %v", err)
			}
		})
	}
}

func TestHasTranscript(t *testing.T) {
	t.Parallel()
	argv := []string{Binary, "has-transcript", testName, "claude", testConv, "/work/a"}
	key := strings.Join(argv, " ")
	cases := map[string]struct {
		rp      reply
		want    bool
		wantErr bool
	}{
		"present":   {reply{}, true, false},
		"absent":    {reply{code: 1}, false, false},
		"exit 2":    {reply{code: 2, stderr: "bad"}, false, true},
		"transport": {reply{err: errors.New("down")}, false, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, nil)
			r.rec.on(key, tc.rp)
			got, err := r.c.HasTranscript(context.Background(), testName, harness.Claude, testConv, "/work/a")
			if got != tc.want || (err != nil) != tc.wantErr {
				t.Fatalf("got %v, %v", got, err)
			}
			if a := r.argvs(); len(a) != 1 || !slices.Equal(a[0], argv) {
				t.Fatalf("argv = %v", a)
			}
		})
	}
}
