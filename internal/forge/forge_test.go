package forge

import (
	"testing"

	"ysun.co/miroir/internal/config"
)

func TestDispatchGithub(t *testing.T) {
	f, err := Dispatch(config.Github, "dummy")
	if err != nil {
		t.Fatal(err)
	}
	if f == nil {
		t.Error("expected non-nil forge")
	}
}

func TestDispatchSourcehut(t *testing.T) {
	f, err := Dispatch(config.Sourcehut, "dummy")
	if err != nil {
		t.Fatal(err)
	}
	if f == nil {
		t.Error("expected non-nil forge")
	}
}

func TestDispatchUnknown(t *testing.T) {
	_, err := Dispatch(config.Forge(99), "dummy")
	if err == nil {
		t.Error("expected error for unknown forge")
	}
}

func TestDescOrEmpty(t *testing.T) {
	s := "hello"
	if got := descOrEmpty(&s); got != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
	if got := descOrEmpty(nil); got != "" {
		t.Errorf("got %q, want empty", got)
	}
}
