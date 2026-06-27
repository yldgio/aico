package cmd

import (
	"bufio"
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
	shareConfig  bool // deprecated, kept for backward compat (now does import)
	importConfig bool
	detach       bool
	name         string
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
			"can re-attach later or open a shell with `aico exec`. In an interactive\n" +
			"session, quitting the agent drops you into a bash shell inside the\n" +
			"container; relaunch the agent by name (e.g. `pi`), or exit the shell to\n" +
			"return to the host (the container keeps running). A container's mode is\n" +
			"fixed at creation: passing -d for a container that already exists in\n" +
			"interactive mode prompts to recreate it (use --new to skip the prompt).",
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
			// If the first arg isn't a known agent, treat it as a container name.
			if _, err := agents.Lookup(args[0]); err != nil {
				return runByName(args[0], extra, o)
			}
			return runAgent(args[0], path, extra, o)
		},
	}
	f := c.Flags()
	f.BoolVar(&o.newContainer, "new", false, "discard any existing container and create a fresh one")
	f.BoolVarP(&o.detach, "detach", "d", false, "keep the container running after the agent exits")
	f.StringVar(&o.name, "name", "", "assign a short name to the container (default: folder basename)")
	f.StringVar(&o.image, "image", "", "use a custom image instead of the built-in agent image")
	f.StringVar(&o.runtime, "runtime", "", "container runtime to use (default: auto-detect docker, then podman)")
	f.BoolVar(&o.verbose, "verbose", false, "print warnings, e.g. when an --import-config source dir is missing")
	f.BoolVar(&o.dryRun, "dry-run", false, "print what would run without creating a container")
	f.BoolVar(&o.importConfig, "import-config", false, "copy host config into the container on first run (one-time; does not overwrite on resume)")
	f.BoolVar(&o.shareConfig, "share-config", false, "deprecated: alias for --import-config")
	_ = f.MarkHidden("share-config")
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
	authPlan := auth.Build(agent, false) // shareConfig mounts removed; import-config copies instead

	if o.verbose {
		for _, w := range authPlan.Warnings {
			fmt.Fprintln(os.Stderr, "aico: warning: "+w)
		}
	}

	mountSrc, workdir := platform.WorkspaceMount(absPath)

	// Determine the container's short name (for aico ls / name-based access).
	shortName := resolveContainerName(o.name, agent.Name, absPath)

	// Build the agent command (agent binary + any trailing args).
	agentCmd := append([]string{}, agent.Command...)
	agentCmd = append(agentCmd, extraArgs...)

	// Determine Docker TTY flags based on whether stdin is a terminal.
	interactiveFlag := "-i"
	if isTTY() {
		interactiveFlag = "-it"
	}

	// Common volume/workdir/label args shared by both -d and non-d creation.
	commonArgs := []string{"--name", name,
		"-v", fmt.Sprintf("%s:%s", mountSrc, workdir), "-w", workdir}
	commonArgs = append(commonArgs, containerLabels(agent.Name, absPath, shortName)...)
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

	// Mode conflict: an explicit -d on a container that was created in
	// interactive mode. A container's mode is fixed at creation, so honoring
	// -d requires recreating it. Confirm before destroying anything.
	if !o.newContainer && o.detach && rt.Exists(name) && !isDetached(rt, name) {
		ok, err := confirmDetachRecreate(name)
		if err != nil {
			return err
		}
		if !ok {
			fmt.Fprintln(os.Stderr, "aico: cancelled; container left unchanged.")
			return nil
		}
		_ = rt.Remove(name)
	}

	// Resume path: an existing container is reused unless --new was given.
	if !o.newContainer && rt.Exists(name) {
		if rt.Running(name) {
			if isDetached(rt, name) {
				// -d container (CMD=sleep infinity): exec agent into it.
				return rt.Exec(name, isTTY(), agentExecCmd(agentCmd, isTTY())...)
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
			return rt.Exec(name, isTTY(), agentExecCmd(agentCmd, isTTY())...)
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

	wantImport := (o.importConfig || o.shareConfig) && len(agent.ConfigMounts) > 0

	if o.detach {
		// Create with sleep infinity as the main process, then exec agent.
		createArgs := append([]string{"run", "-d"}, commonArgs...)
		createArgs = append(createArgs, image, "sleep", "infinity")
		if _, err := rt.Output(createArgs...); err != nil {
			return fmt.Errorf("create detached container: %w", err)
		}
		if wantImport {
			importConfig(rt, name, agent)
		}
		return rt.Exec(name, isTTY(), agentExecCmd(agentCmd, isTTY())...)
	}

	if wantImport {
		// Split into create + cp + start so we can copy config before the
		// agent starts.
		createArgs := append([]string{"create", interactiveFlag}, commonArgs...)
		createArgs = append(createArgs, image)
		createArgs = append(createArgs, agentCmd...)
		if _, err := rt.Output(createArgs...); err != nil {
			return fmt.Errorf("create container: %w", err)
		}
		importConfig(rt, name, agent)
		return rt.Start(name)
	}

	// Non-d, no import: single interactive run (current behavior).
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

// agentExecCmd builds the command passed to `docker exec` when launching the
// agent in a detached (-d) container.
//
// In interactive mode it wraps the agent so that quitting the agent drops the
// user into an interactive bash shell *inside the container* (agent -> shell ->
// agent), instead of returning to the host. The wrapper runs the agent, prints
// a one-line hint, then exec's bash; args are passed as argv (via $@) so no
// shell quoting is required. In non-interactive mode it returns the agent
// command unchanged, preserving scripted/piped execution.
func agentExecCmd(agentCmd []string, tty bool) []string {
	if !tty {
		return agentCmd
	}
	const hint = "echo 'aico: agent exited - you are in the container shell. " +
		"relaunch the agent by name, or type exit (Ctrl-D) to leave (the container keeps running).'"
	script := `"$@"; ` + hint + `; exec bash`
	return append([]string{"bash", "-c", script, "aico"}, agentCmd...)
}

// confirmDetachRecreate asks the user whether to destroy and recreate an
// existing interactive container so it can run in detached (-d) mode. A
// container's mode is fixed at creation time, so honoring -d requires a fresh
// container. In non-interactive mode (no TTY) it returns an error instead of
// prompting, so scripts fail clearly rather than hang waiting for input.
func confirmDetachRecreate(name string) (bool, error) {
	if !isTTY() {
		return false, fmt.Errorf("container %s exists but was created without -d (interactive mode); aico cannot switch it to detached mode\n\nfix: recreate it with `aico run ... -d --new` (this destroys the current container)", name)
	}
	fmt.Fprintf(os.Stderr,
		"container %s was created in interactive mode.\n"+
			"-d (detached) on an existing container requires recreating it, which destroys the current container.\n"+
			"destroy and recreate it in detached mode? [y/N] ", name)
	line, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		return false, nil
	}
	return isAffirmative(line), nil
}

// isAffirmative reports whether a typed answer means "yes".
func isAffirmative(s string) bool {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "y", "yes":
		return true
	default:
		return false
	}
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
		execArgs := append([]string{"exec", execFlag, name}, agentExecCmd(agentCmd, isTTY())...)
		fmt.Fprintf(os.Stderr, "[dry-run] exec:      %s %s\n", rtBin, strings.Join(execArgs, " "))
	} else {
		createArgs := append([]string{"run", interactiveFlag}, commonArgs...)
		createArgs = append(createArgs, image)
		createArgs = append(createArgs, agentCmd...)
		fmt.Fprintf(os.Stderr, "[dry-run] command:   %s %s\n", rtBin, strings.Join(createArgs, " "))
	}
}

// importConfig copies host config directories into the container (one-time).
// Only copies sources that exist on the host; skips silently otherwise.
func importConfig(rt *runtime.Runtime, name string, agent agents.Agent) {
	for _, src := range agent.ConfigMounts {
		host := auth.ConfigHostPath(src)
		if _, err := os.Stat(host); err != nil {
			continue
		}
		// docker cp requires a trailing /. to copy contents into target dir.
		if err := rt.CopyTo(name, host+"/.", src.Target); err != nil {
			fmt.Fprintf(os.Stderr, "aico: warning: could not import config from %s: %v\n", host, err)
		}
	}
}
