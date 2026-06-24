// Package auth builds the container mount and environment arguments needed to
// forward host credentials into an agent container. File-based credentials are
// mounted read-only; API-key credentials are forwarded as environment values.
package auth

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/yldgio/aico/internal/agents"
	"github.com/yldgio/aico/internal/platform"
)

// Plan is the resolved set of runtime arguments for forwarding auth, plus any
// human-readable warnings (e.g. credentials not found on the host).
type Plan struct {
	Args     []string // runtime args: -v ...:...:ro and -e NAME=value pairs
	Warnings []string // notes about skipped/missing credentials
}

// hostPath resolves an AuthSource to an absolute host path.
func hostPath(s agents.AuthSource) string {
	switch s.Base {
	case agents.BaseConfig:
		return filepath.Join(platform.ConfigDir(), s.Rel)
	default:
		return filepath.Join(platform.HomeDir(), s.Rel)
	}
}

// Build computes the auth-forwarding Plan for an agent. Missing file-based
// credentials are skipped silently (recorded as warnings); unset env vars are
// skipped without warning since they may legitimately be provided another way.
func Build(a agents.Agent) Plan {
	var p Plan
	for _, src := range a.FileAuth {
		host := hostPath(src)
		if _, err := os.Stat(host); err != nil {
			p.Warnings = append(p.Warnings,
				fmt.Sprintf("auth not found for %s: %s (agent may fail to authenticate)", a.Name, host))
			continue
		}
		p.Args = append(p.Args, "-v", fmt.Sprintf("%s:%s:ro", host, src.Container))
	}
	for _, name := range a.EnvVars {
		if _, ok := os.LookupEnv(name); ok {
			// Pass the variable by name only (no "=value"). The runtime inherits
			// the value from aico's environment, so the secret never appears in
			// the runtime's argv (where `ps` / /proc/<pid>/cmdline could leak it).
			p.Args = append(p.Args, "-e", name)
		}
	}
	return p
}
