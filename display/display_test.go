package display

import "testing"

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
	if d.lines[0].text != "" || d.lines[1].text != "" {
		t.Fatal("expected lines to be cleared")
	}
}
