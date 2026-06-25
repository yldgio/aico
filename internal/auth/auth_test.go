package auth

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/yldgio/aico/internal/agents"
)

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

func TestBuildUsesPersistentVolumeNotHostPath(t *testing.T) {
	p := Build(mustLookup(t, "pi"), false)
	if !argsHave(p.Args, "-v", "aico-auth-pi:/root/.pi/agent") {
		t.Errorf("pi: expected login volume mount, got %v", p.Args)
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

func TestBuildForwardsEnvKeyByNameOnly(t *testing.T) {
	t.Setenv("OPENAI_API_KEY", "sk-secret-should-not-appear")
	p := Build(mustLookup(t, "codex"), false)
	if !argsHave(p.Args, "-v", "aico-auth-codex:/root/.codex") {
		t.Errorf("codex: expected login volume, got %v", p.Args)
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
	p := Build(mustLookup(t, "claude"), false)
	if !argsHave(p.Args, "-v", "aico-auth-claude:/root/.claude") {
		t.Errorf("claude: expected login volume, got %v", p.Args)
	}
	if !argsHave(p.Args, "-e", "ANTHROPIC_API_KEY") {
		t.Errorf("claude: expected -e ANTHROPIC_API_KEY, got %v", p.Args)
	}
}

func TestBuildOpencodeVolumeTarget(t *testing.T) {
	p := Build(mustLookup(t, "opencode"), false)
	if !argsHave(p.Args, "-v", "aico-auth-opencode:/root/.local/share/opencode") {
		t.Errorf("opencode: expected data-dir login volume, got %v", p.Args)
	}
}

func TestShareConfigAddsReadOnlyMountWhenPresent(t *testing.T) {
	dir := t.TempDir()
	// On Windows, ConfigDir() checks APPDATA before XDG_CONFIG_HOME.
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("APPDATA", dir)
	if err := os.MkdirAll(filepath.Join(dir, "opencode"), 0o755); err != nil {
		t.Fatal(err)
	}

	without := Build(mustLookup(t, "opencode"), false)
	for _, a := range without.Args {
		if strings.Contains(a, ":ro") {
			t.Fatalf("without --share-config there must be no :ro mount: %v", without.Args)
		}
	}

	with := Build(mustLookup(t, "opencode"), true)
	want := filepath.Join(dir, "opencode") + ":/root/.config/opencode:ro"
	if !argsHave(with.Args, "-v", want) {
		t.Errorf("--share-config: expected %q in %v", want, with.Args)
	}
}

func TestShareConfigMissingDirWarnsAndSkips(t *testing.T) {
	empty := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", empty)
	t.Setenv("APPDATA", empty) // Windows: ConfigDir() checks APPDATA first
	p := Build(mustLookup(t, "opencode"), true)
	for _, a := range p.Args {
		if strings.Contains(a, ":ro") {
			t.Errorf("missing host config must not be mounted: %v", p.Args)
		}
	}
	if len(p.Warnings) == 0 {
		t.Errorf("missing shared config should produce a warning")
	}
}

func TestCopilotHasKeyringVolumes(t *testing.T) {
	p := Build(mustLookup(t, "copilot-cli"), false)
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
