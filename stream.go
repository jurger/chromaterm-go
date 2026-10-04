package chromaterm

import (
	"bytes"
	"io"
	"regexp"
	"time"
)

// readSize is the most data handled per read, as in ChromaTerm.
const readSize = 8192

// splitRE finds where processing may split the output (ECMA-035 and -048):
// new lines and vertical spaces, C1 and independent control functions,
// character set selections, CSI sequences other than SGR, and control
// strings (DCS, SOS, OSC, PM, APC). Text between them is highlighted; the
// separators pass through unchanged.
var splitRE = regexp.MustCompile(`\r\n?|[\n\v\f]|` +
	`\x1b[\x30-\x4f\x51-\x57\x59-\x5a\x5c\x60-\x7e]|` +
	`\x1b[\x23\x28-\x2b\x2d-\x2f][\x20-\x7e]|` +
	`\x1b\x5b[\x30-\x3f]*[\x20-\x2f]*[\x40-\x6c\x6e-\x7e]|` +
	`\x1b[\x50\x58\x5d\x5e\x5f][^\x07\x1b]*(?:\x07|\x1b\x5c)?`)

// altScreenRE matches the private modes full-screen programs (vim, less,
// htop) switch on and off; their screens are drawn, not printed, so they are
// left unhighlighted.
var altScreenRE = regexp.MustCompile(`^\x1b\[\?(?:1049|1047|47)([hl])$`)

type chunk struct {
	data, sep []byte
}

// splitBuffer splits buf into text and the separator after it. The last
// chunk's separator is empty when buf ends with text.
func splitBuffer(buf []byte) []chunk {
	var chunks []chunk
	last := 0
	for _, loc := range splitRE.FindAllIndex(buf, -1) {
		chunks = append(chunks, chunk{data: buf[last:loc[0]], sep: buf[loc[0]:loc[1]]})
		last = loc[1]
	}
	if last < len(buf) || len(chunks) == 0 {
		chunks = append(chunks, chunk{data: buf[last:]})
	}
	return chunks
}

func isControlStringStart(sep []byte) bool {
	return len(sep) >= 2 && sep[0] == 0x1b && bytes.IndexByte([]byte{0x50, 0x58, 0x5d, 0x5e, 0x5f}, sep[1]) >= 0
}

func isControlStringEnd(sep []byte) bool {
	return bytes.HasSuffix(sep, []byte{0x07}) || bytes.HasSuffix(sep, []byte{0x1b, 0x5c})
}

// Wait bounds for more data before the last, possibly cut-off chunk is
// highlighted on its own: 1/64 s while forwarding keystrokes, relaxed to
// 1/8 s once output spans lines.
const (
	minWait = time.Second / 64
	maxWait = time.Second / 8
)

// Stream highlights everything read from in and writes it to out, until in
// reaches EOF. It is a port of ChromaTerm's process_input for a program run
// in a PTY: complete chunks are written at once, and the trailing chunk waits
// briefly for the rest of its line so a match is not split across reads.
type Stream struct {
	Config *Config
	alt    bool
}

func (s *Stream) write(c chunk, highlight bool) []byte {
	var b []byte
	if highlight && !s.alt && len(c.data) > 0 {
		b = append(b, s.Config.Highlight(string(c.data))...)
	} else {
		b = append(b, c.data...)
	}
	b = append(b, c.sep...)
	if m := altScreenRE.FindSubmatch(c.sep); m != nil {
		s.alt = m[1][0] == 'h'
	}
	return b
}

// Run copies in to out with highlighting.
func (s *Stream) Run(in io.Reader, out io.Writer) error {
	reads := make(chan []byte)
	go func() {
		defer close(reads)
		for {
			buf := make([]byte, readSize)
			n, err := in.Read(buf)
			if n > 0 {
				reads <- buf[:n]
			}
			if err != nil {
				return
			}
		}
	}()

	var buffer, pending []byte
	havePending, open := false, true
	for {
		var dataRead []byte
		if havePending {
			dataRead, havePending = pending, false
		} else if open {
			dataRead, open = <-reads
		}
		buffer = append(buffer, dataRead...)
		if len(buffer) == 0 {
			if !open {
				return nil
			}
			continue
		}
		chunks := splitBuffer(buffer)
		var w []byte
		for _, c := range chunks[:len(chunks)-1] {
			w = append(w, s.write(c, true)...)
		}
		wait := minWait
		if bytes.IndexByte(buffer, '\n') >= 0 {
			wait = maxWait
		}
		last := chunks[len(chunks)-1]
		rest := append(append([]byte{}, last.data...), last.sep...)
		switch {
		// An incomplete control string; wait for the rest of it.
		case len(dataRead) > 0 && isControlStringStart(last.sep) && !isControlStringEnd(last.sep):
			buffer = rest
		// A single character is typing echoed back; don't highlight it.
		// Shells like zsh add backspaces around it.
		case len(buffer) < 2+bytes.Count(dataRead, []byte{'\b'})*2:
			w = append(w, s.write(last, false)...)
			buffer = nil
		// More data is on its way; fetch it before highlighting.
		case len(dataRead) > 0 && len(buffer) <= readSize && open:
			timer := time.NewTimer(wait)
			select {
			case next, ok := <-reads:
				timer.Stop()
				// Either way the last chunk waits for the next pass; at EOF that
				// pass handles it with no new data, as ChromaTerm does.
				pending, havePending = next, ok
				open = ok
				buffer = rest
			case <-timer.C:
				w = append(w, s.write(last, true)...)
				buffer = nil
			}
		default:
			w = append(w, s.write(last, true)...)
			buffer = nil
		}
		if len(w) > 0 {
			if _, err := out.Write(w); err != nil {
				return err
			}
		}
	}
}
