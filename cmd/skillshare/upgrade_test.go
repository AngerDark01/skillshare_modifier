//go:build !windows

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestNeedsSudo_WritableDir(t *testing.T) {
	dir := t.TempDir()
	fakeBin := filepath.Join(dir, "skillshare")
	if needsSudo(fakeBin) {
		t.Error("expected needsSudo=false for writable temp dir")
	}
}

func TestNeedsSudo_NonWritableDir(t *testing.T) {
	if os.Getuid() == 0 {
		t.Skip("test requires non-root user")
	}

	dir := t.TempDir()
	os.Chmod(dir, 0555)
	t.Cleanup(func() { os.Chmod(dir, 0755) })

	fakeBin := filepath.Join(dir, "skillshare")
	if !needsSudo(fakeBin) {
		t.Error("expected needsSudo=true for read-only dir")
	}
}

func TestNeedsSudo_NonExistentDir(t *testing.T) {
	fakeBin := filepath.Join(t.TempDir(), "nonexistent", "skillshare")
	if !needsSudo(fakeBin) {
		t.Error("expected needsSudo=true for non-existent dir")
	}
}

func TestReexecWithSudo_NoSudoInPath(t *testing.T) {
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", "")
	defer os.Setenv("PATH", origPath)

	err := reexecWithSudo("/usr/local/bin/skillshare")
	if err == nil {
		t.Fatal("expected error when sudo is not in PATH")
	}
	if got := err.Error(); !strings.Contains(got, "sudo not found") {
		t.Errorf("expected 'sudo not found' in error, got: %s", got)
	}
}

func TestReexecWithSudo_ExecArgs(t *testing.T) {
	// Capture what execFunc receives
	var gotPath string
	var gotArgs []string

	orig := execFunc
	execFunc = func(argv0 string, argv []string, envv []string) error {
		gotPath = argv0
		gotArgs = argv
		return nil
	}
	defer func() { execFunc = orig }()

	// Fake sudo in PATH
	dir := t.TempDir()
	fakeSudo := filepath.Join(dir, "sudo")
	os.WriteFile(fakeSudo, []byte("#!/bin/sh\n"), 0755)
	t.Setenv("PATH", dir)

	// Override os.Args for the test
	origArgs := os.Args
	os.Args = []string{"skillshare", "upgrade", "--force"}
	defer func() { os.Args = origArgs }()

	err := reexecWithSudo("/usr/local/bin/skillshare")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if gotPath != fakeSudo {
		t.Errorf("exec path = %q, want %q", gotPath, fakeSudo)
	}
	wantArgs := []string{"sudo", "/usr/local/bin/skillshare", "upgrade", "--force"}
	if len(gotArgs) != len(wantArgs) {
		t.Fatalf("args = %v, want %v", gotArgs, wantArgs)
	}
	for i, w := range wantArgs {
		if gotArgs[i] != w {
			t.Errorf("args[%d] = %q, want %q", i, gotArgs[i], w)
		}
	}
}
