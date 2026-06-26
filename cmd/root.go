// Package cmd wires up the aico command-line interface.
package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newRootCmd(bi BuildInfo) *cobra.Command {
	root := &cobra.Command{
		Use:           "aico",
		Short:         "Launch or resume an isolated container for an AI coding agent",
		Long:          "aico runs an AI coding agent inside a container, mounting the current\nfolder and keeping your login across sessions. Re-running on the\nsame path resumes the same container.",
		Version:       bi.short(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	// `aico --version` prints exactly the short string (no "aico version " prefix).
	root.SetVersionTemplate("{{.Version}}\n")
	root.AddCommand(newRunCmd())
	root.AddCommand(newExecCmd())
	root.AddCommand(newLsCmd())
	root.AddCommand(newRmCmd())
	root.AddCommand(newPurgeCmd())
	root.AddCommand(newUninstallCmd())
	root.AddCommand(newUpgradeCmd(bi))
	root.AddCommand(newVersionCmd(bi))
	return root
}

// Execute runs the aico CLI, exiting non-zero on error.
// Agent/container exit codes pass through; aico infrastructure errors exit 125.
func Execute(bi BuildInfo) {
	if err := newRootCmd(bi).Execute(); err != nil {
		var ee *ExitError
		if errors.As(err, &ee) {
			os.Exit(ee.Code)
		}
		// Container process exited non-zero: pass through the exit code.
		if code := exitCode(err); code > 0 {
			os.Exit(code)
		}
		// aico infrastructure error.
		fmt.Fprintln(os.Stderr, "aico: "+err.Error())
		os.Exit(125)
	}
}
