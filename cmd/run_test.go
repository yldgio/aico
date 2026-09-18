package cmd

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/yldgio/aico/internal/runtime"
)

// TestMain lets this test binary double as a fake runtime CLI: with
// AICO_TEST_FAKE_BIN=1 it records its own argv to AICO_TEST_FAKE_BIN_RECORD
// and exits 0 instead of running tests. This is the same stub pattern as
// internal/runtime/runtime_test.go, and lets the argv aico would hand to
// docker/podman be asserted on Windows without a real runtime installed.
func TestMain(m *testing.M) {
	if os.Getenv("AICO_TEST_FAKE_BIN") == "1" {
		_ = os.WriteFile(os.Getenv("AICO_TEST_FAKE_BIN_RECORD"),
			[]byte(strings.Join(os.Args[1:], "\x00")), 0o644)
		os.Exit(0)
	}
	os.Exit(m.Run())
}

// fakeRuntime returns a Runtime whose Bin is this test binary re-invoked as a
// stub, plus a reader for the argv it was last called with.
func fakeRuntime(t *testing.T) (*runtime.Runtime, func() []string) {
	t.Helper()
	record := filepath.Join(t.TempDir(), "argv")
	self, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	t.Setenv("AICO_TEST_FAKE_BIN", "1")
	t.Setenv("AICO_TEST_FAKE_BIN_RECORD", record)
	return &runtime.Runtime{Bin: self}, func() []string {
		data, err := os.ReadFile(record)
		if err != nil {
			t.Fatalf("read record: %v", err)
		}
		return strings.Split(string(data), "\x00")
	}
}

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
	if got := agentExecCmd(in, false, false); !reflect.DeepEqual(got, in) {
		t.Errorf("non-tty passthrough: got %v, want %v", got, in)
	}

	// Interactive: bash wrapper that runs the agent then falls back to a shell.
	got := agentExecCmd([]string{"pi"}, true, false)
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
	g2 := agentExecCmd([]string{"pi", "-p", "fix the tests"}, true, false)
	if last := g2[4:]; !reflect.DeepEqual(last, []string{"pi", "-p", "fix the tests"}) {
		t.Errorf("agent argv with spaces = %v", last)
	}
}

func TestDevenvWrap(t *testing.T) {
	in := []string{"pi", "-p", "x"}
	if got := devenvWrap(in, false); !reflect.DeepEqual(got, in) {
		t.Errorf("devenvWrap(off) = %v, want %v", got, in)
	}
	want := []string{"devenv", "shell", "pi", "-p", "x"}
	if got := devenvWrap(in, true); !reflect.DeepEqual(got, want) {
		t.Errorf("devenvWrap(on) = %v, want %v", got, want)
	}
	// The input slice must not be mutated or aliased.
	if !reflect.DeepEqual(in, []string{"pi", "-p", "x"}) {
		t.Errorf("devenvWrap mutated its input: %v", in)
	}
}

func TestAgentExecCmdDevenv(t *testing.T) {
	// Non-interactive devenv: the agent runs inside `devenv shell`.
	want := []string{"devenv", "shell", "pi"}
	if got := agentExecCmd([]string{"pi"}, false, true); !reflect.DeepEqual(got, want) {
		t.Errorf("non-tty devenv = %v, want %v", got, want)
	}
	// Interactive devenv: agent and the fallback shell both run in the env.
	got := agentExecCmd([]string{"pi"}, true, true)
	if last := got[4:]; !reflect.DeepEqual(last, []string{"devenv", "shell", "pi"}) {
		t.Errorf("tty devenv agent argv = %v, want [devenv shell pi]", last)
	}
	if !strings.Contains(got[2], "exec devenv shell bash") {
		t.Errorf("tty devenv fallback shell not in env: %q", got[2])
	}
}

func TestContainerLabelsDevenv(t *testing.T) {
	plain := containerLabels("pi", "/p", "pi-p", false)
	if strings.Contains(strings.Join(plain, " "), labelDevenv) {
		t.Errorf("non-devenv container got a devenv label: %v", plain)
	}
	dev := containerLabels("pi", "/p", "pi-p", true)
	if !reflect.DeepEqual(dev[len(dev)-2:], []string{"--label", "aico.devenv=true"}) {
		t.Errorf("devenv labels = %v, want trailing --label aico.devenv=true", dev)
	}
}

func TestCommonContainerArgsDevenv(t *testing.T) {
	plain := commonContainerArgs("aico-pi-abc", "/proj", "/workspace", "pi", "/proj", "pi-proj", false, []string{"-v", "aico-auth-pi:/root"})
	if joined := strings.Join(plain, " "); strings.Contains(joined, "aico-nix") {
		t.Errorf("non-devenv args mount the nix volume: %v", plain)
	}
	dev := commonContainerArgs("aico-pi-abc", "/proj", "/workspace", "pi", "/proj", "pi-proj", true, []string{"-v", "aico-auth-pi:/root"})
	if !containsPair(dev, "-v", nixVolumeArg) {
		t.Errorf("devenv args missing -v %s: %v", nixVolumeArg, dev)
	}
	// Auth args still come last so they are unaffected by devenv mode.
	if got := dev[len(dev)-2:]; !reflect.DeepEqual(got, []string{"-v", "aico-auth-pi:/root"}) {
		t.Errorf("auth args not preserved at the end: %v", dev)
	}
}

// launchArgv builds the argv for every launch path with a fixed, readable
// input set, so tests can assert what aico hands to the runtime CLI.
func launchArgv(devenv bool) (common, agentCmd []string, image string) {
	image = "aico-agents:latest"
	if devenv {
		image = "aico-agents-devenv:latest"
	}
	common = commonContainerArgs("aico-pi-abc123", "/proj", "/workspace", "pi", "/proj", "pi-proj", devenv, nil)
	return common, []string{"pi"}, image
}

func TestLaunchPathsArgv(t *testing.T) {
	for _, devenv := range []bool{false, true} {
		common, agentCmd, image := launchArgv(devenv)

		t.Run(pathLabel("interactive-run", devenv), func(t *testing.T) {
			rt, argv := fakeRuntime(t)
			if err := rt.Run(launchArgs("run", "-it", image, common, devenvWrap(agentCmd, devenv))...); err != nil {
				t.Fatalf("Run: %v", err)
			}
			assertLaunchArgv(t, argv(), "run", image, devenv, []string{"pi"})
		})

		t.Run(pathLabel("create-start", devenv), func(t *testing.T) {
			rt, argv := fakeRuntime(t)
			if _, err := rt.Output(launchArgs("create", "-it", image, common, devenvWrap(agentCmd, devenv))...); err != nil {
				t.Fatalf("Output: %v", err)
			}
			assertLaunchArgv(t, argv(), "create", image, devenv, []string{"pi"})
		})

		t.Run(pathLabel("detach-create", devenv), func(t *testing.T) {
			rt, argv := fakeRuntime(t)
			if _, err := rt.Output(detachCreateArgs(image, common)...); err != nil {
				t.Fatalf("Output: %v", err)
			}
			got := argv()
			assertLaunchArgv(t, got, "run", image, devenv, []string{"sleep", "infinity"})
			if got[1] != "-d" {
				t.Errorf("detached create missing -d: %v", got)
			}
		})

		t.Run(pathLabel("detach-exec", devenv), func(t *testing.T) {
			rt, argv := fakeRuntime(t)
			if err := rt.Exec("aico-pi-abc123", false, agentExecCmd(agentCmd, false, devenv)...); err != nil {
				t.Fatalf("Exec: %v", err)
			}
			got := argv()
			want := []string{"exec", "-i", "aico-pi-abc123", "pi"}
			if devenv {
				want = []string{"exec", "-i", "aico-pi-abc123", "devenv", "shell", "pi"}
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("exec argv = %v, want %v", got, want)
			}
		})
	}
}

func pathLabel(name string, devenv bool) string {
	if devenv {
		return name + "/devenv"
	}
	return name + "/plain"
}

// assertLaunchArgv checks the shared shape of a container-creating argv: the
// verb, the image, the devenv volume/label (present only in devenv mode), and
// the command that follows the image.
func assertLaunchArgv(t *testing.T, got []string, verb, image string, devenv bool, cmdAfterImage []string) {
	t.Helper()
	if got[0] != verb {
		t.Errorf("verb = %q, want %q (argv %v)", got[0], verb, got)
	}
	idx := -1
	for i, a := range got {
		if a == image {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatalf("image %q not in argv %v", image, got)
	}
	wantCmd := cmdAfterImage
	if devenv && cmdAfterImage[0] == "pi" {
		wantCmd = append([]string{"devenv", "shell"}, cmdAfterImage...)
	}
	if !reflect.DeepEqual(got[idx+1:], wantCmd) {
		t.Errorf("command after image = %v, want %v", got[idx+1:], wantCmd)
	}
	hasNix := containsPair(got, "-v", nixVolumeArg)
	hasLabel := containsPair(got, "--label", "aico.devenv=true")
	if hasNix != devenv || hasLabel != devenv {
		t.Errorf("devenv=%v but nix volume=%v, devenv label=%v (argv %v)", devenv, hasNix, hasLabel, got)
	}
}

func containsPair(args []string, flag, value string) bool {
	for i := 0; i+1 < len(args); i++ {
		if args[i] == flag && args[i+1] == value {
			return true
		}
	}
	return false
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
