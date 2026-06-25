package cmd

import (
	"errors"
	"os/exec"
)

// exitCode extracts the process exit code from an error returned by
// exec.Command.Run(). Returns -1 if the error is not an ExitError.
func exitCode(err error) int {
	var ee *exec.ExitError
	if errors.As(err, &ee) {
		return ee.ExitCode()
	}
	return -1
}

// ExitError wraps an exit code so Execute() can pass it through to os.Exit().
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string { return "" }
