// Package display provides the default terminal and log-based reporter for miroir.
package display

import (
	"fmt"
	"os"
	"strings"
	"sync"

	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/log"
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

// Display renders a live-updating progress grid in TTY mode
// using direct ANSI escape codes, or structured log in non-TTY mode.
type Display struct {
	tty    bool
	mu     sync.Mutex
	lines  []line
	width  int
	stride int
	theme  Theme
	drawn  bool
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
	} else {
		d.log = log.NewWithOptions(os.Stdout, log.Options{
			ReportTimestamp: false,
			ReportCaller:    false,
		})
	}
	return d
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

// redraw repaints all lines. caller must hold d.mu.
func (d *Display) redraw() {
	var buf strings.Builder
	if d.drawn {
		fmt.Fprintf(&buf, "\x1b[%dA", len(d.lines))
	}
	for _, l := range d.lines {
		buf.WriteString("\x1b[2K")
		buf.WriteString(d.styled(l.kind).Render(l.text))
		buf.WriteByte('\n')
	}
	os.Stdout.WriteString(buf.String())
	d.drawn = true
}

func (d *Display) set(idx int, l line) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if idx >= 0 && idx < len(d.lines) {
		d.lines[idx] = l
	}
	d.redraw()
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
		d.set(slot*d.stride+2+2*j, line{msg, lineOutput})
	} else {
		d.mu.Lock()
		d.log.Debug(msg, "indent", 2)
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
		d.set(slot*d.stride+2+2*j, line{msg, lineErrorOutput})
	} else {
		d.mu.Lock()
		d.log.Error(msg, "indent", 2)
		d.mu.Unlock()
	}
}

func (d *Display) Clear(slot int) {
	if d.tty {
		d.mu.Lock()
		defer d.mu.Unlock()
		base := slot * d.stride
		for i := range d.stride {
			if base+i < len(d.lines) {
				d.lines[base+i] = line{}
			}
		}
		d.redraw()
	}
}

func (d *Display) Finish() {
	if d.tty {
		d.mu.Lock()
		d.redraw()
		d.mu.Unlock()
	}
}
