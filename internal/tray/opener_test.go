package tray

import (
	"reflect"
	"testing"
)

func TestOpenerCommand(t *testing.T) {
	tests := []struct {
		goos string
		name string
		args []string
	}{
		{goos: "windows", name: "rundll32.exe", args: []string{"url.dll,FileProtocolHandler", "https://example.com"}},
		{goos: "darwin", name: "open", args: []string{"https://example.com"}},
		{goos: "linux", name: "xdg-open", args: []string{"https://example.com"}},
	}
	for _, test := range tests {
		name, args, err := openerCommand(test.goos, "https://example.com")
		if err != nil {
			t.Fatal(err)
		}
		if name != test.name || !reflect.DeepEqual(args, test.args) {
			t.Fatalf("%s command = %q %v", test.goos, name, args)
		}
	}
}

func TestOpenerCommandRejectsUnsupportedPlatform(t *testing.T) {
	if _, _, err := openerCommand("plan9", "https://example.com"); err == nil {
		t.Fatal("expected unsupported platform error")
	}
}
