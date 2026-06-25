// Package platform centralises every operating-system-specific path decision
// so the rest of aico can stay OS-agnostic. All Windows vs Unix branching for
// auth directories lives here.
//
// aico shells out to the container runtime CLI, which resolves its own per-OS
// socket (e.g. the Windows named pipe) automatically, so the socket is
// deliberately not handled here.
//
// The OS-dependent logic is implemented as pure helpers parameterised by GOOS
// and an environment lookup, so every branch is unit-testable on any host.
package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

// envFunc looks up an environment variable, returning its value and whether it
// was set. It mirrors os.LookupEnv and exists so tests can inject env state.
type envFunc func(string) (string, bool)

// HomeDir returns the current user's home directory. On Windows this resolves
// to %USERPROFILE%; on Unix to $HOME.
func HomeDir() string {
	if h, err := os.UserHomeDir(); err == nil && h != "" {
		return h
	}
	return homeDirFor(runtime.GOOS, os.LookupEnv)
}

func homeDirFor(goos string, env envFunc) string {
	if goos == "windows" {
		if v, ok := env("USERPROFILE"); ok {
			return v
		}
		return ""
	}
	v, _ := env("HOME")
	return v
}

// ConfigDir returns the base directory for per-user config files.
//
//	Windows: %APPDATA%
//	macOS / Linux: $XDG_CONFIG_HOME, falling back to ~/.config
func ConfigDir() string {
	return configDirFor(runtime.GOOS, os.LookupEnv, HomeDir())
}

func configDirFor(goos string, env envFunc, home string) string {
	if goos == "windows" {
		if v, ok := env("APPDATA"); ok && v != "" {
			return v
		}
		return filepath.Join(home, "AppData", "Roaming")
	}
	if v, ok := env("XDG_CONFIG_HOME"); ok && v != "" {
		return v
	}
	return filepath.Join(home, ".config")
}

// WinWorkspace is the POSIX path the project folder is mounted at inside the
// container on Windows hosts (Windows paths like D:\proj are not valid Linux
// working directories).
const WinWorkspace = "/workspace"

// WorkspaceMount returns the bind-mount source (a host path) and target for the
// project folder. The target doubles as the container working directory.
//
// On Unix the folder is mounted at the same absolute path it has on the host,
// so paths the agent prints match the host. On Windows the host path (e.g.
// D:\proj) cannot be a Linux path, so the folder is mounted at /workspace.
func WorkspaceMount(hostAbsPath string) (source, target string) {
	return workspaceMountFor(runtime.GOOS, hostAbsPath)
}

func workspaceMountFor(goos, hostAbsPath string) (source, target string) {
	if goos == "windows" {
		return hostAbsPath, WinWorkspace
	}
	return hostAbsPath, hostAbsPath
}
