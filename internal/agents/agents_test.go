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

func TestV1AgentsHaveLoginVolumeExceptCopilot(t *testing.T) {
	for _, n := range Names() {
		a, _ := Lookup(n)
		if n == "copilot-cli" {
			if len(a.AuthVolumes) != 0 {
				t.Errorf("copilot-cli must have no AuthVolumes in v1, got %v", a.AuthVolumes)
			}
			continue
		}
		if len(a.AuthVolumes) == 0 {
			t.Errorf("%s: expected at least one login AuthVolume", n)
		}
		for _, v := range a.AuthVolumes {
			if v.Target == "" {
				t.Errorf("%s: AuthVolume target must not be empty", n)
			}
			if VolumeName(n, v) != "aico-auth-"+n && v.Suffix == "" {
				t.Errorf("%s: unexpected volume name %q", n, VolumeName(n, v))
			}
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
