package display

import (
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
)

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
