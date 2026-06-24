// Package image owns the built-in agent image: the embedded Dockerfile and the
// logic to build it on demand when it is not already present locally. It lives
// alongside the Dockerfile so `docker build images/` and the embedded copy are
// always the same file.
package image

import (
	"embed"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yldgio/aico/internal/runtime"
)

// DefaultTag is the image built and used when the user does not pass --image.
const DefaultTag = "aico-agents:latest"

//go:embed Dockerfile
var dockerfiles embed.FS

// Dockerfile returns the embedded Dockerfile contents.
func Dockerfile() ([]byte, error) { return dockerfiles.ReadFile("Dockerfile") }

// EnsureBuilt builds DefaultTag if it is not already present locally. It is a
// no-op when the image exists. Build output is streamed to the user.
func EnsureBuilt(r *runtime.Runtime) error {
	if r.ImageExists(DefaultTag) {
		return nil
	}
	fmt.Fprintf(os.Stderr, "aico: building %s (first run, this is a one-time step)...\n", DefaultTag)

	dir, err := os.MkdirTemp("", "aico-build-*")
	if err != nil {
		return fmt.Errorf("create build context: %w", err)
	}
	defer os.RemoveAll(dir)

	df, err := Dockerfile()
	if err != nil {
		return fmt.Errorf("read embedded Dockerfile: %w", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "Dockerfile"), df, 0o644); err != nil {
		return fmt.Errorf("write Dockerfile: %w", err)
	}
	if err := r.Run("build", "-t", DefaultTag, dir); err != nil {
		return fmt.Errorf("build %s: %w", DefaultTag, err)
	}
	return nil
}
