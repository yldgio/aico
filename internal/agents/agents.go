// Package agents defines the supported AI coding agents and how their
// host authentication is forwarded into a container.
package agents

import "fmt"

// PathBase identifies which host base directory an auth source is relative to.
type PathBase int

const (
	// BaseHome is the user's home directory (~ / %USERPROFILE%).
	BaseHome PathBase = iota
	// BaseConfig is the user's config directory (~/.config / %APPDATA%).
	BaseConfig
)

// AuthSource describes a host directory of agent credentials that should be
// bind-mounted (read-only) into the container.
type AuthSource struct {
	Base      PathBase // which host base directory Rel is relative to
	Rel       string   // path relative to Base, e.g. ".pi/agent"
	Container string   // absolute destination path inside the container
}

// Agent is a supported coding agent.
type Agent struct {
	Name     string       // user-facing name, e.g. "copilot-cli"
	Command  []string     // command + args to launch inside the container
	FileAuth []AuthSource // host credential dirs to mount read-only
	EnvVars  []string     // host env vars to forward if set
}

// registry holds every supported agent keyed by user-facing name.
var registry = map[string]Agent{
	"pi": {
		Name:    "pi",
		Command: []string{"pi"},
		FileAuth: []AuthSource{
			{Base: BaseHome, Rel: ".pi/agent", Container: "/root/.pi/agent"},
		},
	},
	"opencode": {
		Name:    "opencode",
		Command: []string{"opencode"},
		FileAuth: []AuthSource{
			{Base: BaseConfig, Rel: "opencode", Container: "/root/.config/opencode"},
		},
		EnvVars: []string{"OPENAI_API_KEY", "ANTHROPIC_API_KEY"},
	},
	"copilot-cli": {
		Name:    "copilot-cli",
		Command: []string{"copilot"},
		FileAuth: []AuthSource{
			{Base: BaseHome, Rel: ".copilot", Container: "/root/.copilot"},
			{Base: BaseConfig, Rel: "gh", Container: "/root/.config/gh"},
		},
	},
	"codex": {
		Name:    "codex",
		Command: []string{"codex"},
		EnvVars: []string{"OPENAI_API_KEY"},
	},
	"claude": {
		Name:    "claude",
		Command: []string{"claude"},
		EnvVars: []string{"ANTHROPIC_API_KEY"},
	},
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
