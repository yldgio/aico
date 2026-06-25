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
	"github.com/yldgio/aico/internal/platform"
	"github.com/yldgio/aico/internal/runtime"
)

type runOpts struct {
	newContainer bool
	image        string
	runtime      string
	verbose      bool
	dryRun       bool
	shareConfig  bool
	detach       bool
}

func newRunCmd() *cobra.Command {
	o := &runOpts{}
	c := &cobra.Command{
		Use:   "run <agent> [path] [-- agent-args...]",
		Short: "Run or resume an agent container for a project folder",
		Long: "Run or resume an agent container for a project folder.\n\n" +
			"<agent> is one of: pi, opencode, copilot-cli, codex, claude\n" +
			"[path]  is the project folder to mount (defaults to the current directory).\n" +
			"[-- args] are forwarded to the agent command.\n\n" +
			"On first use a container is created; subsequent runs on the same path\n" +
			"resume it. Use --new to discard and recreate it.\n\n" +
			"With -d the container stays running after the agent exits, so you\n" +
			"can re-attach later or open a shell with `aico exec`.",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ""
			if len(args) >= 2 {
				path = args[1]
			}
			agentArgs := cmd.ArgsLenAtDash()
			var extra []string
			if agentArgs >= 0 {
				extra = args[agentArgs:]
			}
			return runAgent(args[0], path, extra, o)
		},
	}
	f := c.Flags()
	f.BoolVar(&o.newContainer, "new", false, "discard any existing container and create a fresh one")
	f.BoolVarP(&o.detach, "detach", "d", false, "keep the container running after the agent exits")
	f.StringVar(&o.image, "image", "", "use a custom image instead of the built-in agent image")
	f.StringVar(&o.runtime, "runtime", "", "container runtime to use (default: auto-detect docker, then podman)")
	f.BoolVar(&o.verbose, "verbose", false, "print warnings, e.g. when a shared config dir is missing")
	f.BoolVar(&o.dryRun, "dry-run", false, "print what would run without creating a container")
	f.BoolVar(&o.shareConfig, "share-config", false, "also mount the host config dir read-only (off by default; login itself persists in a volume)")
	return c
}

func runAgent(agentName, path string, extraArgs []string, o *runOpts) error {
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
	authPlan := auth.Build(agent, o.shareConfig)

	if o.verbose {
		for _, w := range authPlan.Warnings {
			fmt.Fprintln(os.Stderr, "aico: warning: "+w)
		}
	}

	mountSrc, workdir := platform.WorkspaceMount(absPath)

	// Build the agent command (agent binary + any trailing args).
	agentCmd := append([]string{}, agent.Command...)
	agentCmd = append(agentCmd, extraArgs...)

	// Determine Docker TTY flags based on whether stdin is a terminal.
	interactiveFlag := "-i"
	if isTTY() {
		interactiveFlag = "-it"
	}

	// Common volume/workdir args shared by both -d and non-d creation.
	commonArgs := []string{"--name", name,
		"-v", fmt.Sprintf("%s:%s", mountSrc, workdir), "-w", workdir}
	commonArgs = append(commonArgs, authPlan.Args...)

	if o.dryRun {
		printDryRunDetach(rtBin, image, name, workdir, commonArgs, agentCmd, o.detach, interactiveFlag)
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
			if isDetached(rt, name) {
				// -d container (CMD=sleep infinity): exec agent into it.
				return rt.Exec(name, isTTY(), agentCmd...)
			}
			// Non-d container: re-attach to the running process.
			return rt.Attach(name)
		}
		// Container is stopped.
		if isDetached(rt, name) {
			// Was started with -d: restart in background, then exec agent.
			if err := rt.StartBackground(name); err != nil {
				return fmt.Errorf("start container: %w", err)
			}
			return rt.Exec(name, isTTY(), agentCmd...)
		}
		// Non-d container: interactive start (original behavior).
		return rt.Start(name)
	}

	// Fresh container: ensure the image exists (unless the user supplied one).
	if o.image == "" {
		if err := images.EnsureBuilt(rt); err != nil {
			return err
		}
	}

	if o.detach {
		// Create with sleep infinity as the main process, then exec agent.
		createArgs := append([]string{"run", "-d"}, commonArgs...)
		createArgs = append(createArgs, image, "sleep", "infinity")
		if _, err := rt.Output(createArgs...); err != nil {
			return fmt.Errorf("create detached container: %w", err)
		}
		return rt.Exec(name, isTTY(), agentCmd...)
	}

	// Non-d: single interactive run (current behavior).
	createArgs := append([]string{"run", interactiveFlag}, commonArgs...)
	createArgs = append(createArgs, image)
	createArgs = append(createArgs, agentCmd...)
	return rt.Run(createArgs...)
}

// isDetached reports whether a container was created in detach mode.
// It checks if the container's command is exactly "sleep infinity", which
// identifies containers created with -d.
func isDetached(rt *runtime.Runtime, name string) bool {
	return rt.ContainerCommand(name) == "sleep infinity"
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

func printDryRunDetach(rtBin, image, name, workdir string, commonArgs, agentCmd []string, detach bool, interactiveFlag string) {
	if rtBin == "" {
		rtBin = "(none detected — install docker or podman)"
	}
	fmt.Fprintf(os.Stderr, "[dry-run] runtime:   %s\n", rtBin)
	fmt.Fprintf(os.Stderr, "[dry-run] image:     %s\n", image)
	fmt.Fprintf(os.Stderr, "[dry-run] container: %s\n", name)
	fmt.Fprintf(os.Stderr, "[dry-run] workspace: %s\n", workdir)
	if detach {
		createArgs := append([]string{"run", "-d"}, commonArgs...)
		createArgs = append(createArgs, image, "sleep", "infinity")
		fmt.Fprintf(os.Stderr, "[dry-run] create:    %s %s\n", rtBin, strings.Join(createArgs, " "))
		execFlag := "-i"
		if isTTY() {
			execFlag = "-it"
		}
		execArgs := append([]string{"exec", execFlag, name}, agentCmd...)
		fmt.Fprintf(os.Stderr, "[dry-run] exec:      %s %s\n", rtBin, strings.Join(execArgs, " "))
	} else {
		createArgs := append([]string{"run", interactiveFlag}, commonArgs...)
		createArgs = append(createArgs, image)
		createArgs = append(createArgs, agentCmd...)
		fmt.Fprintf(os.Stderr, "[dry-run] command:   %s %s\n", rtBin, strings.Join(createArgs, " "))
	}
}
