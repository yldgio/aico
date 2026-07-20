// Package auth builds the container mount and environment arguments needed to
// preserve an agent's root state (login + settings) across sessions.
//
// By default, root state is persisted in a volume scoped to the current
// project (agent + absolute path): each project gets its own isolated root, so
// logging into one project's container never surfaces in another project's
// container. Passing sharedRoot switches to a single global per-agent volume
// (aico-auth-<agent>), the same name aico has always used, so opting in
// reuses any pre-existing global volume rather than creating a new one.
//
// Nothing from the host is read by default. API-key credentials are forwarded
// by environment-variable name (never as a value, so the secret never appears
// in the runtime's argv). Host config directories are bind-mounted read-only
// only when the caller opts in with shareConfig (deprecated/ignored; see
// --import-config, which copies instead of mounting).
package auth

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/yldgio/aico/internal/agents"
	"github.com/yldgio/aico/internal/platform"
)

// RootMode identifies which root-volume strategy a Plan was built with.
type RootMode string

const (
	// RootIsolated is the default: one root-volume set per project.
	RootIsolated RootMode = "project-isolated"
	// RootShared is the --shared-root opt-in: one global root-volume set per agent.
	RootShared RootMode = "shared-root"
)

// Plan is the resolved set of runtime arguments for agent auth, plus any
// human-readable warnings (e.g. a shared config directory not found on the host).
type Plan struct {
	Args        []string // runtime args: -v name:target, -e NAME
	Warnings    []string // notes about skipped/missing shared config
	RootMode    RootMode // which root-volume strategy was selected
	RootVolumes []string // resolved root-volume names, for --dry-run/reporting
}

// ConfigHostPath resolves a ConfigSource to an absolute host path.
func ConfigHostPath(s agents.ConfigSource) string {
	switch s.Base {
	case agents.BaseConfig:
		return filepath.Join(platform.ConfigDir(), s.Rel)
	default:
		return filepath.Join(platform.HomeDir(), s.Rel)
	}
}

// Build computes the auth Plan for an agent.
//
// absPath is the absolute project path; it is used to derive project-scoped
// root-volume names when sharedRoot is false. When sharedRoot is true, the
// agent's original global per-agent volume names are used instead, reusing
// any volume that already exists from a previous --shared-root run.
//
// Always: one persistent root volume per AuthVolume, and each set EnvVar
// forwarded by name.
func Build(a agents.Agent, absPath string, sharedRoot bool) Plan {
	var p Plan
	if sharedRoot {
		p.RootMode = RootShared
	} else {
		p.RootMode = RootIsolated
	}

	// Persistent root volumes. Docker auto-creates the named volume on first
	// use, so no host lookup is needed.
	for _, v := range a.AuthVolumes {
		volName := agents.VolumeName(a.Name, v, absPath, sharedRoot)
		p.RootVolumes = append(p.RootVolumes, volName)
		p.Args = append(p.Args, "-v", fmt.Sprintf("%s:%s", volName, v.Target))
	}

	// API-key env vars: pass by name only (no "=value"). The runtime inherits
	// the value from aico's environment, so the secret never appears in the
	// runtime's argv (where `ps` / /proc/<pid>/cmdline could leak it).
	for _, name := range a.EnvVars {
		if _, ok := os.LookupEnv(name); ok {
			p.Args = append(p.Args, "-e", name)
		}
	}

	return p
}
