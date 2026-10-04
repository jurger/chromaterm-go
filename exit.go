package chromaterm

import (
	"errors"
	"os/exec"
)

// ExitCode turns the error from running a program into its exit code: 0 for
// success, the program's code when it exited, 1 when it was killed by a
// signal, and 127 when it could not run at all.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		if code := exitErr.ExitCode(); code >= 0 {
			return code
		}
		return 1
	}
	return 127
}
