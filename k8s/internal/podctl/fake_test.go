package podctl

import (
	"context"
	"io"
	"strings"
	"sync"
)

// reply is one canned answer, keyed by the joined argv.
type reply struct {
	stdout, stderr string
	code           int
	err            error
}

// call is one recorded Exec.
type call struct {
	pod   string
	argv  []string
	stdin []byte
}

// recorder is an Executor that records each call and answers from a table.
// An argv with no entry answers exit code 0.
type recorder struct {
	mu      sync.Mutex
	calls   []call
	replies map[string]reply
}

func newRecorder() *recorder { return &recorder{replies: map[string]reply{}} }

func (r *recorder) on(argv string, rp reply) { r.replies[argv] = rp }

func (r *recorder) Exec(_ context.Context, pod string, argv []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	var in []byte
	if stdin != nil {
		in, _ = io.ReadAll(stdin)
	}
	r.mu.Lock()
	r.calls = append(r.calls, call{pod: pod, argv: append([]string(nil), argv...), stdin: in})
	rp := r.replies[strings.Join(argv, " ")]
	r.mu.Unlock()
	if stdout != nil && rp.stdout != "" {
		_, _ = io.WriteString(stdout, rp.stdout)
	}
	if stderr != nil && rp.stderr != "" {
		_, _ = io.WriteString(stderr, rp.stderr)
	}
	return rp.code, rp.err
}
