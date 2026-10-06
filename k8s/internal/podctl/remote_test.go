package podctl

import "testing"

func TestExecutorRemoteImplementsExecutor(t *testing.T) {
	var _ Executor = (*RemoteExecutor)(nil)
}
