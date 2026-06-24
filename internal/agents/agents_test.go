package agents

import (
	"strings"
	"testing"
)

func TestLookupKnown(t *testing.T) {
	for _, n := range Names() {
		if _, err := Lookup(n); err != nil {
			t.Errorf("Lookup(%q) failed: %v", n, err)
		}
	}
}

func TestLookupUnknownNamesValidAgents(t *testing.T) {
	_, err := Lookup("nope")
	if err == nil {
		t.Fatal("expected error for unknown agent")
	}
	for _, n := range Names() {
		if !strings.Contains(err.Error(), n) {
			t.Errorf("error message missing agent %q: %v", n, err)
		}
	}
}
