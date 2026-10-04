package chromaterm

import (
	"bytes"
	"io"
	"strings"
	"sync"
	"testing"
	"time"
)

// feed writes parts into a pipe with pauses, then closes it, and returns
// what the stream produced.
func feed(t *testing.T, c *Config, pause time.Duration, parts ...string) string {
	t.Helper()
	r, w := io.Pipe()
	var out bytes.Buffer
	done := make(chan error)
	s := &Stream{Config: c}
	go func() { done <- s.Run(r, &out) }()
	for _, part := range parts {
		if _, err := w.Write([]byte(part)); err != nil {
			t.Fatal(err)
		}
		time.Sleep(pause)
	}
	w.Close()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	return out.String()
}

func keywordConfig(t *testing.T) *Config {
	t.Helper()
	c := NewConfig()
	red, _ := ParseColor("f#ff0000", nil, true)
	rule, err := NewRule(`error|12345`, map[any]Color{0: red}, "", false)
	if err != nil {
		t.Fatal(err)
	}
	c.Rules = []*Rule{rule}
	return c
}

const red = "\x1b[38;2;255;0;0m"

func TestStreamHighlightsLines(t *testing.T) {
	got := feed(t, keywordConfig(t), 0, "an error here\r\nfine\r\n")
	if got != "an "+red+"error\x1b[39m here\r\nfine\r\n" {
		t.Fatalf("got %q", got)
	}
}

func TestStreamLeavesTypedCharacterAlone(t *testing.T) {
	c := NewConfig()
	digit, _ := ParseColor("f#ff0000", nil, true)
	rule, _ := NewRule(`\d`, map[any]Color{0: digit}, "", false)
	c.Rules = []*Rule{rule}
	if got := feed(t, c, 0, "5"); got != "5" {
		t.Fatalf("an echoed keystroke should pass through: %q", got)
	}
}

func TestStreamJoinsMatchSplitAcrossReads(t *testing.T) {
	// The second half arrives well within the wait, so the match is whole.
	got := feed(t, keywordConfig(t), time.Millisecond, "id 123", "45 ok\r\n")
	if got != "id "+red+"12345\x1b[39m ok\r\n" {
		t.Fatalf("got %q", got)
	}
}

func TestStreamFlushesTrailingTextAfterWait(t *testing.T) {
	r, w := io.Pipe()
	var out safeBuffer
	s := &Stream{Config: keywordConfig(t)}
	go s.Run(r, &out)
	w.Write([]byte("prompt error$ "))
	deadline := time.Now().Add(2 * time.Second)
	for !strings.Contains(out.String(), "$ ") && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got := out.String(); got != "prompt "+red+"error\x1b[39m$ " {
		t.Fatalf("a prompt without a new line should still be shown: %q", got)
	}
	w.Close()
}

func TestStreamSkipsAlternateScreen(t *testing.T) {
	got := feed(t, keywordConfig(t), 0, "error\r\n\x1b[?1049herror in vim\x1b[?1049lerror\r\n")
	want := red + "error\x1b[39m\r\n\x1b[?1049herror in vim\x1b[?1049l" + red + "error\x1b[39m\r\n"
	if got != want {
		t.Fatalf("got  %q\nwant %q", got, want)
	}
}

func TestStreamWaitsForControlStringEnd(t *testing.T) {
	got := feed(t, keywordConfig(t), time.Millisecond, "\x1b]0;error ti", "tle\x07error\r\n")
	if got != "\x1b]0;error title\x07"+red+"error\x1b[39m\r\n" {
		t.Fatalf("got %q", got)
	}
}

type safeBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *safeBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *safeBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}
