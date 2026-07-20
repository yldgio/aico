// Package agents defines the supported AI coding agents and how their root
// state (login + settings) is persisted across container sessions.
//
// Auth model (see specs/auth-volumes.md and specs/shared-root-opt-in.md): each
// agent's root state lives in a named Docker volume. By default that volume is
// scoped per project (agent + project path), so separate projects using the
// same agent never share login/config state. Passing --shared-root switches to
// a single global per-agent volume (aico-auth-<agent>), preserving the
// original "log in once, every project shares it" behavior for users who want
// it. Nothing from the host is mounted by default. The user can opt in to
// importing host config with --import-config (a one-time copy, unrelated to
// --shared-root).
package agents

import (
	"fmt"

	"github.com/yldgio/aico/internal/container"
)

// PathBase identifies which host base directory a config source is relative to.
type PathBase int

const (
	// BaseHome is the user's home directory (~ / %USERPROFILE%).
	BaseHome PathBase = iota
	// BaseConfig is the user's config directory (~/.config / %APPDATA%).
	BaseConfig
)

// AuthVolume is a named Docker volume that persists part of an agent's root
// state (login, settings, session data) across container runs.
//
// The resolved volume name depends on the root mode selected at run time (see
// SharedVolumeName and ProjectVolumeName):
//   - project-isolated (default): aico-auth-<agent>-<pathHash>[-<Suffix>]
//   - shared (--shared-root):     aico-auth-<agent>[-<Suffix>]
type AuthVolume struct {
	Suffix string // optional volume-name suffix; empty => no suffix
	Target string // absolute container path where the volume is mounted
}

// ConfigSource is a host config directory that is bind-mounted read-only into
// the container only when the user passes --share-config. It must point at a
// directory that is separate from any AuthVolume target (otherwise it would
// collide with the persistent root volume).
type ConfigSource struct {
	Base   PathBase // which host base directory Rel is relative to
	Rel    string   // path relative to Base, e.g. "opencode"
	Target string   // absolute destination path inside the container
}

// Agent is a supported coding agent.
type Agent struct {
	Name         string         // user-facing name, e.g. "copilot-cli"
	Command      []string       // command + args to launch inside the container
	AuthVolumes  []AuthVolume   // root-state volumes persisted across sessions
	ConfigMounts []ConfigSource // host config dirs shared only with --share-config
	EnvVars      []string       // host env vars to forward by name if set
}

// registry holds every supported agent keyed by user-facing name.
//
// AuthVolume targets are the agent's *Linux* login location, because login
// happens inside the Linux container. For pi, codex and claude the login and
// settings share one directory, so config travels inside the volume and there
// is no separable ConfigMount. opencode keeps config (~/.config/opencode)
// separate from its login (~/.local/share/opencode), so it has a ConfigMount.
//
// copilot-cli is intentionally without an AuthVolume: it stores its token in
// the system keyring (libsecret), not a file, so persisting it requires the
// keyring machinery deferred to v2 (see specs/auth-volumes.md). Without a
// volume its login simply does not persist; aico never writes a clear-text
// token to a volume.
var registry = map[string]Agent{
	"pi": {
		Name:    "pi",
		Command: []string{"pi"},
		AuthVolumes: []AuthVolume{
			{Target: "/root/.pi/agent"},
		},
	},
	"opencode": {
		Name:    "opencode",
		Command: []string{"opencode"},
		AuthVolumes: []AuthVolume{
			{Target: "/root/.local/share/opencode"},
		},
		ConfigMounts: []ConfigSource{
			{Base: BaseConfig, Rel: "opencode", Target: "/root/.config/opencode"},
		},
		EnvVars: []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY"},
	},
	"copilot-cli": {
		Name:    "copilot-cli",
		Command: []string{"/usr/local/bin/copilot-entrypoint.sh"},
		AuthVolumes: []AuthVolume{
			{Target: "/root/.copilot"},
			{Suffix: "gh", Target: "/root/.config/gh"},
			{Suffix: "keyring", Target: "/root/.local/share/keyrings"},
		},
	},
	"codex": {
		Name:    "codex",
		Command: []string{"codex"},
		AuthVolumes: []AuthVolume{
			{Target: "/root/.codex"},
		},
		EnvVars: []string{"OPENAI_API_KEY"},
	},
	"claude": {
		Name:    "claude",
		Command: []string{"claude"},
		AuthVolumes: []AuthVolume{
			{Target: "/root/.claude"},
		},
		EnvVars: []string{"ANTHROPIC_API_KEY"},
	},
}

// SharedVolumeName returns the global per-agent volume name for an AuthVolume,
// reused by every project when --shared-root is passed. This is the same
// naming scheme aico has always used (aico-auth-<agent>[-<Suffix>]), preserved
// so existing global volumes keep working once a user opts into sharing.
func SharedVolumeName(agent string, v AuthVolume) string {
	if v.Suffix == "" {
		return "aico-auth-" + agent
	}
	return "aico-auth-" + agent + "-" + v.Suffix
}

// ProjectVolumeName returns the default, project-scoped volume name for an
// AuthVolume: aico-auth-<agent>-<pathHash>[-<Suffix>]. absPath must already be
// an absolute, cleaned path (see internal/container.Hash) so the name is
// stable across invocations for the same project.
func ProjectVolumeName(agent string, v AuthVolume, absPath string) string {
	name := "aico-auth-" + agent + "-" + container.Hash(absPath)
	if v.Suffix != "" {
		name += "-" + v.Suffix
	}
	return name
}

// VolumeName returns the resolved volume name for an AuthVolume given the
// requested root mode: SharedVolumeName when shared is true, otherwise
// ProjectVolumeName (the default, isolated-per-project behavior).
func VolumeName(agent string, v AuthVolume, absPath string, shared bool) string {
	if shared {
		return SharedVolumeName(agent, v)
	}
	return ProjectVolumeName(agent, v, absPath)
}

// Names returns the sorted list of supported agent names.
func Names() []string {
	return []string{"pi", "opencode", "copilot-cli", "codex", "claude"}
}

// Lookup returns the agent definition for name, or an error naming the valid
// agents if name is unknown.
func Lookup(name string) (Agent, error) {
	a, ok := registry[name]
	if !ok {
		return Agent{}, fmt.Errorf(
			"unknown agent %q\n\nsupported agents: %v\nexample: aico run pi",
			name, Names())
	}
	return a, nil
}
