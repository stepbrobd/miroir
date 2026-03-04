package display

import (
	"bytes"
	"os"
	"strings"
	"sync"

	tea "charm.land/bubbletea/v2"
	lipgloss "charm.land/lipgloss/v2"
	"github.com/charmbracelet/log"
	"golang.org/x/term"
)

type repoMsg struct {
	slot int
	text string
}
type remoteMsg struct {
	slot, j int
	text    string
}
type outputMsg struct {
	slot, j int
	text    string
}
type errorMsg struct {
	slot int
	text string
}
type errorRemoteMsg struct {
	slot, j int
	text    string
}
type errorOutputMsg struct {
	slot, j int
	text    string
}
type clearMsg struct{ slot int }
type finishMsg struct{}

type model struct {
	lines   []string
	width   int
	stride  int
	repos   int
	remotes int
	theme   Theme
}

func newModel(repos, remotes, width int, th Theme) model {
	stride := 1 + 2*remotes
	total := repos * stride
	lines := make([]string, max(1, total))
	return model{lines: lines, width: width, stride: stride, repos: repos, remotes: remotes, theme: th}
}

func (m model) Init() tea.Cmd { return nil }

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case repoMsg:
		if l := msg.slot * m.stride; l < len(m.lines) {
			m.lines[l] = m.theme.Repo.Render(msg.text)
		}
	case remoteMsg:
		if l := msg.slot*m.stride + 1 + 2*msg.j; l < len(m.lines) {
			m.lines[l] = m.theme.Remote.Render(msg.text)
		}
	case outputMsg:
		if l := msg.slot*m.stride + 2 + 2*msg.j; l < len(m.lines) {
			m.lines[l] = m.theme.Output.Render(msg.text)
		}
	case errorMsg:
		if l := msg.slot * m.stride; l < len(m.lines) {
			m.lines[l] = m.theme.Error.Render(msg.text)
		}
	case errorRemoteMsg:
		if l := msg.slot*m.stride + 1 + 2*msg.j; l < len(m.lines) {
			m.lines[l] = m.theme.Error.PaddingLeft(2).Render(msg.text)
		}
	case errorOutputMsg:
		if l := msg.slot*m.stride + 2 + 2*msg.j; l < len(m.lines) {
			m.lines[l] = m.theme.Error.PaddingLeft(4).Render(msg.text)
		}
	case clearMsg:
		base := msg.slot * m.stride
		for i := range m.stride {
			if base+i < len(m.lines) {
				m.lines[base+i] = ""
			}
		}
	case finishMsg:
		return m, tea.Quit
	}
	return m, nil
}

func (m model) View() tea.View {
	var b strings.Builder
	for i, l := range m.lines {
		b.WriteString(l)
		if pad := m.width - lipgloss.Width(l); pad > 0 {
			b.WriteString(strings.Repeat(" ", pad))
		}
		if i < len(m.lines)-1 {
			b.WriteByte('\n')
		}
	}
	return tea.NewView(b.String())
}

type Display struct {
	tty  bool
	prog *tea.Program
	done chan error
	log  *log.Logger
	mu   sync.Mutex // guards log writes in non-TTY mode
}

// bubbletea when stdout is a TTY, charm log otherwise.
// ttyOverride forces the mode when non-nil.
func New(repos, remotes int, th Theme, ttyOverride *bool) *Display {
	tty := term.IsTerminal(int(os.Stdout.Fd()))
	if ttyOverride != nil {
		tty = *ttyOverride
	}
	d := &Display{tty: tty, done: make(chan error, 1)}

	if tty {
		w, _, _ := term.GetSize(int(os.Stdout.Fd()))
		if w <= 0 {
			w = 80
		}
		m := newModel(repos, remotes, w, th)
		d.prog = tea.NewProgram(m,
			tea.WithInput(bytes.NewReader(nil)),
			tea.WithoutSignalHandler(),
		)
		go func() {
			_, err := d.prog.Run()
			d.done <- err
		}()
	} else {
		d.log = log.NewWithOptions(os.Stdout, log.Options{
			ReportTimestamp: false,
			ReportCaller:    false,
		})
	}
	return d
}

func (d *Display) Repo(slot int, msg string) {
	if d.tty {
		d.prog.Send(repoMsg{slot, msg})
	} else {
		d.mu.Lock()
		d.log.Info(msg)
		d.mu.Unlock()
	}
}

func (d *Display) Remote(slot, j int, msg string) {
	if d.tty {
		d.prog.Send(remoteMsg{slot, j, msg})
	} else {
		d.mu.Lock()
		d.log.Info(msg, "indent", 1)
		d.mu.Unlock()
	}
}

func (d *Display) Output(slot, j int, msg string) {
	if d.tty {
		d.prog.Send(outputMsg{slot, j, msg})
	} else {
		d.mu.Lock()
		d.log.Debug(msg, "indent", 2)
		d.mu.Unlock()
	}
}

func (d *Display) Error(slot int, msg string) {
	if d.tty {
		d.prog.Send(errorMsg{slot, msg})
	} else {
		d.mu.Lock()
		d.log.Error(msg)
		d.mu.Unlock()
	}
}

func (d *Display) ErrorRemote(slot, j int, msg string) {
	if d.tty {
		d.prog.Send(errorRemoteMsg{slot, j, msg})
	} else {
		d.mu.Lock()
		d.log.Error(msg, "indent", 1)
		d.mu.Unlock()
	}
}

func (d *Display) ErrorOutput(slot, j int, msg string) {
	if d.tty {
		d.prog.Send(errorOutputMsg{slot, j, msg})
	} else {
		d.mu.Lock()
		d.log.Error(msg, "indent", 2)
		d.mu.Unlock()
	}
}

func (d *Display) Clear(slot int) {
	if d.tty {
		d.prog.Send(clearMsg{slot})
	}
}

func (d *Display) Finish() {
	if d.tty {
		d.prog.Send(finishMsg{})
		<-d.done
	}
}
