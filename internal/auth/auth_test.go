package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yldgio/aico/internal/agents"
)

const testPath = "/tmp/aico-auth-test-project"

func mustLookup(t *testing.T, name string) agents.Agent {
	t.Helper()
	a, err := agents.Lookup(name)
	if err != nil {
		t.Fatalf("Lookup(%q): %v", name, err)
	}
	return a
}

// argsHave reports whether the flag/value pair appears consecutively in args.
func argsHave(args []string, flag, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
}

func TestBuildDefaultUsesProjectIsolatedVolumeNotHostPath(t *testing.T) {
	p := Build(mustLookup(t, "pi"), testPath, false)
	if p.RootMode != RootIsolated {
		t.Errorf("pi default: RootMode = %q, want %q", p.RootMode, RootIsolated)
	}
	wantVol := agents.ProjectVolumeName("pi", agents.AuthVolume{Target: "/root/.pi/agent"}, testPath)
	if !argsHave(p.Args, "-v", wantVol+":/root/.pi/agent") {
		t.Errorf("pi: expected project-scoped root volume mount %q, got %v", wantVol, p.Args)
	}
	if argsHave(p.Args, "-v", "aico-auth-pi:/root/.pi/agent") {
		t.Errorf("pi default run must NOT use the global shared volume: %v", p.Args)
	}
	for _, a := range p.Args {
		if strings.Contains(a, ":ro") {
			t.Errorf("pi default run must not mount anything read-only: %v", p.Args)
		}
		if strings.HasPrefix(a, "/") || strings.Contains(a, ":\\") {
			t.Errorf("pi default run must not reference a host path: %q", a)
		}
	}
}

func TestBuildSharedRootReusesGlobalVolumeAcrossProjects(t *testing.T) {
	p1 := Build(mustLookup(t, "pi"), "/tmp/projectA", true)
	p2 := Build(mustLookup(t, "pi"), "/tmp/projectB", true)
	if p1.RootMode != RootShared || p2.RootMode != RootShared {
		t.Fatalf("expected RootShared for both, got %q and %q", p1.RootMode, p2.RootMode)
	}
	if !argsHave(p1.Args, "-v", "aico-auth-pi:/root/.pi/agent") {
		t.Errorf("projectA: expected shared global volume, got %v", p1.Args)
	}
	if !argsHave(p2.Args, "-v", "aico-auth-pi:/root/.pi/agent") {
		t.Errorf("projectB: expected shared global volume, got %v", p2.Args)
	}
}

func TestBuildDifferentProjectsGetDifferentDefaultVolumes(t *testing.T) {
	pA := Build(mustLookup(t, "pi"), "/tmp/projectA", false)
	pB := Build(mustLookup(t, "pi"), "/tmp/projectB", false)
	if len(pA.RootVolumes) == 0 || len(pB.RootVolumes) == 0 {
		t.Fatalf("expected root volumes to be reported: %v / %v", pA.RootVolumes, pB.RootVolumes)
	}
	if pA.RootVolumes[0] == pB.RootVolumes[0] {
		t.Errorf("expected different default root volumes per project, got %q for both", pA.RootVolumes[0])
	}
}

func TestBuildForwardsEnvKeyByNameOnly(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-secret-should-not-appear")
	p := Build(mustLookup(t, "codex"), testPath, false)
	wantVol := agents.ProjectVolumeName("codex", agents.AuthVolume{Target: "/root/.codex"}, testPath)
	if !argsHave(p.Args, "-v", wantVol+":/root/.codex") {
		t.Errorf("codex: expected root volume, got %v", p.Args)
	}
	if !argsHave(p.Args, "-e", "OPENAI_API_KEY") {
		t.Errorf("codex: expected -e OPENAI_API_KEY, got %v", p.Args)
	}
	for _, a := range p.Args {
		if strings.Contains(a, "=") {
			t.Errorf("no arg may contain a secret value (KEY=VALUE form): %q", a)
		}
		if strings.Contains(a, "sk-secret") {
			t.Fatalf("secret leaked into args: %q", a)
		}
	}
}

func TestBuildClaudeVolumeAndKey(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "x")
	p := Build(mustLookup(t, "claude"), testPath, true)
	if !argsHave(p.Args, "-v", "aico-auth-claude:/root/.claude") {
		t.Errorf("claude: expected shared root volume, got %v", p.Args)
	}
	if !argsHave(p.Args, "-e", "ANTHROPIC_API_KEY") {
		t.Errorf("claude: expected -e ANTHROPIC_API_KEY, got %v", p.Args)
	}
}

func TestBuildOpencodeVolumeTarget(t *testing.T) {
	p := Build(mustLookup(t, "opencode"), testPath, true)
	if !argsHave(p.Args, "-v", "aico-auth-opencode:/root/.local/share/opencode") {
		t.Errorf("opencode: expected data-dir root volume, got %v", p.Args)
	}
}

func TestShareConfigNoLongerAddsBindMount(t *testing.T) {
	// --share-config is deprecated; Build never adds :ro mounts regardless of
	// the shared-root mode.
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", dir)
	if err := os.MkdirAll(filepath.Join(dir, "opencode"), 0o755); err != nil {
		t.Fatal(err)
	}

	p := Build(mustLookup(t, "opencode"), testPath, false)
	for _, a := range p.Args {
		if strings.Contains(a, ":ro") {
			t.Errorf("Build should no longer add :ro mounts (import-config uses docker cp): %v", p.Args)
		}
	}
}

func TestBuildNeverAddsShareConfigMounts(t *testing.T) {
	// With the migration to --import-config (docker cp), Build should never
	// produce :ro bind mounts regardless of the root mode.
	empty := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", empty)
	t.Setenv("APPDATA", empty)
	p := Build(mustLookup(t, "opencode"), testPath, true)
	for _, a := range p.Args {
		if strings.Contains(a, ":ro") {
			t.Errorf("unexpected :ro mount in %v", p.Args)
		}
	}
	if len(p.Warnings) != 0 {
		t.Errorf("no warnings expected when share-config mounts are removed, got %v", p.Warnings)
	}
}

func TestCopilotHasKeyringVolumes(t *testing.T) {
	p := Build(mustLookup(t, "copilot-cli"), testPath, true)
	wantVolumes := []string{
		"aico-auth-copilot-cli:/root/.copilot",
		"aico-auth-copilot-cli-gh:/root/.config/gh",
		"aico-auth-copilot-cli-keyring:/root/.local/share/keyrings",
	}
	for _, want := range wantVolumes {
		if !argsHave(p.Args, "-v", want) {
			t.Errorf("copilot-cli: missing volume %q in %v", want, p.Args)
		}
	}
}
