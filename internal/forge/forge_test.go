package forge

import (
	"fmt"
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

func TestSrhtIsExists(t *testing.T) {
	tests := []struct {
		msg  string
		want bool
	}{
		{"this name is already in use", true},
		{"repository already exists", true},
		{"Already Exists", true},
		{"invalid name", false},
		{"repository not found", false},
		{"permission denied", false},
		{"internal server error", false},
	}
	for _, tt := range tests {
		got := srhtIsExists(fmt.Errorf("%s", tt.msg))
		if got != tt.want {
			t.Errorf("srhtIsExists(%q) = %v, want %v", tt.msg, got, tt.want)
		}
	}
}
