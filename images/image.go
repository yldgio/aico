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

// EnsureBuilt builds DefaultTag if it is not present or is outdated. The image
// is considered outdated when its aico.image.hash label doesn't match the hash
// of the current embedded build context. Build output is streamed to the user.
func EnsureBuilt(r *runtime.Runtime) error {
	want := contentHash()

	if r.ImageExists(DefaultTag) {
		// Check if the image is up to date.
		got, _ := r.ImageLabel(DefaultTag, imageVersionLabel)
		if got == want {
			return nil
		}
		fmt.Fprintf(os.Stderr, "aico: image outdated, rebuilding %s...\n", DefaultTag)
	} else {
		fmt.Fprintf(os.Stderr, "aico: building %s (first run, this is a one-time step)...\n", DefaultTag)
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
	if err := r.Run("build", "-t", DefaultTag, "--label", imageVersionLabel+"="+want, dir); err != nil {
		return fmt.Errorf("build %s: %w", DefaultTag, err)
	}
	return nil
}
