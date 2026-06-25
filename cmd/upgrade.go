package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
)

const githubRepo = "yldgio/aico"

func newUpgradeCmd(bi BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade",
		Short: "Upgrade aico to the latest release",
		Long: "Download and replace the aico binary with the latest release from GitHub.\n" +
			"The agent image is rebuilt automatically on the next run if outdated.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return upgrade(bi)
		},
	}
}

func upgrade(bi BuildInfo) error {
	fmt.Println("› checking latest release...")

	tag, err := latestTag()
	if err != nil {
		return fmt.Errorf("fetch latest release: %w", err)
	}

	current := bi.Version
	latest := strings.TrimPrefix(tag, "v")

	if current == latest {
		fmt.Printf("✓ already up to date (%s)\n", tag)
		return nil
	}
	fmt.Printf("  current: %s → latest: %s\n", current, tag)

	self, err := os.Executable()
	if err != nil {
		return fmt.Errorf("cannot determine binary path: %w", err)
	}
	self, err = filepath.EvalSymlinks(self)
	if err != nil {
		return fmt.Errorf("resolve binary path: %w", err)
	}

	goos := runtime.GOOS
	arch := runtime.GOARCH
	ext := "tar.gz"
	if goos == "windows" {
		ext = "zip"
	}

	version := strings.TrimPrefix(tag, "v")
	archive := fmt.Sprintf("aico_%s_%s_%s.%s", version, goos, arch, ext)
	url := fmt.Sprintf("https://github.com/%s/releases/download/%s/%s", githubRepo, tag, archive)

	fmt.Printf("› downloading %s...\n", archive)

	tmpDir, err := os.MkdirTemp("", "aico-upgrade-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	archivePath := filepath.Join(tmpDir, archive)
	if err := downloadFile(url, archivePath); err != nil {
		return fmt.Errorf("download: %w", err)
	}

	fmt.Println("› extracting...")
	binName := "aico"
	if goos == "windows" {
		binName = "aico.exe"
	}

	newBin := filepath.Join(tmpDir, binName)
	if ext == "tar.gz" {
		if err := extractTarGz(archivePath, tmpDir); err != nil {
			return fmt.Errorf("extract: %w", err)
		}
	} else {
		if err := extractZip(archivePath, tmpDir); err != nil {
			return fmt.Errorf("extract: %w", err)
		}
	}

	if _, err := os.Stat(newBin); err != nil {
		return fmt.Errorf("binary not found in archive: %s", binName)
	}

	// Replace the current binary. On Windows the running binary is locked,
	// so we rename-then-move instead of overwriting in place.
	fmt.Printf("› replacing %s...\n", self)
	if err := replaceBinary(self, newBin); err != nil {
		return fmt.Errorf("replace binary: %w\n\nfix: try running with elevated permissions, or download manually from\n  https://github.com/%s/releases/latest", err, githubRepo)
	}

	fmt.Printf("\n✓ upgraded to %s\n", tag)
	fmt.Println("  the agent image will rebuild automatically on next run if needed.")
	return nil
}

func latestTag() (string, error) {
	url := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)
	resp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("GitHub API: %s", resp.Status)
	}
	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	if release.TagName == "" {
		return "", fmt.Errorf("no tag_name in response")
	}
	return release.TagName, nil
}

func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, url)
	}
	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, resp.Body)
	return err
}

func replaceBinary(oldPath, newPath string) error {
	newData, err := os.ReadFile(newPath)
	if err != nil {
		return err
	}

	// The running binary is locked on both Windows (file lock) and Linux
	// ("text file busy"). Rename the old binary out of the way first, write
	// the new one at the original path, then clean up.
	backup := oldPath + ".old"
	_ = os.Remove(backup)
	if err := os.Rename(oldPath, backup); err != nil {
		return fmt.Errorf("rename current binary: %w", err)
	}
	if err := os.WriteFile(oldPath, newData, 0o755); err != nil {
		// Restore on failure.
		_ = os.Rename(backup, oldPath)
		return err
	}
	_ = os.Remove(backup)
	return nil
}
