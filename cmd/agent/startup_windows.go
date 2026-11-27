package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf16"

	"golang.org/x/sys/windows/registry"
)

const startupKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const startupValue = "AllMyRemote"

func startupCommand(exe string) (string, error) {
	if !filepath.IsAbs(exe) || strings.ContainsAny(exe, "\"\r\n\x00") {
		return "", errors.New("invalid application path")
	}
	cmd := "\"" + exe + "\" --autostart"
	if len(utf16.Encode([]rune(cmd))) > 250 {
		return "", errors.New("application path too long for Windows startup")
	}
	return cmd, nil
}

func startupEnabled() (bool, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, startupKey, registry.QUERY_VALUE)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errors.New("cannot read Windows startup preference")
	}
	defer k.Close()
	_, _, err = k.GetStringValue(startupValue)
	if errors.Is(err, registry.ErrNotExist) {
		return false, nil
	}
	if err != nil {
		return false, errors.New("cannot read Windows startup preference")
	}
	return true, nil
}

func setStartup(enable bool) error {
	if !enable {
		k, err := registry.OpenKey(registry.CURRENT_USER, startupKey, registry.SET_VALUE)
		if errors.Is(err, registry.ErrNotExist) {
			return nil
		}
		if err != nil {
			return errors.New("cannot disable Windows startup")
		}
		defer k.Close()
		if err = k.DeleteValue(startupValue); err != nil && !errors.Is(err, registry.ErrNotExist) {
			return errors.New("cannot disable Windows startup")
		}
		return nil
	}
	exe, err := os.Executable()
	if err != nil {
		return errors.New("cannot determine application path")
	}
	cmd, err := startupCommand(exe)
	if err != nil {
		return err
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, startupKey, registry.SET_VALUE)
	if err != nil {
		return errors.New("cannot enable Windows startup")
	}
	defer k.Close()
	if err = k.SetStringValue(startupValue, cmd); err != nil {
		return errors.New("cannot enable Windows startup")
	}
	return nil
}
