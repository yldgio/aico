// Package auth builds the container mount and environment arguments needed to
// preserve an agent's login across sessions.
//
// Login is persisted in a per-agent global named volume (aico-auth-<agent>):
// the user logs in once inside the container and stays logged in for every
// future run. Nothing from the host is read by default. API-key credentials are
// forwarded by environment-variable name (never as a value, so the secret never
// appears in the runtime's argv). Host config directories are bind-mounted
// read-only only when the caller opts in with shareConfig.
package auth

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/yldgio/aico/internal/agents"
	"github.com/yldgio/aico/internal/platform"
)

// Plan is the resolved set of runtime arguments for agent auth, plus any
// human-readable warnings (e.g. a shared config directory not found on the host).
type Plan struct {
	Args     []string // runtime args: -v name:target, -e NAME, -v host:target:ro
	Warnings []string // notes about skipped/missing shared config
}

// configHostPath resolves a ConfigSource to an absolute host path.
func configHostPath(s agents.ConfigSource) string {
	switch s.Base {
	case agents.BaseConfig:
		return filepath.Join(platform.ConfigDir(), s.Rel)
	default:
		return filepath.Join(platform.HomeDir(), s.Rel)
	}
}

// Build computes the auth Plan for an agent.
//
// Always: one persistent login volume per AuthVolume, and each set EnvVar
// forwarded by name. When shareConfig is true, each ConfigMount whose host
// directory exists is additionally bind-mounted read-only; missing ones are
// skipped and recorded as warnings.
func Build(a agents.Agent, shareConfig bool) Plan {
	var p Plan

	// Persistent, global per-agent login volumes. Docker auto-creates the named
	// volume on first use, so no host lookup is needed.
	for _, v := range a.AuthVolumes {
		p.Args = append(p.Args, "-v", fmt.Sprintf("%s:%s", agents.VolumeName(a.Name, v), v.Target))
	}

	// API-key env vars: pass by name only (no "=value"). The runtime inherits
	// the value from aico's environment, so the secret never appears in the
	// runtime's argv (where `ps` / /proc/<pid>/cmdline could leak it).
	for _, name := range a.EnvVars {
		if _, ok := os.LookupEnv(name); ok {
			p.Args = append(p.Args, "-e", name)
		}
	}

	// Opt-in host config sharing, read-only.
	if shareConfig {
		for _, src := range a.ConfigMounts {
			host := configHostPath(src)
			if _, err := os.Stat(host); err != nil {
				p.Warnings = append(p.Warnings,
					fmt.Sprintf("--share-config: host config not found for %s: %s (skipped)", a.Name, host))
				continue
			}
			p.Args = append(p.Args, "-v", fmt.Sprintf("%s:%s:ro", host, src.Target))
		}
	}

	return p
}
