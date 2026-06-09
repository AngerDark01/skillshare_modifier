//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

func needsSudo(path string) bool {
	return unix.Access(filepath.Dir(path), unix.W_OK) != nil
}

// execFunc is the syscall used to replace the process. Overridden in tests.
var execFunc = unix.Exec

func reexecWithSudo(execPath string) error {
	sudoPath, err := exec.LookPath("sudo")
	if err != nil {
		return fmt.Errorf("sudo not found, please run: sudo %s", strings.Join(os.Args, " "))
	}
	args := []string{"sudo", execPath}
	args = append(args, os.Args[1:]...)
	return execFunc(sudoPath, args, os.Environ())
}
