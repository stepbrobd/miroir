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
	d.Output(0, 0, "")
	if d.lines[2].text != outputPlaceholder {
		t.Fatalf("empty output should keep placeholder, got %+v", d.lines[2])
	}
}
