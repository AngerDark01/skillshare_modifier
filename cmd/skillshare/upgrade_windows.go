//go:build windows

package main

import "fmt"

func needsSudo(_ string) bool {
	return false
}

func reexecWithSudo(_ string) error {
	return fmt.Errorf("sudo is not supported on Windows")
}
