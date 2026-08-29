package tray

import (
	"errors"
	"os/exec"
	"runtime"
)

func OpenURL(target string) error {
	return openTarget(runtime.GOOS, target)
}

func OpenPath(target string) error {
	return openTarget(runtime.GOOS, target)
}

func openTarget(goos, target string) error {
	name, arguments, err := openerCommand(goos, target)
	if err != nil {
		return err
	}
	command := exec.Command(name, arguments...)
	if err := command.Start(); err != nil {
		return err
	}
	return command.Process.Release()
}

func openerCommand(goos, target string) (string, []string, error) {
	switch goos {
	case "windows":
		return "rundll32.exe", []string{"url.dll,FileProtocolHandler", target}, nil
	case "darwin":
		return "open", []string{target}, nil
	case "linux", "freebsd", "openbsd", "netbsd":
		return "xdg-open", []string{target}, nil
	default:
		return "", nil, errors.New("opening desktop targets is unsupported on this platform")
	}
}
