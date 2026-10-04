//go:build windows

package chromaterm

import (
	"errors"
	"io"
	"os"
)

// PTYSupported reports whether RunInPTY works on this platform.
const PTYSupported = false

// RunInPTY needs a Unix PTY; on Windows it always returns an error.
func RunInPTY([]string, *Config, *os.File, io.Writer) (int, error) {
	return 0, errors.New("chromaterm: RunInPTY needs a Unix PTY")
}
