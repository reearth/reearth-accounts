package main

import "testing"

// Every log call in the server goes through applog with typed fields
// (ADR-0003 decision 9). This fails when a printf-style call comes back.
func TestNoPrintfStyleLogCalls(t *testing.T) {
	n, skipped, err := run([]string{"../../internal", "../../pkg", "../../cmd/reearth-accounts-admin"}, false)
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range skipped {
		t.Errorf("%s: %s", s.pos, s.reason)
	}
	if n > 0 {
		t.Errorf("%d log calls are not converted; run: go run ./tools/logcodemod ./internal ./pkg ./cmd/reearth-accounts-admin", n)
	}
}
