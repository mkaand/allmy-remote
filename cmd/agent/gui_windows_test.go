package main

import (
	"strings"
	"testing"
)

func TestStartupCommand(t *testing.T) {
	path := `C:\Program Files\AllMy Remote\allmy-remote.exe`
	got, err := startupCommand(path)
	if err != nil || got != `"C:\Program Files\AllMy Remote\allmy-remote.exe" --autostart` {
		t.Fatal("startup command did not quote the executable path")
	}
	for _, invalid := range []string{"relative.exe", `C:\bad"name.exe`, `C:\` + strings.Repeat("a", 260) + ".exe"} {
		if _, err := startupCommand(invalid); err == nil {
			t.Fatal("unsafe startup command accepted")
		}
	}
}

func TestEmbeddedApplicationIcon(t *testing.T) {
	instance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	icon, _, _ := user32.NewProc("LoadImageW").Call(instance, 101, 1, 32, 32, 0)
	if icon == 0 {
		t.Fatal("application icon resource could not be loaded")
	}
	user32.NewProc("DestroyIcon").Call(icon)
}
