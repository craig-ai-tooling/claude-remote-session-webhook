package tmuxctl

// The exported argv builders. A second Controller (spec 017, kubernetes mode)
// runs these same commands inside a session pod through the Kubernetes exec API,
// so the bytes that reach a pane stay the ones this package composed (FR-002).
// It has to reach them from another package, and the builders in fake.go are
// unexported on purpose: renaming them would edit every test that names one,
// and the tmux-tagged suite among them.
//
// Each wrapper is one call to the builder of the same name and adds nothing.
// That is the property TestArgvWrappersEqualBuilders holds, and it is why there
// is no logic here to review: a wrapper that did anything else would be a
// second definition of the contract in tmuxctl.md.
//
// None of them carries the -L socket flag. Exec.args prepends it, and it is
// Exec's business; a caller running these somewhere other than the daemon's own
// tmux server adds its own server selection ahead of argv[1:].

// ArgvNew is `tmux new-session`: a detached session running the login shell
// only, in workDir.
func ArgvNew(name, workDir string) []string { return argvNew(name, workDir) }

// ArgvSetOption is `tmux set-option` on the session's active pane target.
func ArgvSetOption(name, option, value string) []string {
	return argvSetOption(name, option, value)
}

// ArgvSendKeys carries daemon-authored key constants only. Caller text goes
// through ArgvPaste, for the reason argvSendKeys states.
func ArgvSendKeys(name string, keys ...string) []string { return argvSendKeys(name, keys...) }

// ArgvPaste returns the two commands a paste is, in the order Exec.Paste runs
// them. The caller's payload rides on the standard input of the first and never
// appears in either argv.
//
// There is no single paste builder, so this returns a pair rather than
// composing one: a wrapper that joined the two would be an argv tmux never
// receives.
func ArgvPaste(name string) (loadBuffer, pasteBuffer []string) {
	return argvLoadBuffer(name), argvPasteBuffer(name)
}

// ArgvCapturePane is `tmux capture-pane -p`, without -e.
func ArgvCapturePane(name string) []string { return argvCapturePane(name) }

// ArgvResize is `tmux resize-window`. The dimensions are clamped by the
// builder, so this cannot hand tmux a number it rejects.
func ArgvResize(name string, cols, rows int) []string { return argvResize(name, cols, rows) }

// ArgvKill is `tmux kill-session`.
func ArgvKill(name string) []string { return argvKill(name) }

// ArgvHas is `tmux has-session`.
func ArgvHas(name string) []string { return argvHas(name) }

// ArgvReconcileEnv is the read half of the environment reconciliation.
func ArgvReconcileEnv() []string { return argvReconcileEnv() }

// ArgvList is `tmux list-sessions` with the format string parseSessions reads.
func ArgvList() []string { return argvList() }
