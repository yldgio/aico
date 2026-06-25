package platform

import (
	"path/filepath"
	"testing"
)

// envMap builds an envFunc from a map for deterministic testing.
func envMap(m map[string]string) envFunc {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func TestConfigDirWindowsUsesAppData(t *testing.T) {
	got := configDirFor("windows", envMap(map[string]string{
		"APPDATA": `C:\Users\dev\AppData\Roaming`,
	}), `C:\Users\dev`)
	want := `C:\Users\dev\AppData\Roaming`
	if got != want {
		t.Fatalf("windows config dir = %q, want %q", got, want)
	}
}

func TestConfigDirWindowsFallsBackToProfile(t *testing.T) {
	got := configDirFor("windows", envMap(map[string]string{}), `C:\Users\dev`)
	want := filepath.Join(`C:\Users\dev`, "AppData", "Roaming")
	if got != want {
		t.Fatalf("windows config fallback = %q, want %q", got, want)
	}
}

func TestConfigDirUnixUsesXDG(t *testing.T) {
	got := configDirFor("linux", envMap(map[string]string{
		"XDG_CONFIG_HOME": "/custom/cfg",
	}), "/home/dev")
	if got != "/custom/cfg" {
		t.Fatalf("xdg config dir = %q, want /custom/cfg", got)
	}
}

func TestConfigDirUnixFallsBackToDotConfig(t *testing.T) {
	got := configDirFor("darwin", envMap(map[string]string{}), "/home/dev")
	if got != "/home/dev/.config" {
		t.Fatalf("unix config fallback = %q, want /home/dev/.config", got)
	}
}

func TestHomeDirWindowsUsesUserProfile(t *testing.T) {
	got := homeDirFor("windows", envMap(map[string]string{
		"USERPROFILE": `C:\Users\dev`,
	}))
	if got != `C:\Users\dev` {
		t.Fatalf("windows home = %q, want C:\\Users\\dev", got)
	}
}

func TestWorkspaceMountUnixUsesSamePath(t *testing.T) {
	src, target := workspaceMountFor("linux", "/home/dev/project")
	if src != "/home/dev/project" || target != "/home/dev/project" {
		t.Fatalf("unix mount = (%q,%q), want same host path on both sides", src, target)
	}
}

func TestWorkspaceMountWindowsMapsToPosix(t *testing.T) {
	src, target := workspaceMountFor("windows", `D:\projects\foundry\ag-001`)
	if src != `D:\projects\foundry\ag-001` {
		t.Fatalf("windows mount source = %q, want the host path", src)
	}
	if target != WinWorkspace {
		t.Fatalf("windows mount target = %q, want %q (a POSIX path)", target, WinWorkspace)
	}
	if target == "" || target[0] != '/' {
		t.Fatalf("windows container path %q must be an absolute POSIX path", target)
	}
}
