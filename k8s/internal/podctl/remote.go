package podctl

import (
	"context"
	"errors"
	"io"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"
	"k8s.io/streaming/pkg/httpstream"
)

// RemoteExecutor runs argv through the pods/exec subresource, WebSocket first
// with a SPDY fallback.
type RemoteExecutor struct {
	Config    *rest.Config
	Client    kubernetes.Interface
	Namespace string
}

// Exec implements Executor.
func (r *RemoteExecutor) Exec(ctx context.Context, pod string, argv []string, stdin io.Reader, stdout, stderr io.Writer) (int, error) {
	req := r.Client.CoreV1().RESTClient().Post().Resource("pods").Namespace(r.Namespace).Name(pod).
		SubResource("exec").VersionedParams(&corev1.PodExecOptions{Container: Container, Command: argv,
		Stdin: stdin != nil, Stdout: stdout != nil, Stderr: stderr != nil}, scheme.ParameterCodec)
	ws, err := remotecommand.NewWebSocketExecutor(r.Config, "GET", req.URL().String())
	if err != nil {
		return 0, err
	}
	spdy, err := remotecommand.NewSPDYExecutor(r.Config, "POST", req.URL())
	if err != nil {
		return 0, err
	}
	ex, err := remotecommand.NewFallbackExecutor(ws, spdy, func(err error) bool {
		return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
	})
	if err != nil {
		return 0, err
	}
	err = ex.StreamWithContext(ctx, remotecommand.StreamOptions{Stdin: stdin, Stdout: stdout, Stderr: stderr})
	var ce utilexec.CodeExitError
	if errors.As(err, &ce) {
		return ce.Code, nil
	}
	return 0, err
}
