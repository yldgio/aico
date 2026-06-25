package cmd

import (
	"fmt"
	"runtime"
	"runtime/debug"

	"github.com/spf13/cobra"
)

// BuildInfo carries version metadata injected at build time (via -ldflags) and
// is the single source of truth for what `--version` / `version` report.
type BuildInfo struct {
	Version string // semver tag, e.g. "v0.1.1" ("dev" for unversioned builds)
	Commit  string // git commit hash
	Date    string // build date
}

// resolve fills in missing fields from the Go module build info, so a binary
// installed via `go install ...@v0.1.1` still reports its version even though
// it was not built with ldflags.
func (b BuildInfo) resolve() BuildInfo {
	if b.Version == "" || b.Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			b.Version = info.Main.Version
		}
	}
	if b.Version == "" {
		b.Version = "dev"
	}
	if b.Commit == "" {
		if info, ok := debug.ReadBuildInfo(); ok {
			for _, s := range info.Settings {
				if s.Key == "vcs.revision" {
					b.Commit = s.Value
				}
			}
		}
	}
	return b
}

// short returns the one-line version string used by `aico --version`.
func (b BuildInfo) short() string {
	r := b.resolve()
	return fmt.Sprintf("aico %s", r.Version)
}

// long returns the detailed version block used by `aico version`.
func (b BuildInfo) long() string {
	r := b.resolve()
	out := fmt.Sprintf("aico %s\n", r.Version)
	if r.Commit != "" {
		out += fmt.Sprintf("commit:  %s\n", r.Commit)
	}
	if r.Date != "" {
		out += fmt.Sprintf("built:   %s\n", r.Date)
	}
	out += fmt.Sprintf("go:      %s\nos/arch: %s/%s\n",
		runtime.Version(), runtime.GOOS, runtime.GOARCH)
	return out
}

func newVersionCmd(bi BuildInfo) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print version information",
		Args:  cobra.NoArgs,
		Run: func(cmd *cobra.Command, _ []string) {
			fmt.Fprint(cmd.OutOrStdout(), bi.long())
		},
	}
}
