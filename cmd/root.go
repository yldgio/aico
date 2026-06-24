// Package cmd wires up the aico command-line interface.
package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "aico",
		Short:         "Launch or resume an isolated container for an AI coding agent",
		Long:          "aico runs an AI coding agent inside a container, mounting the current\nfolder and forwarding your existing host credentials. Re-running on the\nsame path resumes the same container.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.AddCommand(newRunCmd())
	return root
}

// Execute runs the aico CLI, exiting non-zero on error.
func Execute() {
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, "aico: "+err.Error())
		os.Exit(1)
	}
}
