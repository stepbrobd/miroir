package display

import (
	"bytes"
	"os"
	"strings"
	"sync"

	tea "github.com/charmbracelet/bubbletea"
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
type clearMsg struct{ slot int }
type finishMsg struct{}

type model struct {
	lines   []string
	stride  int
	repos   int
	remotes int
	theme   Theme
}

func newModel(repos, remotes int, th Theme) model {
	stride := 1 + 2*remotes
	total := repos * stride
	lines := make([]string, max(1, total))
	return model{lines: lines, stride: stride, repos: repos, remotes: remotes, theme: th}
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

func (m model) View() string {
	var b strings.Builder
	for i, l := range m.lines {
		b.WriteString(l)
		if i < len(m.lines)-1 {
			b.WriteByte('\n')
		}
	}
	return b.String()
}

type Display struct {
	tty  bool
	prog *tea.Program
	done chan error
	log  *log.Logger
	mu   sync.Mutex // guards log writes in non-TTY mode
}

// bubbletea when stdout is a TTY, charm log otherwise
func New(repos, remotes int, th Theme) *Display {
	tty := term.IsTerminal(int(os.Stdout.Fd()))
	d := &Display{tty: tty, done: make(chan error, 1)}

	if tty {
		m := newModel(repos, remotes, th)
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
