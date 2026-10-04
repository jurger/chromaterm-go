//go:build !windows

package chromaterm

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"syscall"

	"github.com/creack/pty"
	"golang.org/x/term"
)

// PTYSupported reports whether RunInPTY works on this platform.
const PTYSupported = true

// RunInPTY runs program in a new PTY and returns its exit code. Input from
// in is copied raw (in is switched to raw mode if it is a terminal), output
// is highlighted with cfg on its way to out, and in's window size follows
// SIGWINCH. An error means the PTY could not be started; run the program
// without highlighting then.
func RunInPTY(program []string, cfg *Config, in *os.File, out io.Writer) (int, error) {
	if len(program) == 0 {
		return 0, errors.New("no program to run")
	}
	cmd := exec.Command(program[0], program[1:]...)
	ptmx, err := pty.Start(cmd)
	if err != nil {
		return 0, err
	}
	defer ptmx.Close()

	_ = pty.InheritSize(in, ptmx)
	winch := make(chan os.Signal, 1)
	signal.Notify(winch, syscall.SIGWINCH)
	defer signal.Stop(winch)
	go func() {
		for range winch {
			_ = pty.InheritSize(in, ptmx)
		}
	}()
	// Ctrl-C reaches the program through the PTY; don't die from a stray SIGINT.
	signal.Ignore(os.Interrupt)
	defer signal.Reset(os.Interrupt)

	if state, err := term.MakeRaw(int(in.Fd())); err == nil {
		defer func() { _ = term.Restore(int(in.Fd()), state) }()
	}
	go func() { _, _ = io.Copy(ptmx, in) }()

	// Reading the PTY fails once the program exits and its side closes.
	_ = (&Stream{Config: cfg}).Run(ptmx, out)
	return ExitCode(cmd.Wait()), nil
}
