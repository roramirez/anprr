package main

import (
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
