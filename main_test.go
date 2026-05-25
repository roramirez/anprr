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
