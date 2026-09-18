// Package image owns the built-in agent image: the embedded Dockerfile, the
// entrypoint scripts, and the logic to build/rebuild the image on demand.
// It lives alongside the Dockerfile so `docker build images/` and the embedded
// copy are always the same files.
package image

import (
	"crypto/sha256"
	"embed"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/yldgio/aico/internal/runtime"
)

// DefaultTag is the image built and used when the user does not pass --image.
const DefaultTag = "aico-agents:latest"

// DevenvTag is the devenv-enabled image, built lazily on first devenv-mode
// run from the Dockerfile's `devenv` target (Nix + devenv CLI on top of the
// same base stage as DefaultTag).
const DevenvTag = "aico-agents-devenv:latest"

// imageVersionLabel is the Docker label used to detect stale images.
const imageVersionLabel = "aico.image.hash"

//go:embed Dockerfile copilot-entrypoint.sh
var buildContext embed.FS

// contentHash computes a short hash of all embedded build-context files.
// When this changes, the image must be rebuilt.
func contentHash() string {
	h := sha256.New()
	_ = fs.WalkDir(buildContext, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		data, _ := buildContext.ReadFile(path)
		h.Write([]byte(path))
		h.Write(data)
		return nil
	})
	return fmt.Sprintf("%x", h.Sum(nil))[:12]
}

// EnsureBuilt builds DefaultTag from the Dockerfile's `base` target if it is
// not present or is outdated. The image is considered outdated when its
// aico.image.hash label doesn't match the hash of the current embedded build
// context. Build output is streamed to the user.
func EnsureBuilt(r *runtime.Runtime) error {
	return ensureBuilt(r, DefaultTag, "base")
}

// EnsureDevenvBuilt builds DevenvTag from the Dockerfile's `devenv` target
// (Nix + devenv CLI on top of the base stage) if it is not present or is
// outdated, using the same content-hash staleness mechanism as EnsureBuilt.
// The first build installs Nix and can take several minutes; output is
// streamed to the user.
func EnsureDevenvBuilt(r *runtime.Runtime) error {
	return ensureBuilt(r, DevenvTag, "devenv")
}

// ensureBuilt builds tag from the given Dockerfile target if it is not
// present or is outdated, per contentHash/imageVersionLabel.
func ensureBuilt(r *runtime.Runtime, tag, target string) error {
	want := contentHash()

	if r.ImageExists(tag) {
		// Check if the image is up to date.
		got, _ := r.ImageLabel(tag, imageVersionLabel)
		if got == want {
			return nil
		}
		fmt.Fprintf(os.Stderr, "aico: image outdated, rebuilding %s...\n", tag)
	} else {
		fmt.Fprintf(os.Stderr, "aico: building %s (first run, this is a one-time step)...\n", tag)
	}

	dir, err := os.MkdirTemp("", "aico-build-*")
	if err != nil {
		return fmt.Errorf("create build context: %w", err)
	}
	defer os.RemoveAll(dir)

	// Write all embedded files into the build context directory.
	err = fs.WalkDir(buildContext, ".", func(path string, d fs.DirEntry, walkErr error) error {
		if walkErr != nil || d.IsDir() {
			return walkErr
		}
		data, readErr := buildContext.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		dest := filepath.Join(dir, path)
		return os.WriteFile(dest, data, 0o644)
	})
	if err != nil {
		return fmt.Errorf("write build context: %w", err)
	}

	// Build with the content hash as a label so future runs detect staleness.
	// --target is required because the Dockerfile has multiple stages and
	// docker build defaults to the last one in the file.
	if err := r.Run("build", "--target", target, "-t", tag, "--label", imageVersionLabel+"="+want, dir); err != nil {
		return fmt.Errorf("build %s: %w", tag, err)
	}
	return nil
}
