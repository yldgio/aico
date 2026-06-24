package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	images "github.com/yldgio/aico/images"
	"github.com/yldgio/aico/internal/agents"
	"github.com/yldgio/aico/internal/auth"
	"github.com/yldgio/aico/internal/container"
	"github.com/yldgio/aico/internal/runtime"
)

type runOpts struct {
	newContainer bool
	image        string
	runtime      string
	verbose      bool
	dryRun       bool
}

func newRunCmd() *cobra.Command {
	o := &runOpts{}
	c := &cobra.Command{
		Use:   "run <agent> [path]",
		Short: "Run or resume an agent container for a project folder",
		Long: "Run or resume an agent container for a project folder.\n\n" +
			"<agent> is one of: pi, opencode, copilot-cli, codex, claude\n" +
			"[path]  is the project folder to mount (defaults to the current directory).\n\n" +
			"On first use a container is created; subsequent runs on the same path\n" +
			"resume it. Use --new to discard and recreate it.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ""
			if len(args) == 2 {
				path = args[1]
			}
			return runAgent(args[0], path, o)
		},
	}
	f := c.Flags()
	f.BoolVar(&o.newContainer, "new", false, "discard any existing container and create a fresh one")
	f.StringVar(&o.image, "image", "", "use a custom image instead of the built-in agent image")
	f.StringVar(&o.runtime, "runtime", "", "container runtime to use (default: auto-detect docker, then podman)")
	f.BoolVar(&o.verbose, "verbose", false, "print warnings, e.g. when host credentials are missing")
	f.BoolVar(&o.dryRun, "dry-run", false, "print what would run without creating a container")
	return c
}

func runAgent(agentName, path string, o *runOpts) error {
	agent, err := agents.Lookup(agentName)
	if err != nil {
		return err
	}

	absPath, err := resolvePath(path)
	if err != nil {
		return err
	}

	rtBin := runtime.Resolve(o.runtime)
	image := o.image
	if image == "" {
		image = images.DefaultTag
	}
	name := container.Name(agent.Name, absPath)
	authPlan := auth.Build(agent)

	if o.verbose {
		for _, w := range authPlan.Warnings {
			fmt.Fprintln(os.Stderr, "aico: warning: "+w)
		}
	}

	// Assemble the create command (used for a fresh container).
	createArgs := []string{"run", "-it", "--name", name,
		"-v", fmt.Sprintf("%s:%s", absPath, absPath), "-w", absPath}
	createArgs = append(createArgs, authPlan.Args...)
	createArgs = append(createArgs, image)
	createArgs = append(createArgs, agent.Command...)

	if o.dryRun {
		printDryRun(rtBin, image, name, absPath, createArgs)
		return nil
	}

	rt, err := runtime.Detect(o.runtime)
	if err != nil {
		return err
	}

	if o.newContainer {
		_ = rt.Remove(name)
	}

	// Resume path: an existing container is reused unless --new was given.
	if !o.newContainer && rt.Exists(name) {
		if rt.Running(name) {
			return rt.Attach(name)
		}
		return rt.Start(name)
	}

	// Fresh container: ensure the image exists (unless the user supplied one),
	// then create + start + attach in a single interactive run.
	if o.image == "" {
		if err := images.EnsureBuilt(rt); err != nil {
			return err
		}
	}
	return rt.Run(createArgs...)
}

// resolvePath converts an optional user path (default: cwd) into a cleaned
// absolute path, erroring if it does not exist.
func resolvePath(path string) (string, error) {
	if path == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", fmt.Errorf("cannot determine current directory: %w", err)
		}
		path = cwd
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path %q: %w", path, err)
	}
	if _, err := os.Stat(abs); err != nil {
		return "", fmt.Errorf("project path does not exist: %s\n\nfix: pass an existing folder, e.g. aico run pi .", abs)
	}
	return abs, nil
}

func printDryRun(rtBin, image, name, absPath string, createArgs []string) {
	if rtBin == "" {
		rtBin = "(none detected — install docker or podman)"
	}
	fmt.Printf("[dry-run] runtime:   %s\n", rtBin)
	fmt.Printf("[dry-run] image:     %s\n", image)
	fmt.Printf("[dry-run] container: %s\n", name)
	fmt.Printf("[dry-run] workspace: %s\n", absPath)
	fmt.Printf("[dry-run] command:   %s %s\n", rtBin, strings.Join(createArgs, " "))
}
