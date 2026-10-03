//go:build windows

package main

import (
	"os/exec"
	"syscall"
)

// hideWindow keeps child herdr processes from opening a console window. It matters for the
// hook binary, which has no console of its own (built with -H=windowsgui).
func hideWindow(cmd *exec.Cmd) {
	const createNoWindow = 0x08000000
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: createNoWindow}
}
