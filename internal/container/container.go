// Package container derives deterministic container identities from an agent
// name and a project path. Identity is pure: the same agent+path always maps to
// the same container name, with no lockfile or labels required.
package container

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
)

// Hash returns the first 8 hex chars of the SHA-256 of absPath. absPath should
// already be an absolute, cleaned path so the identity is stable.
func Hash(absPath string) string {
	sum := sha256.Sum256([]byte(absPath))
	return hex.EncodeToString(sum[:])[:8]
}

// Name returns the deterministic container name for an agent + project path,
// in the form aico-<agent>-<hash>.
func Name(agent, absPath string) string {
	return fmt.Sprintf("aico-%s-%s", agent, Hash(absPath))
}

// VolumeName returns the deterministic fallback named-volume for an agent +
// project path, used to preserve state if the container itself is removed.
func VolumeName(agent, absPath string) string {
	return fmt.Sprintf("aico-%s-%s-data", agent, Hash(absPath))
}
