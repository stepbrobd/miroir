package display

import (
	"bytes"
	"io"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

// quiet makes a tty display that renders into nowhere
func quiet(repos, remotes int) *Display {
	v := true
	d := New(repos, remotes, DefaultTheme, &v)
	d.out = io.Discard
	return d
}

func TestGridIndexingSecondSlot(t *testing.T) {
	d := quiet(2, 2)
	d.Repo(1, "repo1")
	d.Remote(1, 1, "remote11")
	d.Output(1, 1, "out11")
	stride := 1 + 2*2
	if d.lines[stride].text != "repo1" {
		t.Errorf("repo line: got %+v", d.lines[stride])
	}
	if d.lines[stride+3].text != "remote11" {
		t.Errorf("remote line: got %+v", d.lines[stride+3])
	}
	if d.lines[stride+4].text != "out11" {
		t.Errorf("output line: got %+v", d.lines[stride+4])
	}
}

func TestGridErrorLinesRender(t *testing.T) {
	d := quiet(1, 1)
	d.ErrorRemote(0, 0, "origin :: error")
	d.ErrorOutput(0, 0, "boom")
	if d.lines[1].kind != lineErrorRemote || d.lines[1].text != "origin :: error" {
		t.Errorf("error remote line: got %+v", d.lines[1])
	}
	if d.lines[2].kind != lineErrorOutput || d.lines[2].text != "boom" {
		t.Errorf("error output line: got %+v", d.lines[2])
	}
}

func TestFinishWithoutDrawEmitsNothing(t *testing.T) {
	d := quiet(1, 1)
	var buf bytes.Buffer
	d.out = &buf
	d.Finish()
	if buf.Len() != 0 {
		t.Fatalf("expected no output, got %q", buf.String())
	}
}

func TestRedrawRepaintsEveryLine(t *testing.T) {
	d := quiet(1, 1)
	var buf bytes.Buffer
	d.out = &buf
	d.Repo(0, "repo")
	if got := strings.Count(buf.String(), "\x1b[2K"); got != 3 {
		t.Fatalf("expected 3 cleared rows on the first draw got %d in %q", got, buf.String())
	}
}

func TestNewHonorsTTYOverride(t *testing.T) {
	v := false
	d := New(1, 1, DefaultTheme, &v)
	if d.tty {
		t.Fatal("expected non-tty display")
	}
}

func TestPlainModeWritesThroughOut(t *testing.T) {
	v := false
	d := New(1, 1, DefaultTheme, &v)
	var buf bytes.Buffer
	d.out = &buf
	d.Repo(0, "seed :: fetch")
	d.ErrorOutput(0, 0, "boom")
	for _, want := range []string{"seed :: fetch", "boom"} {
		if !strings.Contains(buf.String(), want) {
			t.Fatalf("expected %q in plain output got %q", want, buf.String())
		}
	}
}

func TestClearOnTTY(t *testing.T) {
	d := quiet(1, 1)
	d.lines[0] = line{text: "repo", kind: lineRepo}
	d.lines[1] = line{text: "remote", kind: lineRemote}
	d.Clear(0)
	if d.lines[0].text != "" || d.lines[1].text != "" || d.lines[2].text != outputPlaceholder {
		t.Fatalf("expected slot reset with placeholder, got %+v", d.lines)
	}
}

func TestTTYReservesOutputLinesWithPlaceholder(t *testing.T) {
	d := quiet(1, 2)
	if d.lines[2].text != outputPlaceholder || d.lines[4].text != outputPlaceholder {
		t.Fatalf("expected reserved placeholders, got %+v", d.lines)
	}
}

func TestTTYDoneRemoteKeepsPlaceholderOutput(t *testing.T) {
	d := quiet(1, 1)
	d.Remote(0, 0, "origin :: done")
	if d.lines[2].text != outputPlaceholder {
		t.Fatalf("done remote should keep placeholder got %+v", d.lines[2])
	}
}

func TestTTYDoneRemotePreservesActualOutput(t *testing.T) {
	d := quiet(1, 1)
	d.Output(0, 0, "Everything up-to-date")
	d.Remote(0, 0, "origin :: done")
	if d.lines[2].text != "Everything up-to-date" {
		t.Fatalf("done remote should preserve real output got %+v", d.lines[2])
	}
}

func TestTTYOutputTrimsWhitespace(t *testing.T) {
	d := quiet(1, 1)
	d.Output(0, 0, "   Everything up-to-date   ")
	if d.lines[2].text != "Everything up-to-date" {
		t.Fatalf("expected trimmed output got %+v", d.lines[2])
	}
}

func TestTTYRenderLineTruncatesToOneRow(t *testing.T) {
	d := quiet(1, 1)
	d.width = 24
	got := d.renderLine(line{
		text: "* [new branch]                z3-solver            -> origin/z3-solver",
		kind: lineOutput,
	})
	if strings.Contains(got, "\n") {
		t.Fatalf("expected one rendered row got %q", got)
	}
	if w := ansi.StringWidth(got); w != d.width {
		t.Fatalf("expected rendered width %d got %d with %q", d.width, w, got)
	}
}
