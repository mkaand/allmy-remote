package main

import (
	"errors"
	"os"
	"strings"
	"unsafe"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

const preferenceKey = `Software\AllMyRemote`

type palette struct {
	bg, card, border, text, muted, accent, onAccent, success, danger, disabled uint32
}

type appearanceUI struct {
	title, subtitle, device, origin, serverLabel, grantLabel       uintptr
	preferences, trayToggle, themeLabel, light, auto, dark, footer uintptr
	bodyFont, headingFont, smallFont, titleFont                    uintptr
	bgBrush, cardBrush                                             uintptr
	mode, statusText                                               string
	minimize                                                       bool
	colors                                                         palette
}

type uiRect struct{ Left, Top, Right, Bottom int32 }

type paintStruct struct {
	DC                 uintptr
	Erase              int32
	Rect               uiRect
	Restore, IncUpdate int32
	Reserved           [32]byte
}

type drawItem struct {
	Type, ID, Item, Action, State uint32
	Window, DC                    uintptr
	Rect                          uiRect
	Data                          uintptr
}

func rgb(r, g, b uint32) uint32 { return r | g<<8 | b<<16 }

func validTheme(value string) bool {
	return value == "auto" || value == "light" || value == "dark"
}

func readPreferences() (string, bool) {
	mode, minimize := "auto", true
	k, err := registry.OpenKey(registry.CURRENT_USER, preferenceKey, registry.QUERY_VALUE)
	if err != nil {
		return mode, minimize
	}
	defer k.Close()
	if s, _, err := k.GetStringValue("Theme"); err == nil && validTheme(s) {
		mode = s
	}
	if n, _, err := k.GetIntegerValue("MinimizeToTray"); err == nil && n <= 1 {
		minimize = n == 1
	}
	return mode, minimize
}

func writePreferences(mode string, minimize bool) error {
	if !validTheme(mode) {
		return errors.New("invalid theme preference")
	}
	k, _, err := registry.CreateKey(registry.CURRENT_USER, preferenceKey, registry.SET_VALUE)
	if err != nil {
		return errors.New("cannot save appearance preference")
	}
	defer k.Close()
	if err := k.SetStringValue("Theme", mode); err != nil {
		return err
	}
	var n uint32
	if minimize {
		n = 1
	}
	return k.SetDWordValue("MinimizeToTray", n)
}

func systemDark() bool {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`, registry.QUERY_VALUE)
	if err != nil {
		return false
	}
	defer k.Close()
	n, _, err := k.GetIntegerValue("AppsUseLightTheme")
	return err == nil && n == 0
}

func themeColors(dark bool) palette {
	if dark {
		return palette{rgb(16, 21, 30), rgb(27, 36, 50), rgb(51, 65, 85), rgb(226, 232, 240), rgb(148, 163, 184), rgb(96, 165, 250), rgb(16, 21, 30), rgb(74, 222, 128), rgb(248, 113, 113), rgb(100, 116, 139)}
	}
	return palette{rgb(241, 245, 249), rgb(255, 255, 255), rgb(203, 213, 225), rgb(15, 23, 42), rgb(71, 85, 105), rgb(37, 99, 235), rgb(255, 255, 255), rgb(21, 128, 61), rgb(185, 28, 28), rgb(148, 163, 184)}
}

func makeFont(dpi, size, weight int) uintptr {
	f, _, _ := gdi32.NewProc("CreateFontW").Call(uintptr(-size*dpi/96), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(wide("Segoe UI Variable"))))
	if f == 0 {
		f, _, _ = gdi32.NewProc("CreateFontW").Call(uintptr(-size*dpi/96), 0, 0, 0, uintptr(weight), 0, 0, 0, 1, 0, 0, 5, 0, uintptr(unsafe.Pointer(wide("Segoe UI"))))
	}
	return f
}

func deleteGDI(h uintptr) {
	if h != 0 {
		gdi32.NewProc("DeleteObject").Call(h)
	}
}

func (u *nativeUI) initAppearance() {
	p := &u.appearance
	p.mode, p.minimize = readPreferences()
	p.bodyFont = makeFont(u.dpi, 16, 400)
	p.headingFont = makeFont(u.dpi, 19, 600)
	p.smallFont = makeFont(u.dpi, 14, 400)
	p.titleFont = makeFont(u.dpi, 26, 600)
	for _, h := range []uintptr{p.subtitle, p.serverLabel, p.grantLabel, p.origin, u.server, u.grant, u.status, p.themeLabel} {
		if p.bodyFont != 0 {
			user32.NewProc("SendMessageW").Call(h, 0x0030, p.bodyFont, 1)
		}
	}
	for _, h := range []uintptr{p.device, p.preferences} {
		if p.headingFont != 0 {
			user32.NewProc("SendMessageW").Call(h, 0x0030, p.headingFont, 1)
		}
	}
	if p.titleFont != 0 {
		user32.NewProc("SendMessageW").Call(p.title, 0x0030, p.titleFont, 1)
	}
	if p.smallFont != 0 {
		user32.NewProc("SendMessageW").Call(p.footer, 0x0030, p.smallFont, 1)
	}
	if host, err := os.Hostname(); err == nil && host != "" {
		setText(p.device, host+" | Windows")
	}
	u.applyAppearance()
}

func (u *nativeUI) releaseAppearance() {
	p := &u.appearance
	for _, h := range []uintptr{p.bgBrush, p.cardBrush, p.bodyFont, p.headingFont, p.smallFont, p.titleFont} {
		deleteGDI(h)
	}
}

func (u *nativeUI) applyAppearance() {
	p := &u.appearance
	dark := p.mode == "dark" || (p.mode == "auto" && systemDark())
	p.colors = themeColors(dark)
	oldBG, oldCard := p.bgBrush, p.cardBrush
	p.bgBrush, _, _ = gdi32.NewProc("CreateSolidBrush").Call(uintptr(p.colors.bg))
	p.cardBrush, _, _ = gdi32.NewProc("CreateSolidBrush").Call(uintptr(p.colors.card))
	deleteGDI(oldBG)
	deleteGDI(oldCard)
	value := uint32(0)
	if dark {
		value = 1
	}
	if proc := windows.NewLazySystemDLL("dwmapi.dll").NewProc("DwmSetWindowAttribute"); proc.Find() == nil {
		proc.Call(u.hwnd, 20, uintptr(unsafe.Pointer(&value)), 4)
	}
	u.repaint()
}

func (u *nativeUI) changePreference(mode string, minimize bool) {
	if err := writePreferences(mode, minimize); err != nil {
		alert(u.hwnd, "Could not save preferences.", false)
		return
	}
	u.appearance.mode, u.appearance.minimize = mode, minimize
	u.applyAppearance()
}

func showControl(h uintptr, visible bool) {
	var mode uintptr
	if visible {
		mode = 5
	}
	user32.NewProc("ShowWindow").Call(h, mode)
}

func (u *nativeUI) repaint() {
	user32.NewProc("RedrawWindow").Call(u.hwnd, 0, 0, 0x0085) // invalidate, erase, all children
}

func (u *nativeUI) rect(x, y, w, h int) uiRect {
	s := func(n int) int32 { return int32(n * u.dpi / 96) }
	return uiRect{s(x), s(y), s(x+w), s(y+h)}
}

func rounded(dc uintptr, r uiRect, radius int32, fill, border uint32) {
	brush, _, _ := gdi32.NewProc("CreateSolidBrush").Call(uintptr(fill))
	pen, _, _ := gdi32.NewProc("CreatePen").Call(0, 1, uintptr(border))
	oldB, _, _ := gdi32.NewProc("SelectObject").Call(dc, brush)
	oldP, _, _ := gdi32.NewProc("SelectObject").Call(dc, pen)
	gdi32.NewProc("RoundRect").Call(dc, uintptr(r.Left), uintptr(r.Top), uintptr(r.Right), uintptr(r.Bottom), uintptr(radius), uintptr(radius))
	gdi32.NewProc("SelectObject").Call(dc, oldP)
	gdi32.NewProc("SelectObject").Call(dc, oldB)
	deleteGDI(pen)
	deleteGDI(brush)
}

func (u *nativeUI) paint() {
	var ps paintStruct
	dc, _, _ := user32.NewProc("BeginPaint").Call(u.hwnd, uintptr(unsafe.Pointer(&ps)))
	if dc == 0 {
		return
	}
	defer user32.NewProc("EndPaint").Call(u.hwnd, uintptr(unsafe.Pointer(&ps)))
	var r uiRect
	user32.NewProc("GetClientRect").Call(u.hwnd, uintptr(unsafe.Pointer(&r)))
	p := &u.appearance
	user32.NewProc("FillRect").Call(dc, uintptr(unsafe.Pointer(&r)), p.bgBrush)
	height := 278
	if u.hasState {
		height = 226
	}
	rounded(dc, u.rect(20, 92, 456, height), int32(18*u.dpi/96), p.colors.card, p.colors.border)
	user32.NewProc("DrawIconEx").Call(dc, uintptr(22*u.dpi/96), uintptr(20*u.dpi/96), u.icon, uintptr(44*u.dpi/96), uintptr(44*u.dpi/96), 0, 0, 3)
	status := p.colors.muted
	if strings.HasPrefix(p.statusText, "Connected;") {
		status = p.colors.success
	} else if strings.HasPrefix(p.statusText, "access denied") {
		status = p.colors.danger
	}
	rounded(dc, u.rect(36, 153, 9, 9), int32(9*u.dpi/96), status, status)
}

func (u *nativeUI) controlColor(msg uint32, dc, hwnd uintptr) uintptr {
	p := &u.appearance
	fg, bg, brush := p.colors.text, p.colors.bg, p.bgBrush
	switch hwnd {
	case p.device, u.status, p.serverLabel, p.origin, p.grantLabel, u.server, u.grant:
		bg, brush = p.colors.card, p.cardBrush
	}
	switch hwnd {
	case p.subtitle, p.serverLabel, p.grantLabel, p.footer:
		fg = p.colors.muted
	case u.status:
		if strings.HasPrefix(p.statusText, "Connected;") {
			fg = p.colors.success
		}
	}
	gdi32.NewProc("SetTextColor").Call(dc, uintptr(fg))
	gdi32.NewProc("SetBkColor").Call(dc, uintptr(bg))
	if msg == 0x0138 {
		gdi32.NewProc("SetBkMode").Call(dc, 1)
	}
	return brush
}

func drawLabel(dc, font uintptr, text string, r uiRect, color uint32, flags uintptr) {
	if font != 0 {
		old, _, _ := gdi32.NewProc("SelectObject").Call(dc, font)
		defer gdi32.NewProc("SelectObject").Call(dc, old)
	}
	gdi32.NewProc("SetTextColor").Call(dc, uintptr(color))
	gdi32.NewProc("SetBkMode").Call(dc, 1)
	user32.NewProc("DrawTextW").Call(dc, uintptr(unsafe.Pointer(wide(text))), ^uintptr(0), uintptr(unsafe.Pointer(&r)), flags|0x0800)
}

func (u *nativeUI) drawControl(ptr uintptr) bool {
	if ptr == 0 {
		return false
	}
	d := (*drawItem)(unsafe.Pointer(ptr))
	if d.Type != 4 {
		return false
	}
	p := &u.appearance
	c := p.colors
	r := d.Rect
	brush := p.bgBrush
	if d.ID == idPair || d.ID == idConnect || d.ID == idStop {
		brush = p.cardBrush
	}
	user32.NewProc("FillRect").Call(d.DC, uintptr(unsafe.Pointer(&r)), brush)
	fill, stroke, fg := c.card, c.border, c.text
	disabled := d.State&4 != 0
	label := readText(d.Window)
	switch d.ID {
	case idPair, idConnect:
		fill, stroke, fg = c.accent, c.accent, c.onAccent
	case idStop:
		fg, stroke = c.accent, c.accent
	case idUnpair:
		fg = c.danger
	case idLight, idAuto, idDark:
		mode := map[uint32]string{idLight: "light", idAuto: "auto", idDark: "dark"}[d.ID]
		if mode == p.mode {
			fill, stroke, fg = c.accent, c.accent, c.onAccent
		}
	}
	if disabled {
		fill, stroke, fg = c.card, c.border, c.disabled
	}
	if d.State&1 != 0 && !disabled {
		stroke = c.accent
	}
	rounded(d.DC, r, int32(12*u.dpi/96), fill, stroke)
	if d.ID == idStartup || d.ID == idTray {
		on := u.startEnabled
		detail := "Connect automatically at sign-in"
		if d.ID == idTray {
			on, detail = p.minimize, "Keep running in the notification area"
		}
		s := func(n int) int32 { return int32(n * u.dpi / 96) }
		name := uiRect{r.Left + s(14), r.Top + s(7), r.Right - s(66), r.Top + s(29)}
		sub := uiRect{name.Left, r.Top + s(28), name.Right, r.Bottom - s(3)}
		drawLabel(d.DC, p.bodyFont, label, name, fg, 0x24)
		drawLabel(d.DC, p.smallFont, detail, sub, c.muted, 0x24)
		track := uiRect{r.Right - s(58), r.Top + s(14), r.Right - s(14), r.Top + s(38)}
		color := c.disabled
		if on {
			color = c.accent
		}
		rounded(d.DC, track, s(24), color, color)
		x := track.Left + s(3)
		if on {
			x = track.Right - s(21)
		}
		knob := uiRect{x, track.Top + s(3), x + s(18), track.Bottom - s(3)}
		rounded(d.DC, knob, s(18), rgb(255, 255, 255), rgb(255, 255, 255))
	} else {
		drawLabel(d.DC, p.bodyFont, label, r, fg, 0x25)
	}
	if d.State&0x10 != 0 {
		r.Left += 3
		r.Top += 3
		r.Right -= 3
		r.Bottom -= 3
		user32.NewProc("DrawFocusRect").Call(d.DC, uintptr(unsafe.Pointer(&r)))
	}
	return true
}

func (u *nativeUI) layoutAppearance() {
	p := &u.appearance
	offset := 0
	if u.hasState {
		offset = 52
	}
	px := func(n int) uintptr { return uintptr(n * u.dpi / 96) }
	move := func(h uintptr, x, y, w, height int) {
		user32.NewProc("MoveWindow").Call(h, px(x), px(y-offset), px(w), px(height), 1)
	}
	move(p.preferences, 22, 370, 452, 25)
	move(u.startup, 22, 397, 452, 48)
	move(p.trayToggle, 22, 450, 452, 48)
	move(p.themeLabel, 22, 512, 132, 28)
	move(p.light, 156, 507, 100, 36)
	move(p.auto, 262, 507, 100, 36)
	move(p.dark, 368, 507, 106, 36)
	move(u.unpair, 22, 557, 220, 36)
	move(u.exit, 254, 557, 220, 36)
	move(p.footer, 22, 605, 452, 24)
	user32.NewProc("SetWindowPos").Call(u.hwnd, 0, 0, 0, px(520), px(675-offset), 0x16)
}
