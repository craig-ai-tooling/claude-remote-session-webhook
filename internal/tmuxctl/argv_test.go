package tmuxctl

import (
	"context"
	"slices"
	"testing"
)

const argvTestName = "crswd-9f2c4a1b8e6d3f7a0c5b2e9d4f1a7c3b"

// TestArgvWrappersEqualBuilders holds the one property the exported wrappers
// have: each returns, element for element, what the unexported builder of the
// same name returns. A second Controller runs these argv inside a pod, so a
// wrapper that drifted would change the command tmux receives there while every
// host-mode test stayed green.
//
// The inputs include values the builders treat specially: a key beginning with
// "-", and dimensions outside what tmux accepts, which argvResize clamps.
func TestArgvWrappersEqualBuilders(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		got  []string
		want []string
	}{
		{"New", ArgvNew(argvTestName, "/work/repo"), argvNew(argvTestName, "/work/repo")},
		{"SetOption", ArgvSetOption(argvTestName, OptionManaged, "1"), argvSetOption(argvTestName, OptionManaged, "1")},
		{"SendKeys", ArgvSendKeys(argvTestName, "Enter"), argvSendKeys(argvTestName, "Enter")},
		{"SendKeys with a leading dash", ArgvSendKeys(argvTestName, "-x", "C-c"), argvSendKeys(argvTestName, "-x", "C-c")},
		{"SendKeys with no keys", ArgvSendKeys(argvTestName), argvSendKeys(argvTestName)},
		{"CapturePane", ArgvCapturePane(argvTestName), argvCapturePane(argvTestName)},
		{"Resize", ArgvResize(argvTestName, 44, 24), argvResize(argvTestName, 44, 24)},
		{"Resize below the floor", ArgvResize(argvTestName, 0, -3), argvResize(argvTestName, 0, -3)},
		{"Resize above the ceiling", ArgvResize(argvTestName, 20000, 20000), argvResize(argvTestName, 20000, 20000)},
		{"Kill", ArgvKill(argvTestName), argvKill(argvTestName)},
		{"Has", ArgvHas(argvTestName), argvHas(argvTestName)},
		{"ReconcileEnv", ArgvReconcileEnv(), argvReconcileEnv()},
		{"List", ArgvList(), argvList()},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if len(tc.got) == 0 {
				t.Fatalf("argv is empty; want the builder's %v", tc.want)
			}
			if !slices.Equal(tc.got, tc.want) {
				t.Errorf("wrapper argv = %q, builder argv = %q", tc.got, tc.want)
			}
		})
	}

	load, paste := ArgvPaste(argvTestName)
	if want := argvLoadBuffer(argvTestName); !slices.Equal(load, want) {
		t.Errorf("ArgvPaste load-buffer = %q, builder = %q", load, want)
	}
	if want := argvPasteBuffer(argvTestName); !slices.Equal(paste, want) {
		t.Errorf("ArgvPaste paste-buffer = %q, builder = %q", paste, want)
	}
}

// The clamp is the builder's, so it must reach the wrapper's caller too: a pod
// executing ArgvResize(0, 20000) is told 1 and 10000, never the raw request.
func TestArgvResizeClampsForTheExportedCaller(t *testing.T) {
	t.Parallel()

	got := ArgvResize(argvTestName, 0, 20000)
	want := []string{"tmux", "resize-window", "-t", "=" + argvTestName + ":", "-x", "1", "-y", "10000"}
	if !slices.Equal(got, want) {
		t.Errorf("ArgvResize(0, 20000) = %q, want %q", got, want)
	}
}

// A wrapper hands back a slice its caller may append to or edit. If two calls
// shared backing storage, a pod-side caller adding a flag to one command would
// change the next command built.
func TestArgvWrappersReturnFreshSlices(t *testing.T) {
	t.Parallel()

	first := ArgvCapturePane(argvTestName)
	first[0] = "changed"
	if second := ArgvCapturePane(argvTestName); second[0] != "tmux" {
		t.Errorf("a second call returned %q as argv[0]; the wrappers share storage", second[0])
	}
}

// TestArgvWrappersMatchWhatTheFakeRecords is the assertion from the caller's
// side. The fake records the argv the real controller would run, so if it and
// the exported wrappers ever disagree, a pod-side controller built on the
// wrappers is running a command the daemon's own tests never saw.
func TestArgvWrappersMatchWhatTheFakeRecords(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	f := NewFake()

	steps := []struct {
		op   Op
		run  func() error
		want [][]string
	}{
		{OpNew, func() error { return f.New(ctx, argvTestName, "/work") }, [][]string{ArgvNew(argvTestName, "/work")}},
		{OpSetOption, func() error { return f.SetOption(ctx, argvTestName, OptionName, "label") }, [][]string{ArgvSetOption(argvTestName, OptionName, "label")}},
		{OpSendKeys, func() error { return f.SendKeys(ctx, argvTestName, "Enter") }, [][]string{ArgvSendKeys(argvTestName, "Enter")}},
		{OpPaste, func() error { return f.Paste(ctx, argvTestName, []byte("hi")) }, func() [][]string {
			load, paste := ArgvPaste(argvTestName)
			return [][]string{load, paste}
		}()},
		{OpCapturePane, func() error { _, err := f.CapturePane(ctx, argvTestName); return err }, [][]string{ArgvCapturePane(argvTestName)}},
		{OpResize, func() error { return f.Resize(ctx, argvTestName, 44, 24) }, [][]string{ArgvResize(argvTestName, 44, 24)}},
		{OpHas, func() error { _, err := f.Has(ctx, argvTestName); return err }, [][]string{ArgvHas(argvTestName)}},
		{OpList, func() error { _, err := f.List(ctx); return err }, [][]string{ArgvList()}},
		{OpReconcileEnv, func() error { _, err := f.ReconcileServerEnvironment(ctx); return err }, [][]string{ArgvReconcileEnv()}},
		{OpKill, func() error { return f.Kill(ctx, argvTestName) }, [][]string{ArgvKill(argvTestName)}},
	}
	for _, step := range steps {
		before := len(f.Calls())
		if err := step.run(); err != nil {
			t.Fatalf("%s: %v", step.op, err)
		}
		recorded := f.Calls()[before:]
		if len(recorded) != len(step.want) {
			t.Fatalf("%s recorded %d calls, want %d", step.op, len(recorded), len(step.want))
		}
		for i, call := range recorded {
			if call.Op != step.op {
				t.Errorf("%s call %d recorded under %s", step.op, i, call.Op)
			}
			if !slices.Equal(call.Argv, step.want[i]) {
				t.Errorf("%s call %d: fake recorded %q, wrapper says %q", step.op, i, call.Argv, step.want[i])
			}
		}
	}
}
