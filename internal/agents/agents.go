// Package agents defines the supported AI coding agents and how their login is
// persisted across container sessions.
//
// Auth model (see specs/auth-volumes.md): each agent's login is preserved in a
// per-agent, global named volume (aico-auth-<agent>). The user logs in once
// inside the container and stays logged in for every future run of that agent.
// Nothing from the host is mounted by default. The user can opt in to sharing
// host config directories read-only with --share-config, but only where the
// config dir is separate from the login store.
package agents

import "fmt"

// PathBase identifies which host base directory a config source is relative to.
type PathBase int

const (
	// BaseHome is the user's home directory (~ / %USERPROFILE%).
	BaseHome PathBase = iota
	// BaseConfig is the user's config directory (~/.config / %APPDATA%).
	BaseConfig
)

// AuthVolume is a named Docker volume that persists an agent's login across
// sessions. The volume name is aico-auth-<agent>[-<Suffix>]; it is global per
// agent (shared by every project) so logging in once keeps you logged in.
type AuthVolume struct {
	Suffix string // optional volume-name suffix; empty => aico-auth-<agent>
	Target string // absolute container path where the volume is mounted
}

// ConfigSource is a host config directory that is bind-mounted read-only into
// the container only when the user passes --share-config. It must point at a
// directory that is separate from any AuthVolume target (otherwise it would
// collide with the persistent login volume).
type ConfigSource struct {
	Base   PathBase // which host base directory Rel is relative to
	Rel    string   // path relative to Base, e.g. "opencode"
	Target string   // absolute destination path inside the container
}

// Agent is a supported coding agent.
type Agent struct {
	Name         string         // user-facing name, e.g. "copilot-cli"
	Command      []string       // command + args to launch inside the container
	AuthVolumes  []AuthVolume   // login volumes persisted across sessions
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

// VolumeName returns the global named volume for an AuthVolume of agent name.
func VolumeName(agent string, v AuthVolume) string {
	if v.Suffix == "" {
		return "aico-auth-" + agent
	}
	return "aico-auth-" + agent + "-" + v.Suffix
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
