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

func TestV1AgentsHaveRootVolumeExceptCopilot(t *testing.T) {
	const path = "/tmp/project"
	for _, n := range Names() {
		a, _ := Lookup(n)
		if len(a.AuthVolumes) == 0 {
			t.Errorf("%s: expected at least one root-state AuthVolume", n)
		}
		for _, v := range a.AuthVolumes {
			if v.Target == "" {
				t.Errorf("%s: AuthVolume target must not be empty", n)
			}
			if SharedVolumeName(n, v) != "aico-auth-"+n && v.Suffix == "" {
				t.Errorf("%s: unexpected shared volume name %q", n, SharedVolumeName(n, v))
			}
			if VolumeName(n, v, path, true) != SharedVolumeName(n, v) {
				t.Errorf("%s: VolumeName(shared=true) should match SharedVolumeName", n)
			}
			if VolumeName(n, v, path, false) != ProjectVolumeName(n, v, path) {
				t.Errorf("%s: VolumeName(shared=false) should match ProjectVolumeName", n)
			}
		}
	}
}

func TestProjectVolumeNameDiffersByPath(t *testing.T) {
	a, _ := Lookup("pi")
	v := a.AuthVolumes[0]
	n1 := ProjectVolumeName("pi", v, "/tmp/projectA")
	n2 := ProjectVolumeName("pi", v, "/tmp/projectB")
	if n1 == n2 {
		t.Fatalf("expected different project-scoped volume names for different paths, got %q for both", n1)
	}
	if n1 == SharedVolumeName("pi", v) {
		t.Errorf("project-scoped volume name must not equal the shared name: %q", n1)
	}
}

func TestProjectVolumeNameStableForSamePath(t *testing.T) {
	a, _ := Lookup("pi")
	v := a.AuthVolumes[0]
	const path = "/tmp/project"
	if ProjectVolumeName("pi", v, path) != ProjectVolumeName("pi", v, path) {
		t.Fatalf("ProjectVolumeName must be deterministic for the same path")
	}
}

func TestSharedVolumeNameReusedAcrossPaths(t *testing.T) {
	a, _ := Lookup("pi")
	v := a.AuthVolumes[0]
	n1 := VolumeName("pi", v, "/tmp/projectA", true)
	n2 := VolumeName("pi", v, "/tmp/projectB", true)
	if n1 != n2 {
		t.Fatalf("shared-root mode must resolve to the same volume name regardless of project path: %q vs %q", n1, n2)
	}
	if n1 != "aico-auth-pi" {
		t.Errorf("shared-root pi volume = %q, want aico-auth-pi (preserve pre-existing global volume)", n1)
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
		name := SharedVolumeName("copilot-cli", v)
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
