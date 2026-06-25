package cmd

import (
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	images "github.com/yldgio/aico/images"
	"github.com/yldgio/aico/internal/agents"
	rtpkg "github.com/yldgio/aico/internal/runtime"
)

func newUninstallCmd() *cobra.Command {
	var keepData bool
	c := &cobra.Command{
		Use:   "uninstall",
		Short: "Remove aico, its containers, image, and auth volumes",
		Long: "Remove the aico binary, all aico containers, the agent image,\n" +
			"and the per-agent auth volumes.\n\n" +
			"Use --keep-data to remove the binary and containers but keep the\n" +
			"auth volumes (so you stay logged in if you reinstall).",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return uninstall(keepData)
		},
	}
	c.Flags().BoolVar(&keepData, "keep-data", false, "keep auth volumes (stay logged in if you reinstall)")
	return c
}

func uninstall(keepData bool) error {
	rtBin := rtpkg.Resolve("")
	if rtBin == "" {
		fmt.Fprintln(os.Stderr, "aico: no container runtime found; skipping container cleanup")
	} else {
		cleanupContainers(rtBin)
		cleanupImage(rtBin)
		if !keepData {
			cleanupVolumes(rtBin)
		} else {
			fmt.Println("  keeping auth volumes (--keep-data)")
		}
	}

	removeBinary()
	return nil
}

func cleanupContainers(rtBin string) {
	fmt.Println("› removing aico containers...")
	for _, name := range agents.Names() {
		// List all containers matching the aico-<agent>- prefix.
		out, err := exec.Command(rtBin, "ps", "-a", "--filter", "name=aico-"+name+"-", "--format", "{{.Names}}").Output()
		if err != nil {
			continue
		}
		for _, cn := range strings.Split(strings.TrimSpace(string(out)), "\n") {
			cn = strings.TrimSpace(cn)
			if cn == "" {
				continue
			}
			_ = exec.Command(rtBin, "rm", "-f", cn).Run()
			fmt.Printf("  removed container %s\n", cn)
		}
	}
}

func cleanupImage(rtBin string) {
	fmt.Println("› removing agent image...")
	if err := exec.Command(rtBin, "rmi", "-f", images.DefaultTag).Run(); err != nil {
		fmt.Println("  image not found or already removed")
	} else {
		fmt.Printf("  removed %s\n", images.DefaultTag)
	}
}

func cleanupVolumes(rtBin string) {
	fmt.Println("› removing auth volumes...")
	out, err := exec.Command(rtBin, "volume", "ls", "--format", "{{.Name}}").Output()
	if err != nil {
		return
	}
	for _, vol := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		vol = strings.TrimSpace(vol)
		if !strings.HasPrefix(vol, "aico-auth-") {
			continue
		}
		_ = exec.Command(rtBin, "volume", "rm", "-f", vol).Run()
		fmt.Printf("  removed volume %s\n", vol)
	}
}

func removeBinary() {
	self, err := os.Executable()
	if err != nil {
		fmt.Println("› could not determine binary path; remove it manually")
		return
	}

	if runtime.GOOS == "windows" {
		// Windows locks the running binary — can't self-delete.
		fmt.Println("")
		fmt.Println("✓ cleanup complete. To finish, delete the binary:")
		fmt.Printf("  del \"%s\"\n", self)
		fmt.Println("")
		fmt.Println("  If .local\\bin is empty, you can also remove it from your PATH:")
		fmt.Println("  (Settings → System → About → Advanced → Environment Variables → Path)")
	} else {
		fmt.Printf("› removing %s...\n", self)
		if err := os.Remove(self); err != nil {
			fmt.Printf("  could not remove: %v\n", err)
			fmt.Printf("  try: sudo rm %s\n", self)
		} else {
			fmt.Println("")
			fmt.Printf("✓ aico uninstalled.\n")
		}
	}
}
