package podctl

import (
	"bytes"
	"context"
	"errors"
	"io"
	"reflect"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/nctiggy/claude-remote-session-webhook/api/v1alpha1"
	"github.com/nctiggy/claude-remote-session-webhook/internal/session"
	"github.com/nctiggy/claude-remote-session-webhook/internal/tmuxctl"
	"github.com/nctiggy/claude-remote-session-webhook/k8s/internal/agentsession"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	kfake "k8s.io/client-go/kubernetes/fake"
)

const testName = "crswd-x"

var testNow = time.Date(2026, 10, 6, 12, 0, 0, 0, time.UTC)

type rig struct {
	c    *Controller
	ex   Executor
	rec  *recorder
	pods *kfake.Clientset
	dyn  *dynamicfake.FakeDynamicClient
	obj  *agentsession.Client
}

// newRig builds a Controller over fakes. wrap lets a test put its own Executor
// in front of the recorder.
func newRig(t *testing.T, wrap func(*recorder) Executor) *rig {
	t.Helper()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{agentsession.GVR: v1alpha1.ListKind})
	obj, err := agentsession.New(dyn, "ns")
	if err != nil {
		t.Fatal(err)
	}
	rec := newRecorder()
	var ex Executor = rec
	if wrap != nil {
		ex = wrap(rec)
	}
	pods := kfake.NewSimpleClientset()
	c, err := New(obj, pods, ex, Config{
		Namespace: "ns", PaneBound: 10,
		ReadyTimeout: 200 * time.Millisecond, KillTimeout: 100 * time.Millisecond,
		PollInterval: 5 * time.Millisecond, Now: func() time.Time { return testNow },
	})
	if err != nil {
		t.Fatal(err)
	}
	c.SetDescriber(func(n string) (session.PodRecord, bool) {
		if n != testName {
			return session.PodRecord{}, false
		}
		return session.PodRecord{
			ID: "x", Name: "my label", Owner: "alice", WorkDir: "/work/a", StartCommand: "claude",
			ConversationID: "11111111-2222-3333-4444-555555555555", Deadline: testNow.Add(2*time.Hour + 30*time.Minute + 45*time.Second + 900*time.Millisecond),
		}, true
	})
	return &rig{c: c, ex: ex, rec: rec, pods: pods, dyn: dyn, obj: obj}
}

func (r *rig) pod(t *testing.T, name string, phase corev1.PodPhase) {
	t.Helper()
	_, err := r.pods.CoreV1().Pods("ns").Create(context.Background(),
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "ns"}, Status: corev1.PodStatus{Phase: phase}}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
}

func (r *rig) object(t *testing.T, name string, annotations map[string]string, status v1alpha1.AgentSessionStatus) {
	t.Helper()
	_, err := r.obj.Create(context.Background(), v1alpha1.AgentSession{
		Metadata: v1alpha1.ObjectMeta{Name: name, Annotations: annotations},
		Spec:     v1alpha1.AgentSessionSpec{SessionName: "l", Owner: "o", WorkDir: "/work/a", StartCommand: "claude", Lifetime: "1h"},
		Status:   status,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func (r *rig) creates() int {
	n := 0
	for _, a := range r.dyn.Actions() {
		if a.Matches("create", "agentsessions") {
			n++
		}
	}
	return n
}

func (r *rig) argvs() [][]string {
	r.rec.mu.Lock()
	defer r.rec.mu.Unlock()
	var out [][]string
	for _, c := range r.rec.calls {
		out = append(out, c.argv)
	}
	return out
}

func TestMethodsArgv(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		run  func(*Controller) error
		want []string
	}{
		"send keys": {func(c *Controller) error { return c.SendKeys(context.Background(), testName, "Enter", "C-c") },
			inPod(tmuxctl.ArgvSendKeys(testName, "Enter", "C-c"))},
		"resize": {func(c *Controller) error { return c.Resize(context.Background(), testName, 100, 40) },
			inPod(tmuxctl.ArgvResize(testName, 100, 40))},
		"set option": {func(c *Controller) error {
			return c.SetOption(context.Background(), testName, tmuxctl.OptionOwner, "alice")
		}, inPod(tmuxctl.ArgvSetOption(testName, tmuxctl.OptionOwner, "alice"))},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, nil)
			r.object(t, testName, nil, v1alpha1.AgentSessionStatus{})
			if err := tc.run(r.c); err != nil {
				t.Fatal(err)
			}
			got := r.argvs()
			if len(got) != 1 || !slices.Equal(got[0], tc.want) {
				t.Fatalf("argv = %v, want [%v]", got, tc.want)
			}
			if r.rec.calls[0].pod != testName {
				t.Fatalf("pod = %q", r.rec.calls[0].pod)
			}
		})
	}
}

func TestMethodsNonZeroExit(t *testing.T) {
	t.Parallel()
	r := newRig(t, nil)
	r.object(t, testName, nil, v1alpha1.AgentSessionStatus{})
	r.rec.on(strings.Join(inPod(tmuxctl.ArgvSendKeys(testName, "Enter")), " "), reply{stderr: "can't find session: crswd-x\nmore", code: 1})
	err := r.c.SendKeys(context.Background(), testName, "Enter")
	if err == nil || !strings.Contains(err.Error(), "can't find session") || strings.Contains(err.Error(), "more") {
		t.Fatalf("err = %v, want first stderr line only", err)
	}
	r.rec.on(strings.Join(inPod(tmuxctl.ArgvResize(testName, 80, 24)), " "), reply{err: errors.New("transport down")})
	if err := r.c.Resize(context.Background(), testName, 80, 24); err == nil {
		t.Fatal("transport error was swallowed")
	}
}

func TestPastePayloadInStdinNotArgv(t *testing.T) {
	t.Parallel()
	payload := []byte("secret $(rm -rf /) payload;")
	for _, bracketed := range []bool{false, true} {
		r := newRig(t, nil)
		var err error
		if bracketed {
			err = r.c.PasteBracketed(context.Background(), testName, payload)
		} else {
			err = r.c.Paste(context.Background(), testName, payload)
		}
		if err != nil {
			t.Fatal(err)
		}
		if len(r.rec.calls) != 2 {
			t.Fatalf("bracketed=%v: %d execs, want 2", bracketed, len(r.rec.calls))
		}
		load, paste := r.rec.calls[0], r.rec.calls[1]
		if !bytes.Equal(load.stdin, payload) || paste.stdin != nil {
			t.Fatalf("stdin: load %q paste %q", load.stdin, paste.stdin)
		}
		for _, c := range r.rec.calls {
			if strings.Contains(strings.Join(c.argv, " "), "secret") {
				t.Fatalf("payload in argv %v", c.argv)
			}
		}
		buf := bufferOf(t, load.argv)
		if !strings.HasPrefix(buf, tmuxctl.BufferPrefix) {
			t.Fatalf("buffer %q lacks prefix", buf)
		}
		build := tmuxctl.ArgvPaste
		if bracketed {
			build = tmuxctl.ArgvPasteBracketed
		}
		wantLoad, wantPaste := build(buf, testName)
		if !slices.Equal(load.argv, inPod(wantLoad)) || !slices.Equal(paste.argv, inPod(wantPaste)) {
			t.Fatalf("argv %v / %v", load.argv, paste.argv)
		}
	}
}

// failPaste fails every paste-buffer, whatever random buffer name it carries.
type failPaste struct{ *recorder }

func (f failPaste) Exec(ctx context.Context, pod string, argv []string, in io.Reader, out, errw io.Writer) (int, error) {
	code, err := f.recorder.Exec(ctx, pod, argv, in, out, errw)
	if slices.Contains(argv, "paste-buffer") {
		return 1, nil
	}
	return code, err
}

func TestPasteFailureDeletesBuffer(t *testing.T) {
	t.Parallel()
	r := newRig(t, func(rec *recorder) Executor { return failPaste{rec} })
	if err := r.c.Paste(context.Background(), testName, []byte("x")); err == nil {
		t.Fatal("failed paste-buffer returned nil")
	}
	got := r.argvs()
	if len(got) != 3 {
		t.Fatalf("%d execs, want load, paste, delete: %v", len(got), got)
	}
	buf := bufferOf(t, got[0])
	if !slices.Equal(got[2], inPod(tmuxctl.ArgvDeleteBuffer(buf))) {
		t.Fatalf("third exec %v is not delete-buffer %s", got[2], buf)
	}
}

func TestPanePID(t *testing.T) {
	t.Parallel()
	cases := map[string]struct {
		out     string
		want    int
		wantErr bool
	}{
		"ok": {"4242\n", 4242, false}, "text": {"abc", 0, true}, "zero": {"0\n", 0, true}, "negative": {"-3", 0, true}, "empty": {"", 0, true},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, nil)
			r.rec.on(strings.Join(inPod(tmuxctl.ArgvPanePID(testName)), " "), reply{stdout: tc.out})
			got, err := r.c.PanePID(context.Background(), testName)
			if tc.wantErr {
				if !errors.Is(err, tmuxctl.ErrUnexpectedOutput) {
					t.Fatalf("err = %v, want ErrUnexpectedOutput", err)
				}
				return
			}
			if err != nil || got != tc.want {
				t.Fatalf("got %d, %v", got, err)
			}
		})
	}
}

func TestCaptureHistory(t *testing.T) {
	t.Parallel()
	argv := strings.Join(inPod(tmuxctl.ArgvCaptureHistory(testName)), " ")
	t.Run("strips ansi", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, nil)
		r.rec.on(argv, reply{stdout: "a\x1b[31mred\x1b[0m\nb\n"})
		got, err := r.c.CaptureHistory(context.Background(), testName)
		if err != nil || got != "ared\nb\n" {
			t.Fatalf("got %q, %v", got, err)
		}
	})
	t.Run("too many lines", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, nil)
		r.rec.on(argv, reply{stdout: strings.Repeat("x\n", tmuxctl.HistoryLimit+1)})
		if _, err := r.c.CaptureHistory(context.Background(), testName); !errors.Is(err, tmuxctl.ErrHistoryTooLarge) {
			t.Fatalf("err = %v", err)
		}
	})
	t.Run("too many bytes", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, nil)
		r.rec.on(argv, reply{stdout: strings.Repeat("x", 4<<20+1)})
		if _, err := r.c.CaptureHistory(context.Background(), testName); !errors.Is(err, tmuxctl.ErrHistoryTooLarge) {
			t.Fatalf("err = %v", err)
		}
	})
}

func TestHas(t *testing.T) {
	t.Parallel()
	has := strings.Join(inPod(tmuxctl.ArgvHas(testName)), " ")
	cases := map[string]struct {
		pod     corev1.PodPhase // "" means no pod
		object  bool
		reply   reply
		want    bool
		wantErr bool
		execs   int
	}{
		"running and present": {corev1.PodRunning, true, reply{}, true, false, 1},
		"running and absent":  {corev1.PodRunning, true, reply{code: 1, stderr: "can't find session: crswd-x"}, false, false, 1},
		"running, no server":  {corev1.PodRunning, true, reply{code: 1, stderr: "no server running on /tmp/x"}, false, false, 1},
		"running, other exit": {corev1.PodRunning, true, reply{code: 2, stderr: "boom"}, false, true, 1},
		"running, transport":  {corev1.PodRunning, true, reply{err: errors.New("down")}, false, true, 1},
		"pending pod":         {corev1.PodPending, true, reply{}, true, false, 0},
		"terminating pod":     {corev1.PodSucceeded, false, reply{}, true, false, 0},
		"object without pod":  {"", true, reply{}, true, false, 0},
		"nothing left":        {"", false, reply{}, false, false, 0},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, nil)
			if tc.pod != "" {
				r.pod(t, testName, tc.pod)
			}
			if tc.object {
				r.object(t, testName, nil, v1alpha1.AgentSessionStatus{})
			}
			r.rec.on(has, tc.reply)
			got, err := r.c.Has(context.Background(), testName)
			if (err != nil) != tc.wantErr || got != tc.want {
				t.Fatalf("got %v, %v; want %v, err=%v", got, err, tc.want, tc.wantErr)
			}
			if len(r.rec.calls) != tc.execs {
				t.Fatalf("%d execs, want %d", len(r.rec.calls), tc.execs)
			}
		})
	}
}

func TestNewCreatesOneObject(t *testing.T) {
	t.Parallel()
	r := newRig(t, nil)
	r.pod(t, testName, corev1.PodRunning)
	if err := r.c.New(context.Background(), testName, "/work/a"); err != nil {
		t.Fatal(err)
	}
	got, ok, err := r.obj.Get(context.Background(), testName)
	if err != nil || !ok {
		t.Fatalf("object missing: %v", err)
	}
	want := v1alpha1.AgentSessionSpec{
		SessionName: "my label", Owner: "alice", WorkDir: "/work/a", StartCommand: "claude",
		Conversation: "11111111-2222-3333-4444-555555555555", Lifetime: "2h30m45s",
	}
	if got.Spec != want {
		t.Fatalf("spec = %+v, want %+v", got.Spec, want)
	}
	if err := r.c.New(context.Background(), testName, "/work/a"); err != nil {
		t.Fatal(err)
	}
	if n := r.creates(); n != 1 {
		t.Fatalf("%d creates, want 1", n)
	}
	if !slices.Equal(r.argvs()[0], inPod(tmuxctl.ArgvHas(testName))) {
		t.Fatalf("readiness probe = %v", r.argvs()[0])
	}
}

func TestNewRefusals(t *testing.T) {
	t.Parallel()
	t.Run("lifetime disabled", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, nil)
		r.c.SetDescriber(func(string) (session.PodRecord, bool) {
			return session.PodRecord{Deadline: testNow.Add(time.Hour), LifetimeDisabled: true}, true
		})
		if err := r.c.New(context.Background(), testName, "/work/a"); err == nil || !strings.Contains(err.Error(), "finite lifetime") {
			t.Fatalf("err = %v", err)
		}
		if r.creates() != 0 {
			t.Fatal("object created for a session with no lifetime")
		}
	})
	t.Run("no describer", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, nil)
		r.c.SetDescriber(nil)
		if err := r.c.New(context.Background(), testName, "/work/a"); err == nil {
			t.Fatal("want error")
		}
	})
	t.Run("unknown session", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, nil)
		if err := r.c.New(context.Background(), "crswd-other", "/work/a"); err == nil || !strings.Contains(err.Error(), "no session record") {
			t.Fatalf("err = %v", err)
		}
		if r.creates() != 0 {
			t.Fatal("object created without a record")
		}
	})
	t.Run("deadline passed", func(t *testing.T) {
		t.Parallel()
		r := newRig(t, nil)
		r.c.SetDescriber(func(string) (session.PodRecord, bool) {
			return session.PodRecord{Deadline: testNow.Add(500 * time.Millisecond)}, true
		})
		if err := r.c.New(context.Background(), testName, "/work/a"); err == nil {
			t.Fatal("want error for a lifetime that truncates to zero")
		}
		if r.creates() != 0 {
			t.Fatal("object created with no lifetime left")
		}
	})
}

func TestNewReportsRejected(t *testing.T) {
	t.Parallel()
	r := newRig(t, nil)
	r.object(t, testName, nil, v1alpha1.AgentSessionStatus{Phase: v1alpha1.PhaseRejected, Reason: "workDir outside the allowed root"})
	err := r.c.New(context.Background(), testName, "/work/a")
	if err == nil || !strings.Contains(err.Error(), "workDir outside the allowed root") {
		t.Fatalf("err = %v", err)
	}
}

func TestNewTimeoutNamesPhase(t *testing.T) {
	t.Parallel()
	r := newRig(t, nil)
	r.pod(t, testName, corev1.PodPending)
	err := r.c.New(context.Background(), testName, "/work/a")
	if err == nil || !strings.Contains(err.Error(), string(corev1.PodPending)) {
		t.Fatalf("err = %v, want one naming Pending", err)
	}
}

func TestSetOptionWritesAnnotation(t *testing.T) {
	t.Parallel()
	r := newRig(t, nil)
	r.object(t, testName, map[string]string{"keep": "me"}, v1alpha1.AgentSessionStatus{})
	if err := r.c.SetOption(context.Background(), testName, tmuxctl.OptionManaged, tmuxctl.OptionManagedValue); err != nil {
		t.Fatal(err)
	}
	got, _, err := r.obj.Get(context.Background(), testName)
	if err != nil {
		t.Fatal(err)
	}
	if got.Metadata.Annotations["crswd.craigcloud.io/managed"] != "1" || got.Metadata.Annotations["keep"] != "me" {
		t.Fatalf("annotations = %v", got.Metadata.Annotations)
	}
}

func TestSetOptionRecordFirst(t *testing.T) {
	t.Parallel()
	r := newRig(t, nil)
	if err := r.c.SetOption(context.Background(), testName, tmuxctl.OptionOwner, "alice"); err == nil {
		t.Fatal("annotating a missing object returned nil")
	}
	if len(r.rec.calls) != 0 {
		t.Fatal("the pod was written before the record")
	}
}

func TestKill(t *testing.T) {
	t.Parallel()
	r := newRig(t, nil)
	r.object(t, testName, nil, v1alpha1.AgentSessionStatus{})
	r.pod(t, testName, corev1.PodRunning)
	err := r.c.Kill(context.Background(), testName)
	if err == nil || !strings.Contains(err.Error(), "still there") {
		t.Fatalf("err = %v, want a still-there error", err)
	}
	if _, ok, err := r.obj.Get(context.Background(), testName); err != nil || ok {
		t.Fatalf("object survived Kill (found=%v, err=%v)", ok, err)
	}
	if err := r.pods.CoreV1().Pods("ns").Delete(context.Background(), testName, metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	r.pods.ClearActions()
	if err := r.c.Kill(context.Background(), testName); err != nil {
		t.Fatalf("Kill after the pod went: %v", err)
	}
	for _, a := range r.pods.Actions() {
		if a.GetVerb() == "delete" {
			t.Fatal("Kill deleted a pod itself")
		}
	}
}

func TestReconcileServerEnvironment(t *testing.T) {
	t.Parallel()
	r := newRig(t, nil)
	got, err := r.c.ReconcileServerEnvironment(context.Background())
	if err != nil || !reflect.DeepEqual(got, tmuxctl.Reconciliation{}) || len(r.rec.calls) != 0 {
		t.Fatalf("got %+v, %v, %d execs", got, err, len(r.rec.calls))
	}
}

// bufferOf reads the buffer name off a load-buffer argv, where it follows -b.
func bufferOf(t *testing.T, argv []string) string {
	t.Helper()
	i := slices.Index(argv, "-b")
	if i < 0 || i+1 >= len(argv) {
		t.Fatalf("no -b in %v", argv)
	}
	return argv[i+1]
}
