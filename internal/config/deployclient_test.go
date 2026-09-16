package config_test

// deploy/crswd-api is the client every operator and every piece of automation
// reaches the daemon through, and nothing executed it until this file did. It
// is checked here for the same reason deployexample_test.go checks the unit
// file: deploy/ is a shipped artifact whose failures are silent, and this is
// where the tests that run it live.
//
// What is checked is the contract the daemon's own auth package enforces on the
// other side of the wire — the signed payload's shape (internal/auth/hmac.go)
// and what the client does with the uniform 401 that package produces.
//
// The 401 case is not a hypothetical. On 2026-09-16 four systemd timers started
// in the same second; two of them listed sessions; the signature covers a
// timestamp with one-second resolution and nothing else that varies between
// them, so the two requests were byte-identical and the replay cache (FR-010)
// refused the second. The loser exited non-zero and closed no finished session.
// The daemon is right to refuse — it cannot distinguish that from a captured
// request being replayed — so the client is what has to know it has just issued
// the request for the first time, and ask again under a later timestamp.

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

const clientPath = "../../deploy/crswd-api"

// testSecret is a value for a stub `op` to hand back, not a credential. It is
// never a real shared secret and the daemon it talks to here is an httptest
// server that lives for the length of one test.
const testSecret = "deploy-client-test-secret-0123456789abcdef"

// seenRequest is one arrival at the stub daemon, in the order they came in.
type seenRequest struct {
	method    string
	path      string
	timestamp string
	signature string
	body      string
	bearer    string
}

// stubDaemon answers like the real one: each reply is decided by respond, which
// is handed the number of requests already served.
type stubDaemon struct {
	mu      sync.Mutex
	seen    []seenRequest
	respond func(n int, w http.ResponseWriter)
}

func (d *stubDaemon) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	raw, _ := io.ReadAll(r.Body)

	d.mu.Lock()
	n := len(d.seen)
	d.seen = append(d.seen, seenRequest{
		method:    r.Method,
		path:      r.URL.Path,
		timestamp: r.Header.Get("X-CRSW-Timestamp"),
		signature: r.Header.Get("X-CRSW-Signature"),
		body:      string(raw),
		bearer:    r.Header.Get("Authorization"),
	})
	d.mu.Unlock()

	d.respond(n, w)
}

func (d *stubDaemon) requests() []seenRequest {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := make([]seenRequest, len(d.seen))
	copy(out, d.seen)
	return out
}

// unauthorized is the daemon's uniform denial, byte for byte what
// internal/httpapi writes for every layer-2 failure (FR-011).
func unauthorized(w http.ResponseWriter) {
	w.WriteHeader(http.StatusUnauthorized)
	_, _ = w.Write([]byte(`{"error":"unauthorized"}`))
}

// stubOp writes an `op` that answers the three reads the client makes. Without
// it the client exits before sending anything, and the test would be measuring
// 1Password rather than the client.
func stubOp(t *testing.T) string {
	t.Helper()

	dir := t.TempDir()
	script := fmt.Sprintf(`#!/bin/sh
# Stub 1Password CLI. Only `+"`op read <path>`"+` is used by the client.
case "$2" in
  */shared-secret)        echo '%s' ;;
  */access-client-id)     echo 'test-access-client-id' ;;
  */access-client-secret) echo 'test-access-client-secret' ;;
  *) echo "stub op: unexpected read: $2" >&2; exit 1 ;;
esac
`, testSecret)

	path := filepath.Join(dir, "op")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		t.Fatalf("write stub op: %v", err)
	}
	return dir
}

// runClient executes the real deploy/crswd-api against srv and returns its
// stdout and exit status.
func runClient(t *testing.T, srv *httptest.Server, args ...string) (string, int) {
	t.Helper()

	for _, tool := range []string{"bash", "curl", "openssl", "awk"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s is not on PATH; the client cannot run here", tool)
		}
	}

	cmd := exec.Command("bash", append([]string{clientPath}, args...)...)
	// The stub op goes FIRST: the client only ever appends to PATH, so
	// whatever leads here keeps the lead inside it.
	cmd.Env = append(os.Environ(),
		"PATH="+stubOp(t)+string(os.PathListSeparator)+os.Getenv("PATH"),
		"CRSWD_HOST="+srv.URL,
		"CRSWD_OP_ITEM=op://test-vault/crswd",
	)

	out, err := cmd.Output()
	code := 0
	if exit, ok := err.(*exec.ExitError); ok {
		code = exit.ExitCode()
		t.Logf("client stderr: %s", exit.Stderr)
	} else if err != nil {
		t.Fatalf("run %s: %v", clientPath, err)
	}
	return string(out), code
}

// expectedSignature recomputes the payload the daemon verifies, so this test
// fails if the client ever signs something else.
func expectedSignature(method, path, timestamp, body string) string {
	mac := hmac.New(sha256.New, []byte(testSecret))
	fmt.Fprintf(mac, "%s\n%s\n%s.%s", method, path, timestamp, body)
	return "sha256=" + hex.EncodeToString(mac.Sum(nil))
}

// TestClientSignsThePayloadTheDaemonVerifies pins the wire format: the headers
// that must be present, and the exact bytes under the HMAC.
func TestClientSignsThePayloadTheDaemonVerifies(t *testing.T) {
	daemon := &stubDaemon{respond: func(_ int, w http.ResponseWriter) {
		_, _ = w.Write([]byte(`{"ok":true}`))
	}}
	srv := httptest.NewServer(daemon)
	defer srv.Close()

	body := `{"text":"hi"}`
	out, code := runClient(t, srv, "POST", "/sessions/abc/prompt", body, "session-bearer")
	if code != 0 {
		t.Fatalf("exit %d, want 0; stdout %q", code, out)
	}
	if got := strings.TrimSpace(out); got != `{"ok":true}` {
		t.Fatalf("stdout %q, want the response body", got)
	}

	seen := daemon.requests()
	if len(seen) != 1 {
		t.Fatalf("%d request(s), want exactly 1: a success must not be retried", len(seen))
	}
	got := seen[0]

	if got.method != "POST" || got.path != "/sessions/abc/prompt" {
		t.Errorf("request line %s %s, want POST /sessions/abc/prompt", got.method, got.path)
	}
	if got.body != body {
		t.Errorf("body %q, want %q", got.body, body)
	}
	if got.bearer != "Bearer session-bearer" {
		t.Errorf("Authorization %q, want the bearer passed as the 4th argument", got.bearer)
	}
	if want := expectedSignature("POST", "/sessions/abc/prompt", got.timestamp, body); got.signature != want {
		t.Errorf("signature %q, want %q — the signed payload is METHOD\\nPATH\\ntimestamp.body", got.signature, want)
	}

	ts, err := strconv.ParseInt(got.timestamp, 10, 64)
	if err != nil {
		t.Fatalf("timestamp %q is not the decimal integer the daemon parses: %v", got.timestamp, err)
	}
	if skew := time.Since(time.Unix(ts, 0)); skew < -time.Minute || skew > time.Minute {
		t.Errorf("timestamp is %v from now; the daemon's window is 300s", skew)
	}
}

// TestClientRetriesAReplayedSignature is the 2026-09-16 failure. The daemon
// refuses the first request exactly as it refuses a collision with another
// caller's identical one, and the client must come back with a signature that
// is not the same bytes.
func TestClientRetriesAReplayedSignature(t *testing.T) {
	daemon := &stubDaemon{respond: func(n int, w http.ResponseWriter) {
		if n == 0 {
			unauthorized(w)
			return
		}
		_, _ = w.Write([]byte(`{"sessions":[]}`))
	}}
	srv := httptest.NewServer(daemon)
	defer srv.Close()

	out, code := runClient(t, srv, "GET", "/sessions")
	if code != 0 {
		t.Fatalf("exit %d, want 0; stdout %q", code, out)
	}
	if got := strings.TrimSpace(out); got != `{"sessions":[]}` {
		t.Fatalf("stdout %q, want the retry's body — a caller that reads this cannot see the 401", got)
	}

	seen := daemon.requests()
	if len(seen) != 2 {
		t.Fatalf("%d request(s), want 2: one refused, one retried", len(seen))
	}
	if seen[0].signature == seen[1].signature {
		t.Fatalf("the retry re-sent signature %q — an identical signature is refused again", seen[0].signature)
	}
	if seen[0].timestamp == seen[1].timestamp {
		t.Errorf("both requests carried timestamp %q; the retry must be signed under a later one", seen[0].timestamp)
	}
	if want := expectedSignature("GET", "/sessions", seen[1].timestamp, ""); seen[1].signature != want {
		t.Errorf("retry signature %q, want %q — the retry must be a real signature, not a reused one", seen[1].signature, want)
	}
}

// TestClientRetriesOnceAndReportsTheDenial guards the other direction: a secret
// that is genuinely wrong is refused every time, and the client must hand that
// answer back rather than hammer the daemon.
func TestClientRetriesOnceAndReportsTheDenial(t *testing.T) {
	daemon := &stubDaemon{respond: func(_ int, w http.ResponseWriter) { unauthorized(w) }}
	srv := httptest.NewServer(daemon)
	defer srv.Close()

	out, _ := runClient(t, srv, "GET", "/sessions")
	if got := strings.TrimSpace(out); got != `{"error":"unauthorized"}` {
		t.Fatalf("stdout %q, want the daemon's denial passed through", got)
	}
	if n := len(daemon.requests()); n != 2 {
		t.Fatalf("%d request(s), want 2: the retry happens once, not in a loop", n)
	}
}
