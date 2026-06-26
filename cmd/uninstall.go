package cmd

import (
	"fmt"
	"os"
	"runtime"

	"github.com/spf13/cobra"
)

func newUninstallCmd() *cobra.Command {
	var keepData bool
	c := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove aico completely (purge + delete binary)",
		Long: "Remove the aico binary, all aico containers, the agent image,\n" +
			"and the per-agent auth volumes.\n\n" +
			"Use --keep-data to keep auth volumes (stay logged in if you reinstall).\n" +
			"Use `aico purge` if you only want to reset Docker artifacts without\n" +
			"removing the binary.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return uninstall(keepData)
		},
	}
	c.Flags().BoolVar(&keepData, "keep-data", false, "keep auth volumes (stay logged in if you reinstall)")
	return c
}

func uninstall(keepData bool) error {
	if !keepData {
		_ = purge("")
	} else {
		// Purge containers + image but keep volumes.
		fmt.Fprintln(os.Stderr, "  keeping auth volumes (--keep-data)")
		// Just remove containers + image, skip volumes.
		_ = purgeContainersAndImage("")
	}

	removeBinary()
	return nil
}

// purgeContainersAndImage removes containers and image but not volumes.
func purgeContainersAndImage(rtOverride string) error {
	rt, err := detectRuntime(rtOverride)
	if err != nil {
		return nil
	}

	fmt.Fprintln(os.Stderr, "› removing all aico containers...")
	removeAllContainers(rt)

	fmt.Fprintln(os.Stderr, "› removing agent image...")
	if _, err := rt.Output("rmi", "-f", "aico-agents:latest"); err != nil {
		fmt.Fprintln(os.Stderr, "  image not found or already removed")
	} else {
		fmt.Fprintln(os.Stderr, "  removed aico-agents:latest")
	}
	return nil
}

func removeBinary() {
	self, err := os.Executable()
	if err != nil {
		fmt.Fprintln(os.Stderr, "› could not determine binary path; remove it manually")
		return
	}

	if runtime.GOOS == "windows" {
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "✓ cleanup complete. To finish, delete the binary:")
		fmt.Fprintf(os.Stderr, "  del \"%s\"\n", self)
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "  If .local\\bin is empty, you can also remove it from your PATH:")
		fmt.Fprintln(os.Stderr, "  (Settings → System → About → Advanced → Environment Variables → Path)")
	} else {
		fmt.Fprintf(os.Stderr, "› removing %s...\n", self)
		if err := os.Remove(self); err != nil {
			fmt.Fprintf(os.Stderr, "  could not remove: %v\n", err)
			fmt.Fprintf(os.Stderr, "  try: sudo rm %s\n", self)
		} else {
			fmt.Fprintln(os.Stderr, "")
			fmt.Fprintln(os.Stderr, "✓ aico uninstalled.")
		}
	}
}
