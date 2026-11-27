package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"unsafe"

	"github.com/mkaand/allmy-remote/internal/agent"
	"golang.org/x/sys/windows"
)

func crypt(data []byte, protect bool) ([]byte, error) {
	if len(data) == 0 || len(data) > 16384 {
		return nil, errors.New("invalid local credential data")
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	var out windows.DataBlob
	var err error
	if protect {
		err = windows.CryptProtectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptUnprotectData(&in, nil, nil, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	if err != nil {
		return nil, errors.New("Windows DPAPI operation failed")
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))
	if out.Size == 0 || out.Size > 16384 || out.Data == nil {
		return nil, errors.New("invalid DPAPI output")
	}
	view := unsafe.Slice(out.Data, int(out.Size))
	result := append([]byte(nil), view...)
	clear(view)
	return result, nil
}

func readGrant() (string, error) {
	h := windows.Handle(os.Stdin.Fd())
	var mode uint32
	if err := windows.GetConsoleMode(h, &mode); err != nil {
		return "", errors.New("interactive Windows console required")
	}
	if err := windows.SetConsoleMode(h, mode&^windows.ENABLE_ECHO_INPUT); err != nil {
		return "", errors.New("cannot disable console echo")
	}
	defer windows.SetConsoleMode(h, mode)
	fmt.Print("One-time enrollment grant (hidden): ")
	defer fmt.Println()
	line, err := bufio.NewReaderSize(os.Stdin, 128).ReadSlice('\n')
	if err != nil {
		return "", errors.New("cannot read grant")
	}
	defer clear(line)
	return strings.TrimSpace(string(line)), nil
}

func localPaths() (string, windows.Handle, error) {
	root := os.Getenv("LOCALAPPDATA")
	if !filepath.IsAbs(root) {
		return "", 0, errors.New("local Windows application data unavailable")
	}
	dir := filepath.Join(root, "AllMyRemote")
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", 0, errors.New("cannot create local agent directory")
	}
	name, err := windows.UTF16PtrFromString(filepath.Join(dir, "agent.lock"))
	if err != nil {
		return "", 0, errors.New("invalid local lock path")
	}
	h, err := windows.CreateFile(name, windows.GENERIC_READ|windows.GENERIC_WRITE, 0, nil, windows.OPEN_ALWAYS, windows.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		return "", 0, errors.New("another agent operation is running or local lock is unavailable")
	}
	return filepath.Join(dir, "enrollment.dpapi"), h, nil
}

func save(path string, s agent.State) error {
	plain, err := json.Marshal(s)
	if err != nil {
		return err
	}
	defer clear(plain)
	data, err := crypt(plain, true)
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("cannot create protected enrollment")
	}
	_, writeErr := f.Write(data)
	syncErr := f.Sync()
	closeErr := f.Close()
	if writeErr != nil || syncErr != nil || closeErr != nil {
		_ = os.Remove(path)
		return errors.New("cannot save protected enrollment")
	}
	return nil
}

func load(path string) (agent.State, error) {
	var s agent.State
	f, err := os.Open(path)
	if err != nil {
		return s, errors.New("no saved enrollment; pair first")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, 16385))
	if err != nil {
		return s, errors.New("cannot read protected enrollment")
	}
	plain, err := crypt(data, false)
	if err != nil {
		return s, err
	}
	defer clear(plain)
	if json.Unmarshal(plain, &s) != nil || !s.Valid() {
		return agent.State{}, errors.New("invalid protected enrollment")
	}
	return s, nil
}

func run() error {
	if len(os.Args) < 2 {
		return errors.New("usage: allmy-agent pair --server https://remote.example.com | run | unpair")
	}
	var origin string
	switch os.Args[1] {
	case "pair":
		fs := flag.NewFlagSet("pair", flag.ContinueOnError)
		fs.SetOutput(io.Discard)
		server := fs.String("server", "", "HTTPS server origin")
		if err := fs.Parse(os.Args[2:]); err != nil || fs.NArg() != 0 {
			return errors.New("usage: allmy-agent pair --server HTTPS_ORIGIN")
		}
		var err error
		origin, err = agent.Origin(*server)
		if err != nil {
			return err
		}
	case "run", "unpair":
		if len(os.Args) != 2 {
			return errors.New("unexpected arguments")
		}
	default:
		return errors.New("unknown operation")
	}
	path, lock, err := localPaths()
	if err != nil {
		return err
	}
	defer windows.CloseHandle(lock)
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	switch os.Args[1] {
	case "pair":
		if _, err := os.Lstat(path); !errors.Is(err, os.ErrNotExist) {
			return errors.New("existing enrollment or inaccessible state; revoke and unpair first")
		}
		fmt.Println("Pairing authorizes device presence only. No remote control or clipboard.")
		fmt.Println("Server:", origin)
		grant, err := readGrant()
		if err != nil {
			return err
		}
		s, err := agent.Enroll(ctx, origin, grant, http.DefaultClient)
		if err != nil {
			return err
		}
		if err := save(path, s); err != nil {
			return errors.New("enrollment issued but local save failed; revoke the new device in the dashboard")
		}
		fmt.Println("Paired. Run the agent explicitly to connect.")
	case "run":
		s, err := load(path)
		if err != nil {
			return err
		}
		fmt.Println("AllMy Remote agent: presence only. Ctrl+C stops the connection.")
		return agent.RunWithTelemetry(ctx, s, http.DefaultClient, func(message string) { fmt.Println(message) }, windowsCollector())
	case "unpair":
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return errors.New("cannot remove local enrollment")
		}
		if err := setStartup(false); err != nil {
			return errors.New("local enrollment removed; disable Windows startup manually and revoke in the dashboard")
		}
		fmt.Println("Local enrollment removed. Revoke the device in the dashboard too.")
	}
	return nil
}

// guiBuild is set only for the standalone Windows-subsystem executable.
var guiBuild = "0"

func main() {
	if guiBuild == "1" {
		if len(os.Args) > 2 || (len(os.Args) == 2 && os.Args[1] != "--autostart") {
			alert(0, "Use the separate console agent for command-line operations.", false)
			return
		}
		if err := runGUI(len(os.Args) == 2); err != nil {
			alert(0, err.Error(), false)
			os.Exit(1)
		}
		return
	}
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
