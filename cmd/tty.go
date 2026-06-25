package cmd

import "os"

// isTTY reports whether stdin is connected to an interactive terminal.
// When false, aico runs non-interactively (no -t flag to Docker, output
// streams directly to stdout/stderr for piping and capture).
func isTTY() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice != 0
}
