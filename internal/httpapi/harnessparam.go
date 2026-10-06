package httpapi

import (
	"errors"
	"net/http"
	"net/url"

	"github.com/nctiggy/claude-remote-session-webhook/internal/harness"
)

const (
	// fieldHarness is the form field that names which agent a create starts.
	fieldHarness = "harness"

	harnessClaudeValue = "claude"
	harnessCodexValue  = "codex"

	// codexStartCommandName is the configured name a Codex create runs. It is a
	// fixed literal of this daemon, never read from the field: the field picks one
	// of two harnesses and the daemon resolves that to a command.
	codexStartCommandName = "codex"
)

// errHarnessParam is a harness value this daemon does not accept. A sentinel
// authored here so a trail never echoes what the caller sent, which on this field
// can be a configured command name or a command line.
var errHarnessParam = errors.New("the harness value is not one this daemon accepts")

// parseHarness reads key from values and is the one reader every route uses for a
// harness value. Absent is Claude, the same safe direction offersRemoteControlState
// reads a missing field in. Present, it must be exactly one entry equal to "claude"
// or "codex"; a repeated, empty or differently spelled value is refused.
//
// url.Values is indexed rather than read with Get because Get flattens absent,
// empty and repeated to the same empty string.
func parseHarness(values url.Values, key string) (harness.Name, error) {
	entries, present := values[key]
	if !present {
		return harness.Claude, nil
	}
	if len(entries) != 1 {
		return "", errHarnessParam
	}
	switch entries[0] {
	case harnessClaudeValue:
		return harness.Claude, nil
	case harnessCodexValue:
		return harness.Codex, nil
	}
	return "", errHarnessParam
}

// parseHarnessQuery reads key from r's raw query. r.URL.Query() discards a pair
// it cannot parse, so "?harness=codex&harness=claude;x=1" would read as one
// value; parsing RawQuery directly makes any malformed pair a refusal instead.
// It returns the parsed values too, for a route that reads other keys.
func parseHarnessQuery(r *http.Request, key string) (harness.Name, url.Values, error) {
	values, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return "", nil, errHarnessParam
	}
	h, err := parseHarness(values, key)
	return h, values, err
}
