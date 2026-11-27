package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"runtime"
	"strings"
	"sync"
	"unsafe"

	"github.com/mkaand/allmy-remote/internal/agent"
	"golang.org/x/sys/windows"
)

var user32 = windows.NewLazySystemDLL("user32.dll")
var shell32 = windows.NewLazySystemDLL("shell32.dll")
var kernel32 = windows.NewLazySystemDLL("kernel32.dll")
var gdi32 = windows.NewLazySystemDLL("gdi32.dll")

const (
	wmCommand = 0x0111
	wmClose   = 0x0010
	wmDestroy = 0x0002
	wmSystem  = 0x0112
	wmTray    = 0x8001
	wmUpdate  = 0x8002
	idPair    = 101
	idConnect = 102
	idStop    = 103
	idUnpair  = 104
	idStartup = 105
	idExit    = 106
	idOpen    = 107
)

type windowClass struct {
	Size, Style                        uint32
	Proc                               uintptr
	ClassExtra, WindowExtra            int32
	Instance, Icon, Cursor, Background uintptr
	MenuName, ClassName                *uint16
	SmallIcon                          uintptr
}

type point struct{ X, Y int32 }

type winMessage struct {
	Window         uintptr
	Message        uint32
	WParam, LParam uintptr
	Time           uint32
	Point          point
	Private        uint32
}

type trayData struct {
	Size                uint32
	Window              uintptr
	ID, Flags, Callback uint32
	Icon                uintptr
	Tip                 [128]uint16
	State, StateMask    uint32
	Info                [256]uint16
	Version             uint32
	InfoTitle           [64]uint16
	InfoFlags           uint32
	GUID                [16]byte
	BalloonIcon         uintptr
}

type uiEvent struct {
	message string
	done    bool
}

type nativeUI struct {
	hwnd, instance, icon, font uintptr
	server, grant, status      uintptr
	pair, connect, stop        uintptr
	unpair, startup, exit      uintptr
	path                       string
	state                      agent.State
	hasState, busy, exiting    bool
	tray, startEnabled         bool
	startReadable              bool
	dpi                        int
	taskbar                    uint32
	cancel                     context.CancelFunc
	mu                         sync.Mutex
	event                      uiEvent
}

var currentUI *nativeUI

func wide(s string) *uint16 {
	p, _ := windows.UTF16PtrFromString(s)
	return p
}

func alert(hwnd uintptr, message string, confirm bool) bool {
	flags := uintptr(0x40)
	if confirm {
		flags = 0x24 | 0x100 // Yes/No, question, default No.
	}
	r, _, _ := user32.NewProc("MessageBoxW").Call(hwnd, uintptr(unsafe.Pointer(wide(message))), uintptr(unsafe.Pointer(wide("AllMy Remote"))), flags)
	return r == 6
}

func setText(hwnd uintptr, value string) {
	user32.NewProc("SetWindowTextW").Call(hwnd, uintptr(unsafe.Pointer(wide(value))))
}

func readText(hwnd uintptr) string {
	var buf [513]uint16
	n, _, _ := user32.NewProc("GetWindowTextW").Call(hwnd, uintptr(unsafe.Pointer(&buf[0])), uintptr(len(buf)))
	value := windows.UTF16ToString(buf[:n])
	clear(buf[:])
	return value
}

func show(hwnd uintptr, visible bool) {
	mode := uintptr(0)
	if visible {
		mode = 9 // SW_RESTORE
	}
	user32.NewProc("ShowWindow").Call(hwnd, mode)
	if visible {
		user32.NewProc("SetForegroundWindow").Call(hwnd)
	}
}

func enable(hwnd uintptr, yes bool) {
	value := uintptr(0)
	if yes {
		value = 1
	}
	user32.NewProc("EnableWindow").Call(hwnd, value)
}

func (u *nativeUI) control(class, title string, style uint32, x, y, w, h, id int) uintptr {
	px := func(n int) uintptr { return uintptr(n * u.dpi / 96) }
	hwnd, _, _ := user32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(wide(class))), uintptr(unsafe.Pointer(wide(title))), uintptr(style|0x50000000), px(x), px(y), px(w), px(h), u.hwnd, uintptr(id), u.instance, 0)
	if hwnd != 0 {
		user32.NewProc("SendMessageW").Call(hwnd, 0x0030, u.font, 1)
	}
	return hwnd
}

func (u *nativeUI) build() error {
	const button = 0x00010000
	const edit = 0x00810080 // border, tab stop, horizontal scroll
	u.control("STATIC", "AllMy Remote | Windows agent", 0, 18, 12, 464, 24, 0)
	u.control("STATIC", "Presence only. No remote control or clipboard.", 0, 18, 41, 464, 22, 0)
	u.control("STATIC", "HTTPS server", 0, 18, 74, 464, 20, 0)
	u.server = u.control("EDIT", "", edit, 18, 96, 464, 28, 0)
	u.control("STATIC", "One-time enrollment grant", 0, 18, 136, 464, 20, 0)
	u.grant = u.control("EDIT", "", edit|0x20, 18, 158, 464, 28, 0)
	u.pair = u.control("BUTTON", "Pair device", button, 18, 198, 148, 32, idPair)
	u.connect = u.control("BUTTON", "Connect", button, 176, 198, 148, 32, idConnect)
	u.stop = u.control("BUTTON", "Disconnect", button, 334, 198, 148, 32, idStop)
	u.status = u.control("STATIC", "Not connected", 0, 18, 242, 464, 44, 0)
	u.startup = u.control("BUTTON", "Start with Windows (current user)", button|3, 18, 292, 464, 28, idStartup)
	u.control("STATIC", "Close or minimize to tray. Exit stops the agent.", 0, 18, 326, 464, 22, 0)
	u.unpair = u.control("BUTTON", "Unpair", button, 18, 360, 148, 32, idUnpair)
	u.exit = u.control("BUTTON", "Exit", button, 334, 360, 148, 32, idExit)
	for _, h := range []uintptr{u.server, u.grant, u.pair, u.connect, u.stop, u.status, u.startup, u.unpair, u.exit} {
		if h == 0 {
			return errors.New("cannot create agent controls")
		}
	}
	user32.NewProc("SendMessageW").Call(u.server, 0x00c5, 512, 0)
	user32.NewProc("SendMessageW").Call(u.grant, 0x00c5, 128, 0)
	return nil
}

func (u *nativeUI) notify(action uintptr, text string) bool {
	d := trayData{Window: u.hwnd, ID: 1, Flags: 7, Callback: wmTray, Icon: u.icon}
	d.Size = uint32(unsafe.Sizeof(d))
	v, _ := windows.UTF16FromString("AllMy Remote - " + text)
	copy(d.Tip[:127], v)
	r, _, _ := shell32.NewProc("Shell_NotifyIconW").Call(action, uintptr(unsafe.Pointer(&d)))
	return r != 0
}

func (u *nativeUI) report(text string) {
	setText(u.status, text)
	if u.tray {
		u.notify(1, text)
	}
}

func (u *nativeUI) refresh() {
	paired := u.state.Valid()
	enable(u.server, !u.busy && !u.hasState)
	enable(u.grant, !u.busy && !u.hasState)
	enable(u.pair, !u.busy && !u.hasState)
	enable(u.connect, !u.busy && paired)
	enable(u.stop, u.busy && u.cancel != nil && !u.exiting)
	enable(u.unpair, !u.busy && u.hasState)
	enable(u.startup, !u.busy && u.startReadable && (paired || u.startEnabled))
	checked := uintptr(0)
	if u.startEnabled {
		checked = 1
	}
	user32.NewProc("SendMessageW").Call(u.startup, 0x00f1, checked, 0)
}

func (u *nativeUI) reload() {
	u.state = agent.State{}
	_, err := os.Lstat(u.path)
	u.hasState = !errors.Is(err, os.ErrNotExist)
	if u.hasState {
		s, err := load(u.path)
		if err != nil {
			u.report(err.Error())
		} else {
			u.state = s
			setText(u.server, s.Origin)
			u.report("Paired; disconnected.")
		}
	} else {
		setText(u.server, "")
		u.report("Not paired. Generate a grant in the dashboard.")
	}
	u.startEnabled, err = startupEnabled()
	u.startReadable = err == nil
	u.refresh()
}

func (u *nativeUI) publish(e uiEvent) {
	u.mu.Lock()
	u.event = e
	u.mu.Unlock()
	user32.NewProc("PostMessageW").Call(u.hwnd, wmUpdate, 0, 0)
}

func (u *nativeUI) begin(task func(context.Context) error, initial string) {
	ctx, cancel := context.WithCancel(context.Background())
	u.cancel = cancel
	u.busy = true
	u.report(initial)
	u.refresh()
	go func() {
		err := task(ctx)
		message := "Disconnected."
		if err != nil {
			message = err.Error()
		}
		cancel()
		u.publish(uiEvent{message: message, done: true})
	}()
}

func (u *nativeUI) start() {
	if u.busy || !u.state.Valid() {
		return
	}
	s := u.state
	u.begin(func(ctx context.Context) error {
		return agent.Run(ctx, s, http.DefaultClient, func(message string) {
			u.publish(uiEvent{message: message})
		})
	}, "Connecting...")
}

func (u *nativeUI) pairDevice() {
	if u.busy || u.hasState {
		return
	}
	origin, err := agent.Origin(strings.TrimSpace(readText(u.server)))
	if err != nil {
		u.report(err.Error())
		return
	}
	grant := strings.TrimSpace(readText(u.grant))
	setText(u.grant, "")
	u.begin(func(ctx context.Context) error {
		s, err := agent.Enroll(ctx, origin, grant, http.DefaultClient)
		if err != nil {
			return err
		}
		if err := save(u.path, s); err != nil {
			return errors.New("Enrollment issued; local save failed. Revoke the new dashboard device.")
		}
		return nil
	}, "Pairing...")
}

func (u *nativeUI) stopWork() {
	if u.cancel != nil {
		u.cancel()
		u.cancel = nil
		u.report("Stopping...")
		u.refresh()
	}
}

func (u *nativeUI) quit() {
	u.exiting = true
	if u.busy {
		u.stopWork()
		return
	}
	user32.NewProc("DestroyWindow").Call(u.hwnd)
}

func (u *nativeUI) command(id int) {
	switch id {
	case idOpen:
		show(u.hwnd, true)
	case idPair:
		u.pairDevice()
	case idConnect:
		u.start()
	case idStop:
		u.stopWork()
	case idExit:
		u.quit()
	case idUnpair:
		if u.busy || !u.hasState || !alert(u.hwnd, "Remove local enrollment and disable Auto-Start? Revoke the device in the dashboard too.", true) {
			return
		}
		if err := setStartup(false); err != nil {
			u.report(err.Error())
			return
		}
		if err := os.Remove(u.path); err != nil && !errors.Is(err, os.ErrNotExist) {
			u.reload()
			u.report("Cannot remove local enrollment.")
			return
		}
		u.reload()
	case idStartup:
		if u.busy || !u.startReadable {
			return
		}
		want := !u.startEnabled
		if want && !u.state.Valid() {
			u.refresh()
			return
		}
		if want && !alert(u.hwnd, "Start at Windows sign-in and connect using this enrollment? Keep this EXE in a permanent location. You can disable this at any time.", true) {
			u.refresh()
			return
		}
		if err := setStartup(want); err != nil {
			u.report(err.Error())
		} else {
			u.startEnabled = want
		}
		u.refresh()
	}
}

func (u *nativeUI) menu() {
	menu, _, _ := user32.NewProc("CreatePopupMenu").Call()
	if menu == 0 {
		return
	}
	defer user32.NewProc("DestroyMenu").Call(menu)
	appendItem := func(id int, title string, disabled bool) {
		flags := uintptr(0)
		if disabled {
			flags = 1
		}
		user32.NewProc("AppendMenuW").Call(menu, flags, uintptr(id), uintptr(unsafe.Pointer(wide(title))))
	}
	appendItem(idOpen, "Open AllMy Remote", false)
	appendItem(idConnect, "Connect", u.busy || !u.state.Valid())
	appendItem(idStop, "Disconnect", !u.busy || u.cancel == nil)
	appendItem(idExit, "Exit", false)
	var p point
	user32.NewProc("GetCursorPos").Call(uintptr(unsafe.Pointer(&p)))
	user32.NewProc("SetForegroundWindow").Call(u.hwnd)
	id, _, _ := user32.NewProc("TrackPopupMenu").Call(menu, 0x102, uintptr(p.X), uintptr(p.Y), 0, u.hwnd, 0)
	user32.NewProc("PostMessageW").Call(u.hwnd, 0, 0, 0)
	if id != 0 {
		u.command(int(id))
	}
}

func windowProc(hwnd uintptr, msg uint32, wparam, lparam uintptr) uintptr {
	u := currentUI
	if u != nil && hwnd == u.hwnd {
		switch msg {
		case wmCommand:
			if wparam>>16 == 0 {
				u.command(int(wparam & 0xffff))
			}
			return 0
		case wmClose:
			if u.tray {
				show(hwnd, false)
			} else {
				u.quit()
			}
			return 0
		case wmSystem:
			if wparam&0xfff0 == 0xf020 && u.tray {
				show(hwnd, false)
				return 0
			}
		case wmTray:
			switch lparam {
			case 0x0202, 0x0203:
				show(hwnd, true)
			case 0x0205:
				u.menu()
			}
			return 0
		case wmUpdate:
			u.mu.Lock()
			e := u.event
			u.event = uiEvent{}
			u.mu.Unlock()
			if e.done {
				u.busy, u.cancel = false, nil
				if u.exiting {
					u.quit()
					return 0
				}
				wasPaired := u.state.Valid()
				u.reload()
				if wasPaired || !u.state.Valid() || e.message != "Disconnected." {
					u.report(e.message)
				}
			} else if e.message != "" && u.cancel != nil {
				u.report(e.message)
			}
			return 0
		case wmDestroy:
			if u.tray {
				u.notify(2, "")
			}
			user32.NewProc("PostQuitMessage").Call(0)
			return 0
		default:
			if msg == u.taskbar && msg != 0 {
				u.tray = u.notify(0, "Open agent")
				if !u.tray {
					show(hwnd, true)
				}
				return 0
			}
		}
	}
	r, _, _ := user32.NewProc("DefWindowProcW").Call(hwnd, uintptr(msg), wparam, lparam)
	return r
}

func runGUI(auto bool) error {
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	path, lock, err := localPaths()
	if err != nil {
		return err
	}
	defer windows.CloseHandle(lock)
	if auto {
		on, err := startupEnabled()
		if err != nil || !on {
			return errors.New("Windows startup is not enabled")
		}
	}
	user32.NewProc("SetProcessDPIAware").Call()
	dpi, _, _ := user32.NewProc("GetDpiForSystem").Call()
	if dpi < 96 || dpi > 384 {
		dpi = 96
	}
	instance, _, _ := kernel32.NewProc("GetModuleHandleW").Call(0)
	icon, _, _ := user32.NewProc("LoadImageW").Call(instance, 101, 1, 32, 32, 0)
	if icon == 0 {
		return errors.New("application icon unavailable")
	}
	defer user32.NewProc("DestroyIcon").Call(icon)
	cursor, _, _ := user32.NewProc("LoadCursorW").Call(0, 32512)
	font, _, _ := gdi32.NewProc("GetStockObject").Call(17)
	u := &nativeUI{path: path, instance: instance, icon: icon, font: font, dpi: int(dpi)}
	currentUI = u
	defer func() { currentUI = nil }()
	name := wide("AllMyRemoteAgentWindow")
	wc := windowClass{Proc: windows.NewCallback(windowProc), Instance: instance, Icon: icon, Cursor: cursor, Background: 6, ClassName: name, SmallIcon: icon}
	wc.Size = uint32(unsafe.Sizeof(wc))
	atom, _, _ := user32.NewProc("RegisterClassExW").Call(uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		return errors.New("cannot register agent window")
	}
	defer user32.NewProc("UnregisterClassW").Call(uintptr(unsafe.Pointer(name)), instance)
	taskbar, _, _ := user32.NewProc("RegisterWindowMessageW").Call(uintptr(unsafe.Pointer(wide("TaskbarCreated"))))
	u.taskbar = uint32(taskbar)
	// Fixed client layout; system DPI scaling keeps the controls readable.
	hwnd, _, _ := user32.NewProc("CreateWindowExW").Call(0, uintptr(unsafe.Pointer(name)), uintptr(unsafe.Pointer(wide("AllMy Remote"))), 0x00ca0000, 0x80000000, 0x80000000, 520*dpi/96, 455*dpi/96, 0, 0, instance, 0)
	if hwnd == 0 {
		return errors.New("cannot create agent window")
	}
	u.hwnd = hwnd
	if err := u.build(); err != nil {
		user32.NewProc("DestroyWindow").Call(hwnd)
		return err
	}
	u.tray = u.notify(0, "Open agent")
	u.reload()
	if !auto || !u.tray || !u.state.Valid() {
		show(hwnd, true)
	}
	if u.state.Valid() {
		u.start()
	}
	var msg winMessage
	for {
		r, _, _ := user32.NewProc("GetMessageW").Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if int32(r) < 0 {
			u.stopWork()
			return errors.New("Windows message loop failed")
		}
		if r == 0 {
			return nil
		}
		handled, _, _ := user32.NewProc("IsDialogMessageW").Call(hwnd, uintptr(unsafe.Pointer(&msg)))
		if handled == 0 {
			user32.NewProc("TranslateMessage").Call(uintptr(unsafe.Pointer(&msg)))
			user32.NewProc("DispatchMessageW").Call(uintptr(unsafe.Pointer(&msg)))
		}
	}
}
