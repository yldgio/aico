package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yldgio/aico/internal/agents"
	"github.com/yldgio/aico/internal/container"
	"github.com/yldgio/aico/internal/runtime"
)

func newRmCmd() *cobra.Command {
	var withVolumes bool
	var rtOverride string
	c := &cobra.Command{
		Use:   "rm <name|agent> [path]",
		Short: "Remove an aico container",
		Long: "Remove a specific aico container by name or by agent + path.\n\n" +
			"By default the agent's auth volumes are kept (so you stay logged in\n" +
			"if you recreate). Pass --volumes to also remove the auth volumes.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ""
			if len(args) >= 2 {
				path = args[1]
			}
			return rmContainer(args[0], path, withVolumes, rtOverride)
		},
	}
	c.Flags().BoolVar(&withVolumes, "volumes", false, "also remove the agent's auth volumes (you will need to re-login)")
	c.Flags().StringVar(&rtOverride, "runtime", "", "container runtime to use")
	return c
}

func rmContainer(nameOrAgent, path string, withVolumes bool, rtOverride string) error {
	rt, err := runtime.Detect(rtOverride)
	if err != nil {
		return err
	}

	var cName, agentName string

	// If it's a known agent, resolve by agent+path.
	if _, lookupErr := agents.Lookup(nameOrAgent); lookupErr == nil {
		absPath, err := resolvePath(path)
		if err != nil {
			return err
		}
		cName = container.Name(nameOrAgent, absPath)
		agentName = nameOrAgent
	} else {
		// Resolve by aico.name label.
		var found bool
		cName, agentName, found = findContainerByName(rt, nameOrAgent)
		if !found {
			return fmt.Errorf("no container named %q\n\nfix: use `aico ls` to see available containers", nameOrAgent)
		}
	}

	if !rt.Exists(cName) {
		return fmt.Errorf("container %s does not exist", cName)
	}

	// Remove container (force-stops if running).
	if err := rt.Remove(cName); err != nil {
		return fmt.Errorf("remove container: %w", err)
	}
	fmt.Fprintf(os.Stderr, "removed container %s\n", cName)

	if withVolumes && agentName != "" {
		removeAgentVolumes(rt, agentName)
	}

	return nil
}

func removeAgentVolumes(rt *runtime.Runtime, agentName string) {
	agent, err := agents.Lookup(agentName)
	if err != nil {
		return
	}
	for _, v := range agent.AuthVolumes {
		volName := agents.VolumeName(agentName, v)
		if _, err := rt.Output("volume", "rm", "-f", volName); err == nil {
			fmt.Fprintf(os.Stderr, "removed volume %s\n", volName)
		}
	}
}

func newPurgeCmd() *cobra.Command {
	var rtOverride string
	c := &cobra.Command{
		Use:   "purge",
		Short: "Remove all aico containers, volumes, and the agent image",
		Long: "Nuclear reset: removes every aico container, all auth volumes,\n" +
			"and the shared agent image. The aico binary itself is kept.\n\n" +
			"Use `aico uninstall` to also remove the binary.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return purge(rtOverride)
		},
	}
	c.Flags().StringVar(&rtOverride, "runtime", "", "container runtime to use")
	return c
}

func purge(rtOverride string) error {
	rt, err := detectRuntime(rtOverride)
	if err != nil {
		return nil
	}

	// Remove all aico containers.
	fmt.Fprintln(os.Stderr, "› removing all aico containers...")
	removeAllContainers(rt)

	// Remove all aico-auth-* volumes.
	fmt.Fprintln(os.Stderr, "› removing auth volumes...")
	volOut, _ := rt.Output("volume", "ls", "--format", "{{.Name}}")
	for _, vol := range strings.Split(strings.TrimSpace(volOut), "\n") {
		vol = strings.TrimSpace(vol)
		if !strings.HasPrefix(vol, "aico-auth-") {
			continue
		}
		rt.Output("volume", "rm", "-f", vol)
		fmt.Fprintf(os.Stderr, "  removed %s\n", vol)
	}

	// Remove the agent image.
	fmt.Fprintln(os.Stderr, "› removing agent image...")
	if _, err := rt.Output("rmi", "-f", "aico-agents:latest"); err != nil {
		fmt.Fprintln(os.Stderr, "  image not found or already removed")
	} else {
		fmt.Fprintln(os.Stderr, "  removed aico-agents:latest")
	}

	fmt.Fprintln(os.Stderr, "\n✓ purge complete. Run `aico run <agent>` to start fresh.")
	return nil
}

// detectRuntime is a helper that returns nil if no runtime is found.
func detectRuntime(override string) (*runtime.Runtime, error) {
	rt, err := runtime.Detect(override)
	if err != nil {
		fmt.Fprintln(os.Stderr, "aico: no container runtime found; skipping container cleanup")
		return nil, err
	}
	return rt, nil
}

// removeAllContainers removes all aico containers (by label + name pattern).
func removeAllContainers(rt *runtime.Runtime) {
	out, _ := rt.Output("ps", "-a", "--filter", "label="+labelAgent, "--format", "{{.Names}}")
	for _, cn := range strings.Split(strings.TrimSpace(out), "\n") {
		cn = strings.TrimSpace(cn)
		if cn == "" {
			continue
		}
		_ = rt.Remove(cn)
		fmt.Fprintf(os.Stderr, "  removed %s\n", cn)
	}
	// Also catch pre-label containers by name pattern.
	for _, name := range agents.Names() {
		out, _ := rt.Output("ps", "-a", "--filter", "name=aico-"+name+"-", "--format", "{{.Names}}")
		for _, cn := range strings.Split(strings.TrimSpace(out), "\n") {
			cn = strings.TrimSpace(cn)
			if cn == "" {
				continue
			}
			_ = rt.Remove(cn)
			fmt.Fprintf(os.Stderr, "  removed %s\n", cn)
		}
	}
}
