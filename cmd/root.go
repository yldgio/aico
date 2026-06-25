// Package cmd wires up the aico command-line interface.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newRootCmd(bi BuildInfo) *cobra.Command {
	root := &cobra.Command{
		Use:           "aico",
		Short:         "Launch or resume an isolated container for an AI coding agent",
		Long:          "aico runs an AI coding agent inside a container, mounting the current\nfolder and forwarding your existing host credentials. Re-running on the\nsame path resumes the same container.",
		Version:       bi.short(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	// `aico --version` prints exactly the short string (no "aico version " prefix).
	root.SetVersionTemplate("{{.Version}}\n")
	root.AddCommand(newRunCmd())
	root.AddCommand(newExecCmd())
	root.AddCommand(newUninstallCmd())
	root.AddCommand(newUpgradeCmd(bi))
	root.AddCommand(newVersionCmd(bi))
	return root
}

// Execute runs the aico CLI, exiting non-zero on error.
func Execute(bi BuildInfo) {
	if err := newRootCmd(bi).Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "aico: "+err.Error())
		os.Exit(1)
	}
}
