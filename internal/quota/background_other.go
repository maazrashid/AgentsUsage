//go:build !windows

package quota

import "os/exec"

func configureBackgroundProcess(_ *exec.Cmd) {}
