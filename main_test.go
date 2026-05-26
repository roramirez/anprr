package main

import (
	"os"
	"os/exec"
	"strings"
	"testing"
)

func TestVersion_flags(t *testing.T) {
	if err := exec.Command("go", "build", "-o", "anprr_test_bin", ".").Run(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	t.Cleanup(func() { exec.Command("rm", "-f", "anprr_test_bin").Run() })

	for _, flag := range []string{"--version", "-v", "version"} {
		out, err := exec.Command("./anprr_test_bin", flag).Output()
		if err != nil {
			t.Errorf("%s: unexpected error: %v", flag, err)
			continue
		}
		got := strings.TrimSpace(string(out))
		if got == "" {
			t.Errorf("%s: expected version output, got empty string", flag)
		}
	}
}

func TestVersion_default(t *testing.T) {
	if version != "dev" {
		t.Errorf("expected default version to be \"dev\", got %q", version)
	}
}

func TestReposAdd_duplicateError(t *testing.T) {
	if err := exec.Command("go", "build", "-o", "anprr_test_bin", ".").Run(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	t.Cleanup(func() { exec.Command("rm", "-f", "anprr_test_bin").Run() })

	xdg := t.TempDir()
	run := func(args ...string) (string, int) {
		cmd := exec.Command("./anprr_test_bin", args...)
		cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+xdg)
		out, _ := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), cmd.ProcessState.ExitCode()
	}

	out, code := run("repos", "add", "owner/repo")
	if code != 0 {
		t.Fatalf("first add failed (exit %d): %s", code, out)
	}

	out, code = run("repos", "add", "owner/repo")
	if code == 0 {
		t.Fatal("expected non-zero exit for duplicate repo, got 0")
	}
	if !strings.Contains(out, "already added") {
		t.Errorf("expected 'already added' in output, got: %s", out)
	}
}

func TestSliceContains(t *testing.T) {
	repos := []string{"owner/a", "owner/b"}
	if !sliceContains(repos, "owner/a") {
		t.Error("expected sliceContains to find owner/a")
	}
	if sliceContains(repos, "owner/c") {
		t.Error("expected sliceContains to not find owner/c")
	}
	if sliceContains(nil, "owner/a") {
		t.Error("expected sliceContains to return false for nil slice")
	}
}

func TestRemoveFromSlice(t *testing.T) {
	tests := []struct {
		name      string
		repos     []string
		remove    string
		wantLen   int
		wantFound bool
	}{
		{"removes existing", []string{"a/b", "c/d"}, "a/b", 1, true},
		{"case insensitive", []string{"A/B", "c/d"}, "a/b", 1, true},
		{"not found", []string{"a/b", "c/d"}, "x/y", 2, false},
		{"empty slice", nil, "a/b", 0, false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got, found := removeFromSlice(tc.repos, tc.remove)
			if found != tc.wantFound {
				t.Errorf("found=%v, want %v", found, tc.wantFound)
			}
			if len(got) != tc.wantLen {
				t.Errorf("len=%d, want %d", len(got), tc.wantLen)
			}
		})
	}
}

func TestReposRemove_removesRepo(t *testing.T) {
	if err := exec.Command("go", "build", "-o", "anprr_test_bin", ".").Run(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	t.Cleanup(func() { exec.Command("rm", "-f", "anprr_test_bin").Run() })

	xdg := t.TempDir()
	run := func(args ...string) (string, int) {
		cmd := exec.Command("./anprr_test_bin", args...)
		cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+xdg)
		out, _ := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), cmd.ProcessState.ExitCode()
	}

	run("repos", "add", "owner/repo")

	out, code := run("repos", "remove", "owner/repo")
	if code != 0 {
		t.Fatalf("remove failed (exit %d): %s", code, out)
	}
	if !strings.Contains(out, "Removed") {
		t.Errorf("expected 'Removed' in output, got: %s", out)
	}

	out, _ = run("repos", "list")
	if strings.Contains(out, "owner/repo") {
		t.Errorf("expected repo to be gone after remove, got: %s", out)
	}
}

func TestReposRemove_notFound(t *testing.T) {
	if err := exec.Command("go", "build", "-o", "anprr_test_bin", ".").Run(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	t.Cleanup(func() { exec.Command("rm", "-f", "anprr_test_bin").Run() })

	xdg := t.TempDir()
	run := func(args ...string) (string, int) {
		cmd := exec.Command("./anprr_test_bin", args...)
		cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+xdg)
		out, _ := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), cmd.ProcessState.ExitCode()
	}

	out, code := run("repos", "remove", "owner/missing")
	if code != 0 {
		t.Fatalf("unexpected non-zero exit: %d — %s", code, out)
	}
	if !strings.Contains(out, "not found") {
		t.Errorf("expected 'not found' in output, got: %s", out)
	}
}

func TestReposRemove_scoped(t *testing.T) {
	if err := exec.Command("go", "build", "-o", "anprr_test_bin", ".").Run(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	t.Cleanup(func() { exec.Command("rm", "-f", "anprr_test_bin").Run() })

	xdg := t.TempDir()
	run := func(args ...string) (string, int) {
		cmd := exec.Command("./anprr_test_bin", args...)
		cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+xdg)
		out, _ := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), cmd.ProcessState.ExitCode()
	}

	run("--scope", "work", "repos", "add", "owner/repo")

	out, code := run("--scope", "work", "repos", "remove", "owner/repo")
	if code != 0 {
		t.Fatalf("scoped remove failed (exit %d): %s", code, out)
	}
	if !strings.Contains(out, "Removed") {
		t.Errorf("expected 'Removed' in output, got: %s", out)
	}
}

func TestReposAdd_duplicateError_scoped(t *testing.T) {
	if err := exec.Command("go", "build", "-o", "anprr_test_bin", ".").Run(); err != nil {
		t.Fatalf("build failed: %v", err)
	}
	t.Cleanup(func() { exec.Command("rm", "-f", "anprr_test_bin").Run() })

	xdg := t.TempDir()
	run := func(args ...string) (string, int) {
		cmd := exec.Command("./anprr_test_bin", args...)
		cmd.Env = append(os.Environ(), "XDG_CONFIG_HOME="+xdg)
		out, _ := cmd.CombinedOutput()
		return strings.TrimSpace(string(out)), cmd.ProcessState.ExitCode()
	}

	run("--scope", "work", "repos", "add", "owner/repo")

	out, code := run("--scope", "work", "repos", "add", "owner/repo")
	if code == 0 {
		t.Fatal("expected non-zero exit for duplicate scoped repo, got 0")
	}
	if !strings.Contains(out, "already in scope") {
		t.Errorf("expected 'already in scope' in output, got: %s", out)
	}
}
