//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"
)

var (
	kernel32                  = syscall.NewLazyDLL("kernel32.dll")
	procGetModuleHandleW      = kernel32.NewProc("GetModuleHandleW")
	procRegisterClassExW      = user32.NewProc("RegisterClassExW")
	procCreateWindowExW       = user32.NewProc("CreateWindowExW")
	procDefWindowProcW        = user32.NewProc("DefWindowProcW")
	procGetMessageW           = user32.NewProc("GetMessageW")
	procTranslateMessage      = user32.NewProc("TranslateMessage")
	procDispatchMessageW      = user32.NewProc("DispatchMessageW")
	procCreatePopupMenu       = user32.NewProc("CreatePopupMenu")
	procAppendMenuW           = user32.NewProc("AppendMenuW")
	procTrackPopupMenu        = user32.NewProc("TrackPopupMenu")
	procDestroyMenu           = user32.NewProc("DestroyMenu")
	procGetCursorPos          = user32.NewProc("GetCursorPos")
	procSetForegroundWindow   = user32.NewProc("SetForegroundWindow")
	procPostMessageW          = user32.NewProc("PostMessageW")
	procLoadImageW            = user32.NewProc("LoadImageW")
	procRegisterWindowMessage = user32.NewProc("RegisterWindowMessageW")
	procShellNotifyIconW      = shell32.NewProc("Shell_NotifyIconW")
	procGetSystemMetrics      = user32.NewProc("GetSystemMetrics")
)

const (
	wmApp          = 0x8000
	wmTray         = wmApp + 1
	wmNull         = 0x0000
	wmLButtonUp    = 0x0202
	wmLButtonDbl   = 0x0203
	wmRButtonUp    = 0x0205
	nimAdd         = 0
	nimModify      = 1
	nimDelete      = 2
	nifMessage     = 1
	nifIcon        = 2
	nifTip         = 4
	nifInfo        = 0x10
	mfString       = 0
	mfPopup        = 0x10
	mfGrayed       = 0x1
	mfSeparator    = 0x800
	tpmReturnCmd   = 0x100
	tpmRightButton = 0x2
	cmdOpen        = 1
	cmdQuit        = 2
	cmdGameBase    = 100
)

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         [16]byte
	hBalloonIcon     uintptr
}

type point struct{ x, y int32 }

type msgT struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
	private uint32
}

var (
	trayHwnd    uintptr
	trayNid     notifyIconData
	trayURL     string
	taskbarMsg  uintptr
	trayGameIDs []string
)

func setUTF16(dst []uint16, s string) {
	u := append(utf16.Encode([]rune(strings.ReplaceAll(s, "\x00", ""))), 0)
	if len(u) > len(dst) {
		u = append(u[:len(dst)-1], 0)
	}
	copy(dst, u)
}

func loadTrayIcon() uintptr {
	p := filepath.Join(configDir(), "icon.ico")
	if b, err := uiFS.ReadFile("ui/icon.ico"); err == nil {
		_ = os.MkdirAll(configDir(), 0o755)
		_ = os.WriteFile(p, b, 0o644)
	}
	u, _ := syscall.UTF16PtrFromString(p)
	// Taille « petite icône » du système (16 à 24 px selon l'échelle d'affichage).
	cx, _, _ := procGetSystemMetrics.Call(49) // SM_CXSMICON
	cy, _, _ := procGetSystemMetrics.Call(50) // SM_CYSMICON
	// IMAGE_ICON=1, LR_LOADFROMFILE=0x10
	h, _, _ := procLoadImageW.Call(0, uintptr(unsafe.Pointer(u)), 1, cx, cy, 0x10)
	return h
}

func addTrayIcon() {
	trayNid = notifyIconData{hWnd: trayHwnd, uID: 1, uFlags: nifMessage | nifIcon | nifTip, uCallbackMessage: wmTray, hIcon: loadTrayIcon()}
	trayNid.cbSize = uint32(unsafe.Sizeof(trayNid))
	setUTF16(trayNid.szTip[:], "SimLauncher")
	procShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(&trayNid)))
}

func removeTrayIcon() {
	if trayHwnd != 0 {
		procShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(&trayNid)))
	}
}

// trayNotify affiche une notification Windows depuis l'icône.
func trayNotify(title, text string) {
	if trayHwnd == 0 {
		return
	}
	n := trayNid
	n.uFlags = nifInfo
	setUTF16(n.szInfoTitle[:], title)
	setUTF16(n.szInfo[:], text)
	procShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(&n)))
}

func appendMenu(h uintptr, flags, id uintptr, text string) {
	t, _ := syscall.UTF16PtrFromString(text)
	procAppendMenuW.Call(h, flags, id, uintptr(unsafe.Pointer(t)))
}

func showTrayMenu() {
	c, err := loadConfig()
	menu, _, _ := procCreatePopupMenu.Call()
	defer procDestroyMenu.Call(menu)
	appendMenu(menu, mfString, cmdOpen, T("tray_open"))
	appendMenu(menu, mfSeparator, 0, "")
	trayGameIDs = nil
	if err == nil {
		busy := getStatus().Busy
		for _, g := range c.Games {
			if !g.Enabled {
				continue
			}
			flags := uintptr(mfString)
			if busy {
				flags |= mfGrayed
			}
			appendMenu(menu, flags, uintptr(cmdGameBase+len(trayGameIDs)), T("tray_launch", g.Name))
			trayGameIDs = append(trayGameIDs, g.ID)
		}
		appendMenu(menu, mfSeparator, 0, "")
	}
	appendMenu(menu, mfString, cmdQuit, T("tray_quit"))

	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	procSetForegroundWindow.Call(trayHwnd)
	cmd, _, _ := procTrackPopupMenu.Call(menu, tpmReturnCmd|tpmRightButton, uintptr(pt.x), uintptr(pt.y), 0, trayHwnd, 0)
	procPostMessageW.Call(trayHwnd, wmNull, 0, 0)

	switch {
	case cmd == cmdOpen:
		openUI(trayURL)
	case cmd == cmdQuit:
		quitApp(0)
	case cmd >= cmdGameBase && int(cmd-cmdGameBase) < len(trayGameIDs):
		id := trayGameIDs[cmd-cmdGameBase]
		go func() {
			c, err := loadConfig()
			if err != nil {
				return
			}
			g, _ := c.game(id)
			trayNotify("SimLauncher", T("tray_starting", g.Name))
			if err := launchGame(c, id); err != nil {
				trayNotify(T("err_title"), err.Error())
				return
			}
			if st := getStatus(); st.Result == "warn" {
				trayNotify("SimLauncher", T("tray_warn", g.Name))
			}
			maybeWatch(c, id)
		}()
	}
}

func trayWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	switch {
	case msg == wmTray:
		switch lParam & 0xffff {
		case wmLButtonDbl, wmLButtonUp:
			openUI(trayURL)
		case wmRButtonUp:
			showTrayMenu()
		}
		return 0
	case taskbarMsg != 0 && msg == taskbarMsg: // l'explorateur a redémarré : on remet l'icône
		addTrayIcon()
		return 0
	}
	r, _, _ := procDefWindowProcW.Call(hwnd, msg, wParam, lParam)
	return r
}

// runTray : boucle de messages Windows pour l'icône (bloquante, sur un thread dédié).
func runTray(url string) error {
	runtime.LockOSThread()
	trayURL = url
	hInst, _, _ := procGetModuleHandleW.Call(0)
	cls, _ := syscall.UTF16PtrFromString("SimLauncherTray")
	wc := wndClassEx{lpfnWndProc: syscall.NewCallback(trayWndProc), hInstance: hInst, lpszClassName: cls}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	if r, _, err := procRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc))); r == 0 {
		return fmt.Errorf("RegisterClassEx : %v", err)
	}
	h, _, err := procCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(cls)), 0, 0, 0, 0, 0, 0, 0, hInst, 0)
	if h == 0 {
		return fmt.Errorf("CreateWindowEx : %v", err)
	}
	trayHwnd = h
	tb, _ := syscall.UTF16PtrFromString("TaskbarCreated")
	taskbarMsg, _, _ = procRegisterWindowMessage.Call(uintptr(unsafe.Pointer(tb)))
	addTrayIcon()
	log.Print("Icône de notification active")
	var m msgT
	for {
		r, _, _ := procGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	removeTrayIcon()
	return nil
}

const runKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`

func getAutostart() bool {
	out, err := hidden(exec.Command("reg", "query", runKey, "/v", "SimLauncher")).Output()
	return err == nil && strings.Contains(string(out), "SimLauncher")
}

func setAutostart(on bool) error {
	if !on {
		if !getAutostart() {
			return nil
		}
		return hidden(exec.Command("reg", "delete", runKey, "/v", "SimLauncher", "/f")).Run()
	}
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	return hidden(exec.Command("reg", "add", runKey, "/v", "SimLauncher", "/t", "REG_SZ", "/d", `"`+exe+`" --tray`, "/f")).Run()
}

func quitApp(code int) {
	removeTrayIcon()
	os.Exit(code)
}
