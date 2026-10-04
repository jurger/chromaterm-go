package chromaterm

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"testing"
)

func TestRunInPTYHighlightsProgramOutput(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("needs a Unix PTY")
	}
	null, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer null.Close()
	var out bytes.Buffer
	code, err := RunInPTY([]string{"sh", "-c", "echo connect error to 10.0.0.1; exit 3"}, defaultConfig(t, true), null, &out)
	if err != nil {
		t.Fatal(err)
	}
	if code != 3 {
		t.Fatalf("exit code = %d, want 3", code)
	}
	got := out.String()
	for _, want := range []string{"\x1b[38;2;199;24;0merror", "\x1b[38;2;0;224;209m10.0.0.1", "\x1b[38;2;40;197;1mconnect"} {
		if !strings.Contains(got, want) {
			t.Fatalf("output %q missing %q", got, want)
		}
	}
}

func TestExitCode(t *testing.T) {
	if ExitCode(nil) != 0 {
		t.Fatal("success should be 0")
	}
	if ExitCode(errors.New("could not start")) != 127 {
		t.Fatal("a start failure should be 127")
	}
	if runtime.GOOS != "windows" {
		if code := ExitCode(exec.Command("sh", "-c", "exit 4").Run()); code != 4 {
			t.Fatalf("exit 4 gave %d", code)
		}
	}
}
