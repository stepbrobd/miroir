// Package display provides the default terminal and log-based reporter for miroir.
package display

import (
	"fmt"
	"os"
	"strings"
	"sync"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/log"
	"github.com/charmbracelet/x/ansi"
	"golang.org/x/term"
)

type lineKind int8

const (
	lineEmpty lineKind = iota
	lineRepo
	lineRemote
	lineOutput
	lineError
	lineErrorRemote
	lineErrorOutput
)

type line struct {
	text string
	kind lineKind
}

const outputPlaceholder = "[no output]"

// Display renders a live-updating progress grid in TTY mode
// using direct ANSI escape codes, or structured log in non-TTY mode.
type Display struct {
	tty    bool
	mu     sync.Mutex
	lines  []line
	width  int
	stride int
	theme  Theme
	drawn  int
	ready  bool
	log    *log.Logger
}

// direct ANSI when stdout is a TTY, charm log otherwise.
// ttyOverride forces the mode when non-nil.
func New(repos, remotes int, th Theme, ttyOverride *bool) *Display {
	tty := term.IsTerminal(int(os.Stdout.Fd()))
	if ttyOverride != nil {
		tty = *ttyOverride
	}
	d := &Display{tty: tty, theme: th}

	if tty {
		w, _, _ := term.GetSize(int(os.Stdout.Fd()))
		if w <= 0 {
			w = 80
		}
		d.width = w
		d.stride = 1 + 2*remotes
		total := repos * d.stride
		d.lines = make([]line, max(1, total))
		for slot := range repos {
			d.resetSlot(slot)
		}
	} else {
		d.log = log.NewWithOptions(os.Stdout, log.Options{
			ReportTimestamp: false,
			ReportCaller:    false,
		})
	}
	return d
}

func (d *Display) resetSlot(slot int) {
	if !d.tty {
		return
	}
	base := slot * d.stride
	if base >= len(d.lines) {
		return
	}
	d.lines[base] = line{}
	for i := 1; base+i < len(d.lines) && i < d.stride; i++ {
		if i%2 == 1 {
			d.lines[base+i] = line{}
			continue
		}
		d.lines[base+i] = line{text: outputPlaceholder, kind: lineOutput}
	}
}

// padding is defined here, not in Theme, so error variants stay aligned
func (d *Display) styled(k lineKind) lipgloss.Style {
	w := d.width
	switch k {
	case lineRepo:
		return d.theme.Repo.Width(w).MaxWidth(w)
	case lineRemote:
		return d.theme.Remote.PaddingLeft(2).Width(w).MaxWidth(w)
	case lineOutput:
		return d.theme.Output.PaddingLeft(4).Width(w).MaxWidth(w)
	case lineError:
		return d.theme.Error.Width(w).MaxWidth(w)
	case lineErrorRemote:
		return d.theme.Error.PaddingLeft(2).Width(w).MaxWidth(w)
	case lineErrorOutput:
		return d.theme.Error.PaddingLeft(4).Width(w).MaxWidth(w)
	default:
		return lipgloss.NewStyle().Width(w).MaxWidth(w)
	}
}

func (d *Display) leftPad(k lineKind) int {
	switch k {
	case lineRemote, lineErrorRemote:
		return 2
	case lineOutput, lineErrorOutput:
		return 4
	default:
		return 0
	}
}

func (d *Display) renderLine(l line) string {
	w := max(1, d.width-d.leftPad(l.kind))
	return d.styled(l.kind).Render(ansi.Truncate(l.text, w, ""))
}

func (d *Display) reserve(lines int) {
	if lines <= 0 {
		return
	}
	var buf strings.Builder
	for range lines {
		buf.WriteByte('\n')
	}
	fmt.Fprintf(&buf, "\x1b[%dA", lines)
	os.Stdout.WriteString(buf.String())
	d.ready = true
}

// redraw repaints all provided lines while d.mu is held
func (d *Display) redraw(lines []line) {
	if !d.ready {
		d.reserve(len(lines))
	}
	var buf strings.Builder
	if d.drawn > 0 {
		fmt.Fprintf(&buf, "\x1b[%dA", d.drawn)
	}
	for _, l := range lines {
		buf.WriteString("\x1b[2K")
		buf.WriteString(d.renderLine(l))
		buf.WriteByte('\n')
	}
	if d.drawn > len(lines) {
		extra := d.drawn - len(lines)
		for range extra {
			buf.WriteString("\x1b[2K\n")
		}
		fmt.Fprintf(&buf, "\x1b[%dA", extra)
	}
	os.Stdout.WriteString(buf.String())
	d.drawn = len(lines)
}

func normalizeOutput(msg string) string {
	msg = strings.TrimSpace(msg)
	if msg == "" {
		return outputPlaceholder
	}
	return msg
}

func (d *Display) set(idx int, l line) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if idx >= 0 && idx < len(d.lines) {
		d.lines[idx] = l
	}
	d.redraw(d.lines)
}

func (d *Display) Repo(slot int, msg string) {
	if d.tty {
		d.set(slot*d.stride, line{msg, lineRepo})
	} else {
		d.mu.Lock()
		d.log.Info(msg)
		d.mu.Unlock()
	}
}

func (d *Display) Remote(slot, j int, msg string) {
	if d.tty {
		d.set(slot*d.stride+1+2*j, line{msg, lineRemote})
	} else {
		d.mu.Lock()
		d.log.Info(msg, "indent", 1)
		d.mu.Unlock()
	}
}

func (d *Display) Output(slot, j int, msg string) {
	if d.tty {
		d.set(slot*d.stride+2+2*j, line{normalizeOutput(msg), lineOutput})
	} else {
		msg = strings.TrimSpace(msg)
		if msg == "" {
			return
		}
		d.mu.Lock()
		d.log.Info(msg, "indent", 2)
		d.mu.Unlock()
	}
}

func (d *Display) Error(slot int, msg string) {
	if d.tty {
		d.set(slot*d.stride, line{msg, lineError})
	} else {
		d.mu.Lock()
		d.log.Error(msg)
		d.mu.Unlock()
	}
}

func (d *Display) ErrorRemote(slot, j int, msg string) {
	if d.tty {
		d.set(slot*d.stride+1+2*j, line{msg, lineErrorRemote})
	} else {
		d.mu.Lock()
		d.log.Error(msg, "indent", 1)
		d.mu.Unlock()
	}
}

func (d *Display) ErrorOutput(slot, j int, msg string) {
	if d.tty {
		d.set(slot*d.stride+2+2*j, line{normalizeOutput(msg), lineErrorOutput})
	} else {
		msg = strings.TrimSpace(msg)
		if msg == "" {
			return
		}
		d.mu.Lock()
		d.log.Error(msg, "indent", 2)
		d.mu.Unlock()
	}
}

func (d *Display) Clear(slot int) {
	if d.tty {
		d.mu.Lock()
		defer d.mu.Unlock()
		d.resetSlot(slot)
		d.redraw(d.lines)
	}
}

func (d *Display) Finish() {
	if d.tty {
		d.mu.Lock()
		d.redraw(d.lines)
		d.mu.Unlock()
	}
}
