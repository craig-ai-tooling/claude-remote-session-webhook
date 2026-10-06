package codexauth_test

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nctiggy/claude-remote-session-webhook/internal/codexauth"
)

// golden reads a pane captured from a real Codex process. See testdata/README.md.
func golden(t *testing.T, name string) string {
	t.Helper()

	//nolint:gosec // G304: the path is this test's own literal, joined under testdata.
	body, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatalf("read golden pane: %v", err)
	}
	return string(body)
}

// TestDetectPrompt covers both sign-in screens and the panes that must not match.
//
// **Must fail when** a signed-out Codex session reads as anything but a prompt.
func TestDetectPrompt(t *testing.T) {
	t.Parallel()

	const url = "https://auth.openai.com/codex/device"

	cases := []struct {
		name     string
		pane     string
		wantKind codexauth.Kind
		wantURL  string
		wantCode string
		wantOK   bool
	}{
		{
			name:     "the CLI device-code screen carries link and code",
			pane:     golden(t, "device-code-cli.pane"),
			wantKind: codexauth.KindDeviceCode,
			wantURL:  url,
			wantCode: "ABCD-EFGH1",
			wantOK:   true,
		},
		{
			name:     "the TUI device-code screen carries link and code",
			pane:     golden(t, "device-code-tui.pane"),
			wantKind: codexauth.KindDeviceCode,
			wantURL:  url,
			wantCode: "ABCD-EFGH1",
			wantOK:   true,
		},
		{
			name:     "the signed-out start is named",
			pane:     golden(t, "signed-out.pane"),
			wantKind: codexauth.KindSignedOut,
			wantOK:   true,
		},
		{
			name:     "device code wins when both phrases are on the pane",
			pane:     golden(t, "signed-out.pane") + "\n" + golden(t, "device-code-cli.pane"),
			wantKind: codexauth.KindDeviceCode,
			wantURL:  url,
			wantCode: "ABCD-EFGH1",
			wantOK:   true,
		},
		{
			name:     "a missing URL is still a device-code screen",
			pane:     "2. Enter this one-time code (expires in 15 minutes)\n   ABCD-EFGH1\n",
			wantKind: codexauth.KindDeviceCode,
			wantCode: "ABCD-EFGH1",
			wantOK:   true,
		},
		{
			name:     "a malformed code is dropped, the screen is kept",
			pane:     "https://auth.openai.com/codex/device\nEnter this one-time code\n  not a code\n",
			wantKind: codexauth.KindDeviceCode,
			wantURL:  url,
			wantOK:   true,
		},
		{
			name:     "the code is the first non-empty line after the phrase",
			pane:     "Enter this one-time code\n\n   WXYZ-1234\n",
			wantKind: codexauth.KindDeviceCode,
			wantCode: "WXYZ-1234",
			wantOK:   true,
		},
		{name: "an empty pane matches nothing", pane: "", wantOK: false},
		{name: "a shell prompt matches nothing", pane: "operator@host:~/code$ ", wantOK: false},
		{
			name: "the idle composer matches nothing",
			pane: "› Ask Codex to do anything\n  gpt-5.6-sol default · /home/op/code/repo\n",
		},
		{
			name:   "a quoted signed-out phrase is a mention, not a screen",
			pane:   "const x = \"Sign in with ChatGPT to use Codex as part of your paid plan\"\n",
			wantOK: false,
		},
		{
			name:   "a quoted device-code phrase is a mention, not a screen",
			pane:   "phrase = `Enter this one-time code`\n",
			wantOK: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, ok := codexauth.DetectPrompt(tc.pane)
			if ok != tc.wantOK {
				t.Fatalf("DetectPrompt ok = %v, want %v", ok, tc.wantOK)
			}
			if !tc.wantOK {
				if got != nil {
					t.Fatalf("DetectPrompt returned a prompt with ok=false")
				}
				return
			}
			if got.Kind != tc.wantKind {
				t.Errorf("Kind = %q, want %q", got.Kind, tc.wantKind)
			}
			if got.URL != tc.wantURL {
				t.Errorf("URL = %q, want %q", got.URL, tc.wantURL)
			}
			if got.Code != tc.wantCode {
				t.Errorf("Code = %q, want %q", got.Code, tc.wantCode)
			}
		})
	}
}

// TestPromptStringOmitsCode proves a Prompt formatted by any verb never prints
// the one-time code or the path of the link.
func TestPromptStringOmitsCode(t *testing.T) {
	t.Parallel()

	p := codexauth.Prompt{
		Kind: codexauth.KindDeviceCode,
		URL:  "https://auth.openai.com/codex/device",
		Code: "ABCD-EFGH1",
	}

	for _, rendered := range []string{
		p.String(),
		fmt.Sprintf("%v", p),
		fmt.Sprintf("%s", p), //nolint:staticcheck // S1025: the %s verb is the thing under test
		fmt.Sprintf("%+v", p),
		fmt.Sprintf("%v", &p),
		fmt.Errorf("wrapped: %w", fmt.Errorf("%v", p)).Error(),
	} {
		if strings.Contains(rendered, "ABCD") || strings.Contains(rendered, "EFGH1") {
			t.Errorf("rendered Prompt leaks the code: %q", rendered)
		}
		if strings.Contains(rendered, "/codex/device") {
			t.Errorf("rendered Prompt leaks the URL path: %q", rendered)
		}
	}
	if got := p.String(); !strings.Contains(got, "device-code") || !strings.Contains(got, "auth.openai.com") {
		t.Errorf("String() = %q, want kind and host", got)
	}
}
