//go:build windows

package quota

import (
	"os/exec"
	"testing"
)

func TestConfigureBackgroundProcessHidesWindowsConsole(t *testing.T) {
	command := exec.Command("cmd.exe", "/c", "exit")
	configureBackgroundProcess(command)
	if command.SysProcAttr == nil || !command.SysProcAttr.HideWindow {
		t.Fatal("background process did not enable HideWindow")
	}
	if command.SysProcAttr.CreationFlags&createNoWindow == 0 {
		t.Fatal("background process did not enable CREATE_NO_WINDOW")
	}
}
