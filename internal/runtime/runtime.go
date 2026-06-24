// Package runtime is a thin, vendor-independent wrapper over a container CLI
// (docker, podman, or any OCI-compatible drop-in). No other package in aico
// references a runtime binary by name: swapping in a new runtime means editing
// only this file.
package runtime

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// candidates is the auto-detection order when no explicit runtime is chosen.
var candidates = []string{"docker", "podman"}

// Runtime is a resolved container CLI.
type Runtime struct {
	Bin string // the runtime binary name, e.g. "docker"
}

// Resolve picks the runtime binary name using, in priority order:
//
//  1. the --runtime flag value (override)
//  2. the AICO_RUNTIME environment variable
//  3. auto-detection of candidates on PATH
//
// It returns an empty string if nothing is selected. Resolve never checks that
// an explicitly chosen runtime exists on PATH; that is deferred to Verify so
// that --dry-run can resolve a runtime that is not installed locally.
func Resolve(override string) string {
	if override != "" {
		return override
	}
	if env := os.Getenv("AICO_RUNTIME"); env != "" {
		return env
	}
	for _, c := range candidates {
		if _, err := exec.LookPath(c); err == nil {
			return c
		}
	}
	return ""
}

// Detect resolves and verifies a runtime, returning a ready-to-use Runtime or a
// user-actionable error.
func Detect(override string) (*Runtime, error) {
	bin := Resolve(override)
	if bin == "" {
		return nil, fmt.Errorf(
			"no container runtime found: tried %s\n\nfix: install Docker or Podman, "+
				"or set AICO_RUNTIME / --runtime to your runtime binary",
			strings.Join(candidates, ", "))
	}
	r := &Runtime{Bin: bin}
	if err := r.Verify(); err != nil {
		return nil, err
	}
	return r, nil
}

// Verify checks that the runtime binary is present on PATH.
func (r *Runtime) Verify() error {
	if _, err := exec.LookPath(r.Bin); err != nil {
		return fmt.Errorf(
			"container runtime %q not found on PATH\n\nfix: install it, or set "+
				"AICO_RUNTIME / --runtime to a runtime that is installed", r.Bin)
	}
	return nil
}

// Run executes the runtime with args, wiring the child process to the current
// process's stdio. Use it for interactive commands (run/start/attach).
func (r *Runtime) Run(args ...string) error {
	cmd := exec.Command(r.Bin, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// Output runs the runtime with args and returns trimmed stdout.
func (r *Runtime) Output(args ...string) (string, error) {
	out, err := exec.Command(r.Bin, args...).Output()
	return strings.TrimSpace(string(out)), err
}

// Inspect returns the raw `inspect --format {{format}}` value for ref.
func (r *Runtime) Inspect(ref, format string) (string, error) {
	return r.Output("inspect", "--format", format, ref)
}

// Exists reports whether a container named name exists (running or stopped).
func (r *Runtime) Exists(name string) bool {
	_, err := r.Output("container", "inspect", name)
	return err == nil
}

// Running reports whether a container named name exists and is running.
func (r *Runtime) Running(name string) bool {
	state, err := r.Inspect(name, "{{.State.Running}}")
	return err == nil && state == "true"
}

// ImageExists reports whether an image tag is present locally.
func (r *Runtime) ImageExists(tag string) bool {
	_, err := r.Output("image", "inspect", tag)
	return err == nil
}

// Start starts an existing stopped container and attaches to it interactively.
func (r *Runtime) Start(name string) error { return r.Run("start", "-ai", name) }

// Attach attaches to an already-running container.
func (r *Runtime) Attach(name string) error { return r.Run("attach", name) }

// Stop stops a running container.
func (r *Runtime) Stop(name string) error { return r.Run("stop", name) }

// Remove force-removes a container, ignoring "no such container".
func (r *Runtime) Remove(name string) error {
	_ = exec.Command(r.Bin, "rm", "-f", name).Run()
	return nil
}
