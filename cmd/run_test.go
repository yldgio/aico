package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestHasDevenvNix(t *testing.T) {
	dir := t.TempDir()
	if hasDevenvNix(dir) {
		t.Errorf("hasDevenvNix(%q) = true, want false (no devenv.nix)", dir)
	}
	if err := os.WriteFile(filepath.Join(dir, "devenv.nix"), []byte("{ }"), 0o644); err != nil {
		t.Fatalf("write devenv.nix: %v", err)
	}
	if !hasDevenvNix(dir) {
		t.Errorf("hasDevenvNix(%q) = false, want true (devenv.nix present)", dir)
	}
}

func TestDecideDevenvMode(t *testing.T) {
	cases := []struct {
		name     string
		detected bool
		noDevenv bool
		image    string
		want     bool
	}{
		{"detected, no overrides", true, false, "", true},
		{"not detected", false, false, "", false},
		{"detected but --no-devenv", true, true, "", false},
		{"detected but --image given", true, false, "custom:latest", false},
		{"not detected, --no-devenv also set", false, true, "", false},
		{"not detected, --image also set", false, false, "custom:latest", false},
		{"detected, --no-devenv and --image both set", true, true, "custom:latest", false},
		{"not detected, --no-devenv and --image both set", false, true, "custom:latest", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := decideDevenvMode(c.detected, c.noDevenv, c.image); got != c.want {
				t.Errorf("decideDevenvMode(%v, %v, %q) = %v, want %v", c.detected, c.noDevenv, c.image, got, c.want)
			}
		})
	}
}

func TestAgentExecCmd(t *testing.T) {
	// Non-interactive: passthrough, unchanged.
	in := []string{"pi", "-p", "x"}
	if got := agentExecCmd(in, false); !reflect.DeepEqual(got, in) {
		t.Errorf("non-tty passthrough: got %v, want %v", got, in)
	}

	// Interactive: bash wrapper that runs the agent then falls back to a shell.
	got := agentExecCmd([]string{"pi"}, true)
	if len(got) < 5 {
		t.Fatalf("tty wrapper too short: %v", got)
	}
	if got[0] != "bash" || got[1] != "-c" || got[3] != "aico" {
		t.Errorf("tty wrapper prefix = %v, want [bash -c <script> aico ...]", got[:4])
	}
	if last := got[4:]; !reflect.DeepEqual(last, []string{"pi"}) {
		t.Errorf("agent argv = %v, want [pi]", last)
	}
	script := got[2]
	if !strings.Contains(script, `"$@"`) || !strings.Contains(script, "exec bash") {
		t.Errorf("script missing $@ run or shell fallback: %q", script)
	}

	// Args with spaces survive because they are passed as argv, not embedded.
	g2 := agentExecCmd([]string{"pi", "-p", "fix the tests"}, true)
	if last := g2[4:]; !reflect.DeepEqual(last, []string{"pi", "-p", "fix the tests"}) {
		t.Errorf("agent argv with spaces = %v", last)
	}
}

func TestIsAffirmative(t *testing.T) {
	yes := []string{"y", "Y", "yes", "YES", " yes \n", "y\r\n"}
	no := []string{"", "n", "no", "nope", "x", " \n", "yeah"}
	for _, s := range yes {
		if !isAffirmative(s) {
			t.Errorf("isAffirmative(%q) = false, want true", s)
		}
	}
	for _, s := range no {
		if isAffirmative(s) {
			t.Errorf("isAffirmative(%q) = true, want false", s)
		}
	}
}
