//go:build windows

package quota

import (
	"os/exec"
	"syscall"
)

const createNoWindow = 0x08000000

func configureBackgroundProcess(command *exec.Cmd) {
	if command.SysProcAttr == nil {
		command.SysProcAttr = &syscall.SysProcAttr{}
	}
	command.SysProcAttr.HideWindow = true
	command.SysProcAttr.CreationFlags |= createNoWindow
}
