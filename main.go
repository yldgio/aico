// Command aico launches or resumes an isolated container for an AI coding agent.
package main

import "github.com/yldgio/aico/cmd"

// Build metadata, overridden at release time via -ldflags
// (see .goreleaser.yml). Defaults apply to plain `go build`.
var (
	version = "dev"
	commit  = ""
	date    = ""
)

func main() {
	cmd.Execute(cmd.BuildInfo{Version: version, Commit: commit, Date: date})
}
