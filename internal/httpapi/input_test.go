package httpapi

import "testing"

// A nil bucket would panic on the first typed message, and nothing else in the
// default build would notice until a route called it.
func TestTheInputBudgetIsBuilt(t *testing.T) {
	t.Parallel()

	if newTestServer(t, loopbackListen).inputs == nil {
		t.Fatal("newServer built no input limiter")
	}
}
