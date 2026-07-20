package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"
	"github.com/yldgio/aico/internal/agents"
	"github.com/yldgio/aico/internal/runtime"
)

// Label keys stored on every aico container.
const (
	labelAgent  = "aico.agent"
	labelPath   = "aico.path"
	labelName   = "aico.name"
	labelDevenv = "aico.devenv"
	labelRoot   = "aico.root" // root-volume mode: "project-isolated" or "shared-root"
)

// containerLabels returns the --label args for a new container. rootMode is
// the auth.RootMode string the container was created with (see
// internal/auth), recorded so a later `aico run`/`aico bake` can detect a
// mismatch (e.g. --shared-root passed against a container that was created
// project-isolated) instead of silently reusing the wrong root volumes. The
// aico.devenv label is only set for devenv-mode containers, so containers for
// non-devenv projects are labelled exactly as before.
func containerLabels(agentName, absPath, name, rootMode string, devenv bool) []string {
	labels := []string{
		"--label", labelAgent + "=" + agentName,
		"--label", labelPath + "=" + absPath,
		"--label", labelName + "=" + name,
		"--label", labelRoot + "=" + rootMode,
	}
	if devenv {
		labels = append(labels, "--label", labelDevenv+"=true")
	}
	return labels
}

// containerDevenv reports whether an existing container was created in devenv
// mode, by reading its aico.devenv label.
func containerDevenv(rt *runtime.Runtime, name string) bool {
	v, err := rt.Inspect(name, fmt.Sprintf("{{index .Config.Labels %q}}", labelDevenv))
	return err == nil && strings.TrimSpace(v) == "true"
}

// containerRootMode returns the aico.root label value for cName, or "" if the
// container predates this label -- i.e. a legacy container created under the
// old shared-by-default behavior.
func containerRootMode(rt *runtime.Runtime, cName string) string {
	v, _ := rt.Output("inspect", "--format",
		fmt.Sprintf("{{index .Config.Labels %q}}", labelRoot), cName)
	return strings.TrimSpace(v)
}

// resolveContainerName determines the short name for a container.
// Uses --name if provided, otherwise <agent>-<folder-basename>.
func resolveContainerName(explicit, agentName, absPath string) string {
	if explicit != "" {
		return explicit
	}
	return agentName + "-" + filepath.Base(absPath)
}

// findContainerByName looks up a container by its aico.name label.
// Returns the docker container name (aico-<agent>-<hash>), its agent name,
// and its project path (from the aico.path label; empty for containers
// created before labels existed).
func findContainerByName(rt *runtime.Runtime, name string) (containerName, agentName, absPath string, found bool) {
	// Query all containers with the matching aico.name label.
	out, err := rt.Output("ps", "-a", "--filter", "label="+labelName+"="+name,
		"--format", "{{.Names}}")
	if err != nil || strings.TrimSpace(out) == "" {
		return "", "", "", false
	}
	cName := strings.Split(strings.TrimSpace(out), "\n")[0]

	// Get the agent and path from labels.
	agent, _ := rt.Output("inspect", "--format",
		fmt.Sprintf("{{index .Config.Labels %q}}", labelAgent), cName)
	path, _ := rt.Output("inspect", "--format",
		fmt.Sprintf("{{index .Config.Labels %q}}", labelPath), cName)
	return cName, strings.TrimSpace(agent), strings.TrimSpace(path), true
}

// runByName resumes a container identified by its aico.name label.
func runByName(name string, extraArgs []string, o *runOpts) error {
	rt, err := runtime.Detect(o.runtime)
	if err != nil {
		return err
	}

	cName, agentName, _, found := findContainerByName(rt, name)
	if !found {
		return fmt.Errorf("no container named %q\n\nfix: use `aico ls` to see available containers, or create one with `aico run <agent> [path] --name %s`", name, name)
	}

	agent, err := agents.Lookup(agentName)
	if err != nil {
		return fmt.Errorf("container %q has unknown agent %q", name, agentName)
	}

	agentCmd := append([]string{}, agent.Command...)
	agentCmd = append(agentCmd, extraArgs...)
	// A devenv-mode container must run its agent inside `devenv shell`.
	agentCmd = devenvWrap(agentCmd, containerDevenv(rt, cName))

	if rt.Running(cName) {
		return rt.Exec(cName, isTTY(), agentCmd...)
	}
	if isDetached(rt, cName) {
		if err := rt.StartBackground(cName); err != nil {
			return fmt.Errorf("start container: %w", err)
		}
		return rt.Exec(cName, isTTY(), agentCmd...)
	}
	return rt.Start(cName)
}

// newLsCmd creates the `aico ls` command.
func newLsCmd() *cobra.Command {
	var rtOverride string
	c := &cobra.Command{
		Use:   "ls",
		Short: "List all aico containers",
		Long:  "List all aico containers with their name, agent, project path, and status.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return listContainers(rtOverride)
		},
	}
	c.Flags().StringVar(&rtOverride, "runtime", "", "container runtime to use")
	return c
}

func listContainers(rtOverride string) error {
	rt, err := runtime.Detect(rtOverride)
	if err != nil {
		return err
	}

	// List all containers that have the aico.agent label.
	out, err := rt.Output("ps", "-a", "--filter", "label="+labelAgent,
		"--format", "{{.Names}}\t{{.Status}}")
	if err != nil {
		return err
	}

	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		fmt.Fprintln(os.Stderr, "no aico containers found")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "NAME\tAGENT\tPATH\tSTATUS")

	for _, line := range lines {
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) < 2 {
			continue
		}
		cName := strings.TrimSpace(parts[0])
		status := strings.TrimSpace(parts[1])

		aName, _ := rt.Output("inspect", "--format",
			fmt.Sprintf("{{index .Config.Labels %q}}", labelAgent), cName)
		aPath, _ := rt.Output("inspect", "--format",
			fmt.Sprintf("{{index .Config.Labels %q}}", labelPath), cName)
		aLabel, _ := rt.Output("inspect", "--format",
			fmt.Sprintf("{{index .Config.Labels %q}}", labelName), cName)

		// Shorten status to running/stopped.
		shortStatus := "stopped"
		if strings.HasPrefix(status, "Up") {
			shortStatus = "running"
		}

		displayName := strings.TrimSpace(aLabel)
		if displayName == "" {
			displayName = cName // fallback for containers created before labels
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n",
			displayName,
			strings.TrimSpace(aName),
			strings.TrimSpace(aPath),
			shortStatus)
	}
	w.Flush()
	return nil
}
