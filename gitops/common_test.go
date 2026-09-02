package gitops

import (
	"os"
	"path/filepath"
	"testing"
)

func TestEnsureRepoMissing(t *testing.T) {
	err := ensureRepo(t.TempDir())
	if err == nil {
		t.Error("expected error for missing .git")
	}
}

func TestIsDirty(t *testing.T) {
	if err := Available(); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	git(t, dir, nil, "init")
	dirty, err := isDirty(t.Context(), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if dirty {
		t.Error("fresh repo should not be dirty")
	}
	if err := os.WriteFile(filepath.Join(dir, "tmp.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	dirty, err = isDirty(t.Context(), dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !dirty {
		t.Error("repo with untracked files should be dirty")
	}
}
