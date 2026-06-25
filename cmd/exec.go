package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/yldgio/aico/internal/agents"
	"github.com/yldgio/aico/internal/container"
	"github.com/yldgio/aico/internal/runtime"
)

type execOpts struct {
	runtime string
	dryRun  bool
}

func newExecCmd() *cobra.Command {
	o := &execOpts{}
	c := &cobra.Command{
		Use:   "exec <agent> [path]",
		Short: "Open a shell in a running agent container",
		Long: "Open an interactive bash shell in a running agent container.\n\n" +
			"The container must already be running (e.g. started with `aico run -d`).\n" +
			"Use this to explore the filesystem, debug, or run manual commands\n" +
			"alongside the agent.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := ""
			if len(args) == 2 {
				path = args[1]
			}
			return execShell(args[0], path, o)
		},
	}
	f := c.Flags()
	f.StringVar(&o.runtime, "runtime", "", "container runtime to use (default: auto-detect)")
	f.BoolVar(&o.dryRun, "dry-run", false, "print what would run without executing")
	return c
}

func execShell(agentName, path string, o *execOpts) error {
	agent, err := agents.Lookup(agentName)
	if err != nil {
		return err
	}

	absPath, err := resolvePath(path)
	if err != nil {
		return err
	}

	name := container.Name(agent.Name, absPath)
	rtBin := runtime.Resolve(o.runtime)

	if o.dryRun {
		if rtBin == "" {
			rtBin = "(none detected)"
		}
		fmt.Printf("[dry-run] container: %s\n", name)
		fmt.Printf("[dry-run] command:   %s exec -it %s bash\n", rtBin, name)
		return nil
	}

	rt, err := runtime.Detect(o.runtime)
	if err != nil {
		return err
	}

	if !rt.Running(name) {
		return fmt.Errorf("container %s is not running\n\nfix: start it first with `aico run %s -d`, then use `aico exec`", name, agentName)
	}

	return rt.Exec(name, "bash")
}
