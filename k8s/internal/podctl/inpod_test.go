package podctl

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"
)

const testConv = "11111111-2222-3333-4444-555555555555"

func TestCodexConversationArgv(t *testing.T) {
	t.Parallel()
	r := newRig(t, nil)
	r.sessionPod(t)
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
			r.sessionPod(t)
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
			r.sessionPod(t)
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

func TestInPodLookupsWaitForTheSessionContainer(t *testing.T) {
	t.Parallel()
	conv := strings.Join([]string{Binary, "codex-conversation", testName}, " ")
	cases := map[string]struct {
		pod *corev1.Pod
	}{
		"no pod yet":                 {nil},
		"pending":                    {podWith(corev1.PodPending, nil)},
		"running, container waiting": {podWith(corev1.PodRunning, &corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}})},
		"running, no status yet":     {podWith(corev1.PodRunning, nil)},
		"terminating": {func() *corev1.Pod {
			p := podWith(corev1.PodRunning, &corev1.ContainerState{Running: &corev1.ContainerStateRunning{}})
			now := metav1.Now()
			p.DeletionTimestamp = &now
			return p
		}()},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			r := newRig(t, nil)
			if tc.pod != nil {
				tc.pod.Name, tc.pod.Namespace = testName, "ns"
				if _, err := r.pods.CoreV1().Pods("ns").Create(context.Background(), tc.pod, metav1.CreateOptions{}); err != nil {
					t.Fatal(err)
				}
			}
			r.rec.on(conv, reply{stdout: testConv + "\n"})
			got, err := r.c.CodexConversation(context.Background(), testName)
			if got != "" || err != nil {
				t.Fatalf("CodexConversation = %q, %v; want not yet known, no error", got, err)
			}
			ok, err := r.c.HasTranscript(context.Background(), testName, harness.Claude, testConv, "/work/a")
			if ok || err == nil || !errors.Is(err, ErrNotReady) {
				t.Fatalf("HasTranscript = %v, %v; want an ErrNotReady error, never a definite no", ok, err)
			}
			if len(r.rec.calls) != 0 {
				t.Fatalf("exec'd into a pod that cannot take it: %v", r.argvs())
			}
		})
	}
}

func podWith(phase corev1.PodPhase, state *corev1.ContainerState) *corev1.Pod {
	p := &corev1.Pod{Status: corev1.PodStatus{Phase: phase}}
	if state != nil {
		p.Status.ContainerStatuses = []corev1.ContainerStatus{{Name: Container, State: *state}}
	}
	return p
}

// sessionPod adds a pod whose session container is running.
func (r *rig) sessionPod(t *testing.T) {
	t.Helper()
	p := podWith(corev1.PodRunning, &corev1.ContainerState{Running: &corev1.ContainerStateRunning{}})
	p.Name, p.Namespace = testName, "ns"
	if _, err := r.pods.CoreV1().Pods("ns").Create(context.Background(), p, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestInPodLookupsPropagateAPodLookupFailure(t *testing.T) {
	t.Parallel()
	r := newRig(t, nil)
	r.pods.PrependReactor("get", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, errors.New("apiserver down")
	})
	if got, err := r.c.CodexConversation(context.Background(), testName); err == nil || got != "" {
		t.Fatalf("CodexConversation = %q, %v; want the error", got, err)
	}
	if _, err := r.c.HasTranscript(context.Background(), testName, harness.Claude, testConv, "/work/a"); err == nil || errors.Is(err, ErrNotReady) {
		t.Fatalf("HasTranscript err = %v; want a plain error", err)
	}
}
