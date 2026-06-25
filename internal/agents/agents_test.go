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

func TestCopilotHasThreeVolumes(t *testing.T) {
	a, _ := Lookup("copilot-cli")
	if len(a.AuthVolumes) != 3 {
		t.Fatalf("copilot-cli: expected 3 AuthVolumes, got %d", len(a.AuthVolumes))
	}
	want := map[string]string{
		"aico-auth-copilot-cli":         "/root/.copilot",
		"aico-auth-copilot-cli-gh":      "/root/.config/gh",
		"aico-auth-copilot-cli-keyring": "/root/.local/share/keyrings",
	}
	for _, v := range a.AuthVolumes {
		name := VolumeName("copilot-cli", v)
		expTarget, ok := want[name]
		if !ok {
			t.Errorf("unexpected volume %q -> %q", name, v.Target)
			continue
		}
		if v.Target != expTarget {
			t.Errorf("%s: target = %q, want %q", name, v.Target, expTarget)
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
