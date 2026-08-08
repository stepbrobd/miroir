package display

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

func captureStdout(t *testing.T, f func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	f()
	w.Close()
	os.Stdout = old
	b, err := io.ReadAll(r)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestGridIndexingSecondSlot(t *testing.T) {
	v := true
	d := New(2, 2, DefaultTheme, &v)
	_ = captureStdout(t, func() {
		d.Repo(1, "repo1")
		d.Remote(1, 1, "remote11")
		d.Output(1, 1, "out11")
	})
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
	v := true
	d := New(1, 1, DefaultTheme, &v)
	_ = captureStdout(t, func() {
		d.ErrorRemote(0, 0, "origin :: error")
		d.ErrorOutput(0, 0, "boom")
	})
	if d.lines[1].kind != lineErrorRemote || d.lines[1].text != "origin :: error" {
		t.Errorf("error remote line: got %+v", d.lines[1])
	}
	if d.lines[2].kind != lineErrorOutput || d.lines[2].text != "boom" {
		t.Errorf("error output line: got %+v", d.lines[2])
	}
}

func TestFinishWithoutDrawEmitsNothing(t *testing.T) {
	v := true
	d := New(1, 1, DefaultTheme, &v)
	if out := captureStdout(t, d.Finish); out != "" {
		t.Fatalf("expected no output, got %q", out)
	}
}

func TestNewHonorsTTYOverride(t *testing.T) {
	v := false
	d := New(1, 1, DefaultTheme, &v)
	if d.tty {
		t.Fatal("expected non-tty display")
	}
}

func TestClearOnTTY(t *testing.T) {
	v := true
	d := New(1, 1, DefaultTheme, &v)
	d.lines[0] = line{text: "repo", kind: lineRepo}
	d.lines[1] = line{text: "remote", kind: lineRemote}
	d.Clear(0)
	if d.lines[0].text != "" || d.lines[1].text != "" || d.lines[2].text != outputPlaceholder {
		t.Fatalf("expected slot reset with placeholder, got %+v", d.lines)
	}
}

func TestTTYReservesOutputLinesWithPlaceholder(t *testing.T) {
	v := true
	d := New(1, 2, DefaultTheme, &v)
	if d.lines[2].text != outputPlaceholder || d.lines[4].text != outputPlaceholder {
		t.Fatalf("expected reserved placeholders, got %+v", d.lines)
	}
}

func TestTTYDoneRemoteKeepsPlaceholderOutput(t *testing.T) {
	v := true
	d := New(1, 1, DefaultTheme, &v)
	d.Remote(0, 0, "origin :: done")
	if d.lines[2].text != outputPlaceholder {
		t.Fatalf("done remote should keep placeholder got %+v", d.lines[2])
	}
}

func TestTTYDoneRemotePreservesActualOutput(t *testing.T) {
	v := true
	d := New(1, 1, DefaultTheme, &v)
	d.Output(0, 0, "Everything up-to-date")
	d.Remote(0, 0, "origin :: done")
	if d.lines[2].text != "Everything up-to-date" {
		t.Fatalf("done remote should preserve real output got %+v", d.lines[2])
	}
}

func TestTTYOutputTrimsWhitespace(t *testing.T) {
	v := true
	d := New(1, 1, DefaultTheme, &v)
	d.Output(0, 0, "   Everything up-to-date   ")
	if d.lines[2].text != "Everything up-to-date" {
		t.Fatalf("expected trimmed output got %+v", d.lines[2])
	}
}

func TestTTYRenderLineTruncatesToOneRow(t *testing.T) {
	v := true
	d := New(1, 1, DefaultTheme, &v)
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
