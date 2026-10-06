package podctl

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
	corev1 "k8s.io/api/core/v1"
)

const listConv = "11111111-2222-3333-4444-555555555555"

func annotations(workDir string, extra map[string]string) map[string]string {
	a := map[string]string{}
	for k, v := range map[string]string{
		"managed": tmuxctl.OptionManagedValue, "owner": "alice", "name": "my label",
		"workdir": base64.StdEncoding.EncodeToString([]byte(workDir)), "start": "claude",
		"lifetime": "2h", "conversation": listConv, "binary": "claude",
	} {
		a[AnnotationPrefix+k] = v
	}
	for k, v := range extra {
		a[AnnotationPrefix+k] = v
	}
	return a
}

// listRow is one in-pod list-sessions row in argvList's field order.
func listRow(name, managed, label, workDir, start, lifetime, width, conv, live string) string {
	return strings.Join([]string{name, "1700000000", managed, label,
		base64.StdEncoding.EncodeToString([]byte(workDir)), start, lifetime, width, conv, live}, "|") + "\n"
}

func listArgv() string { return strings.Join(inPod(tmuxctl.ArgvList()), " ") }

func (r *rig) setOptions() []string {
	var out []string
	r.rec.mu.Lock()
	defer r.rec.mu.Unlock()
	for _, c := range r.rec.calls {
		if len(c.argv) > 3 && c.argv[3] == "set-option" {
			out = append(out, c.argv[len(c.argv)-2])
		}
	}
	return out
}

func (r *rig) listCalls() int {
	n := 0
	r.rec.mu.Lock()
	defer r.rec.mu.Unlock()
	for _, c := range r.rec.calls {
		if strings.Join(c.argv, " ") == listArgv() {
			n++
		}
	}
	return n
}

func TestListRowsFromObjects(t *testing.T) {
	t.Parallel()
	r := newRig(t, nil)
	ctx := context.Background()
	r.pod(t, "crswd-run", corev1.PodRunning)
	r.pod(t, "crswd-pend", corev1.PodPending)
	r.object(t, "crswd-run", annotations("/work/a", nil), v1alpha1.AgentSessionStatus{})
	r.object(t, "crswd-pend", annotations("/work/b", nil), v1alpha1.AgentSessionStatus{})
	r.object(t, "crswd-bad", annotations("/work/c", nil), v1alpha1.AgentSessionStatus{Phase: v1alpha1.PhaseRejected})
	r.rec.on(listArgv(), reply{stdout: listRow("crswd-run", "1", "my label", "/work/a", "claude", "2h", "", listConv, "1")})

	rows, err := r.c.List(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2 (a Rejected object has none)", len(rows))
	}
	by := map[string]tmuxctl.SessionInfo{}
	for _, row := range rows {
		by[row.Name] = row
	}
	if got := by["crswd-run"]; got.Claude != tmuxctl.LivenessRunning || !got.Managed || got.WorkDir != "/work/a" {
		t.Errorf("running row = %+v", got)
	}
	pend := by["crswd-pend"]
	if pend.Claude != tmuxctl.LivenessUnknown || pend.WorkDir != "/work/b" || pend.Label != "my label" {
		t.Errorf("pending row = %+v", pend)
	}
	if r.listCalls() != 1 {
		t.Errorf("list-sessions ran %d times, want 1 (only the Running pod)", r.listCalls())
	}
}

func TestListCreatedIsTheObjectTimestamp(t *testing.T) {
	t.Parallel()
	r := newRig(t, nil)
	stamp := time.Date(2026, 10, 5, 8, 30, 0, 0, time.UTC)
	if _, err := r.obj.Create(context.Background(), v1alpha1.AgentSession{
		Metadata: v1alpha1.ObjectMeta{Name: "crswd-t", CreationTimestamp: stamp, Annotations: annotations("/work/a", nil)},
		Spec:     v1alpha1.AgentSessionSpec{SessionName: "l", Owner: "o", WorkDir: "/work/a", StartCommand: "claude", Lifetime: "1h"},
	}); err != nil {
		t.Fatal(err)
	}
	r.pod(t, "crswd-t", corev1.PodRunning)
	r.rec.on(listArgv(), reply{stdout: listRow("crswd-t", "1", "my label", "/work/a", "claude", "2h", "", listConv, "0")})
	rows, err := r.c.List(context.Background())
	if err != nil || len(rows) != 1 {
		t.Fatalf("rows=%v err=%v", rows, err)
	}
	if !rows[0].Created.Equal(stamp) {
		t.Errorf("Created = %v, want the object's %v, not tmux's", rows[0].Created, stamp)
	}
	if rows[0].Claude != tmuxctl.LivenessStopped {
		t.Errorf("Claude = %q, want stopped from the pod", rows[0].Claude)
	}
}

// seqList answers list-sessions from a queue, one reply per call, and passes
// everything else to the recorder.
type seqList struct {
	*recorder
	mu     sync.Mutex
	listed []string
}

func (s *seqList) Exec(ctx context.Context, pod string, argv []string, in io.Reader, out, errw io.Writer) (int, error) {
	if strings.Join(argv, " ") == listArgv() {
		s.recorder.mu.Lock()
		s.recorder.calls = append(s.recorder.calls, call{pod: pod, argv: append([]string(nil), argv...)})
		s.recorder.mu.Unlock()
		s.mu.Lock()
		defer s.mu.Unlock()
		reply := ""
		if len(s.listed) > 0 {
			reply, s.listed = s.listed[0], s.listed[1:]
		}
		_, _ = io.WriteString(out, reply)
		return 0, nil
	}
	return s.recorder.Exec(ctx, pod, argv, in, out, errw)
}

func TestListRepairsUnmanagedPod(t *testing.T) {
	t.Parallel()
	var seq *seqList
	r := newRig(t, func(rec *recorder) Executor { seq = &seqList{recorder: rec}; return seq })
	r.pod(t, "crswd-x", corev1.PodRunning)
	r.object(t, "crswd-x", annotations("/work/a", map[string]string{"width": "120"}), v1alpha1.AgentSessionStatus{})
	// A fresh pod has the tmux session but none of its options; after the replay
	// the second list carries them.
	seq.listed = []string{
		listRow("crswd-x", "", "", "", "", "", "", "", "?"),
		listRow("crswd-x", "1", "my label", "/work/a", "claude", "2h", "120", listConv, "0"),
	}
	rows, err := r.c.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	opts := r.setOptions()
	if len(opts) != 9 {
		t.Fatalf("set-option ran for %v, want one per annotation (8 options and width, managed last)", opts)
	}
	if opts[len(opts)-1] != tmuxctl.OptionManaged {
		t.Errorf("last option is %s, want %s so a cut-off repair reads unmanaged", opts[len(opts)-1], tmuxctl.OptionManaged)
	}
	if r.listCalls() != 2 {
		t.Errorf("list-sessions ran %d times, want 2 (before and after the repair)", r.listCalls())
	}
	if len(rows) != 1 || rows[0].Claude != tmuxctl.LivenessStopped {
		t.Errorf("rows = %+v, want the second list's stopped row, so the supervisor revives", rows)
	}
}

func TestListRepairsDriftedLabel(t *testing.T) {
	t.Parallel()
	var seq *seqList
	r := newRig(t, func(rec *recorder) Executor { seq = &seqList{recorder: rec}; return seq })
	r.pod(t, "crswd-x", corev1.PodRunning)
	r.object(t, "crswd-x", annotations("/work/a", nil), v1alpha1.AgentSessionStatus{})
	seq.listed = []string{
		listRow("crswd-x", "1", "stale", "/work/a", "claude", "2h", "", listConv, "1"),
		listRow("crswd-x", "1", "my label", "/work/a", "claude", "2h", "", listConv, "1"),
	}
	rows, err := r.c.List(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.setOptions()) != 8 {
		t.Errorf("set-option ran for %v, want 8 (no width annotated)", r.setOptions())
	}
	if len(rows) != 1 || rows[0].Label != "my label" {
		t.Errorf("rows = %+v", rows)
	}
}

func TestListLeavesAgreeingPodAlone(t *testing.T) {
	t.Parallel()
	r := newRig(t, nil)
	r.pod(t, "crswd-x", corev1.PodRunning)
	r.object(t, "crswd-x", annotations("/work/a", nil), v1alpha1.AgentSessionStatus{})
	r.rec.on(listArgv(), reply{stdout: listRow("crswd-x", "1", "my label", "/work/a", "claude", "2h", "", listConv, "1")})
	if _, err := r.c.List(context.Background()); err != nil {
		t.Fatal(err)
	}
	if n := len(r.setOptions()); n != 0 {
		t.Errorf("repaired an agreeing pod with %d set-option calls", n)
	}
}

func TestListExecErrorKeepsRow(t *testing.T) {
	t.Parallel()
	r := newRig(t, nil)
	r.pod(t, "crswd-x", corev1.PodRunning)
	r.object(t, "crswd-x", annotations("/work/a", nil), v1alpha1.AgentSessionStatus{})
	r.rec.on(listArgv(), reply{err: errors.New("transport")})
	rows, err := r.c.List(context.Background())
	if err != nil {
		t.Fatalf("one pod's exec error failed List: %v", err)
	}
	if len(rows) != 1 || rows[0].Claude != tmuxctl.LivenessUnknown {
		t.Errorf("rows = %+v, want one Unknown row", rows)
	}
}
