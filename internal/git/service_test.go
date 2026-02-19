package git

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBuildSSHURL(t *testing.T) {
	svc := NewService()
	cases := []struct {
		in   string
		want string
	}{
		{"https://github.com/org/repo.git", "git@github-work:org/repo.git"},
		{"git@github.com:org/repo.git", "git@github-work:org/repo.git"},
		{"ssh://git@github.com/org/repo.git", "git@github-work:org/repo.git"},
	}
	for _, c := range cases {
		got, err := svc.BuildSSHURL(c.in, "github-work")
		if err != nil {
			t.Fatalf("unexpected error for %q: %v", c.in, err)
		}
		if got != c.want {
			t.Fatalf("for %q want %q got %q", c.in, c.want, got)
		}
	}
}

func TestResolveCloneTargetExistingNonEmptyDir(t *testing.T) {
	base := t.TempDir()
	if err := os.WriteFile(filepath.Join(base, "marker.txt"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	got, err := resolveCloneTarget(base, "git@gh-work:org/my-repo.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := filepath.Join(base, "my-repo")
	if got != want {
		t.Fatalf("want %q got %q", want, got)
	}
}

func TestResolveCloneTargetExistingEmptyDir(t *testing.T) {
	base := t.TempDir()
	got, err := resolveCloneTarget(base, "git@gh-work:org/my-repo.git")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != base {
		t.Fatalf("want %q got %q", base, got)
	}
}
