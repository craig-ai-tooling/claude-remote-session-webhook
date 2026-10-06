package httpapi

import (
	"errors"
	"net/url"
	"testing"

	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
)

func TestParseHarness(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		values  url.Values
		want    harness.Name
		wantErr bool
	}{
		{"absent", url.Values{}, harness.Claude, false},
		{"claude", url.Values{fieldHarness: {"claude"}}, harness.Claude, false},
		{"codex", url.Values{fieldHarness: {"codex"}}, harness.Codex, false},
		{"wrong case", url.Values{fieldHarness: {"Codex"}}, "", true},
		{"empty", url.Values{fieldHarness: {""}}, "", true},
		{"present with no entries", url.Values{fieldHarness: {}}, "", true},
		{"duplicate", url.Values{fieldHarness: {"codex", "codex"}}, "", true},
		{"codex and claude", url.Values{fieldHarness: {"codex", "claude"}}, "", true},
		{"unknown", url.Values{fieldHarness: {"evil"}}, "", true},
		{"a configured command line", url.Values{fieldHarness: {"codex --dangerously-bypass-approvals-and-sandbox"}}, "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, err := parseHarness(tt.values, fieldHarness)
			if tt.wantErr {
				if !errors.Is(err, errHarnessParam) {
					t.Fatalf("err = %v, want errHarnessParam", err)
				}
				if got != "" {
					t.Errorf("harness = %q alongside an error, want none", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("err = %v, want nil", err)
			}
			if got != tt.want {
				t.Errorf("harness = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseHarnessIgnoresOtherKeys(t *testing.T) {
	t.Parallel()

	got, err := parseHarness(url.Values{"other": {"codex"}}, fieldHarness)
	if err != nil || got != harness.Claude {
		t.Errorf("parseHarness = (%q, %v), want (claude, nil)", got, err)
	}
}
