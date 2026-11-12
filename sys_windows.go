//go:build windows

package main

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
	"time"
	"unsafe"
)

var (
	shell32           = syscall.NewLazyDLL("shell32.dll")
	procShellExecuteW = shell32.NewProc("ShellExecuteW")
	user32            = syscall.NewLazyDLL("user32.dll")
	procMessageBoxW   = user32.NewProc("MessageBoxW")
)

type winSys struct{}

func newSys() Sys { return winSys{} }

func (winSys) Running() (map[string]bool, error) {
	snap, err := syscall.CreateToolhelp32Snapshot(syscall.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil, err
	}
	defer syscall.CloseHandle(snap)
	var pe syscall.ProcessEntry32
	pe.Size = uint32(unsafe.Sizeof(pe))
	out := map[string]bool{}
	if err := syscall.Process32First(snap, &pe); err != nil {
		return nil, err
	}
	for {
		out[strings.ToLower(syscall.UTF16ToString(pe.ExeFile[:]))] = true
		if err := syscall.Process32Next(snap, &pe); err != nil {
			break
		}
	}
	return out, nil
}

func shellExecute(verb, file, params, dir string) error {
	p := func(s string) *uint16 {
		if s == "" {
			return nil
		}
		u, _ := syscall.UTF16PtrFromString(s)
		return u
	}
	r, _, _ := procShellExecuteW.Call(0,
		uintptr(unsafe.Pointer(p(verb))), uintptr(unsafe.Pointer(p(file))),
		uintptr(unsafe.Pointer(p(params))), uintptr(unsafe.Pointer(p(dir))), 1 /*SW_SHOWNORMAL*/)
	if r <= 32 {
		return TE("shell_fail", r)
	}
	return nil
}

// Start passe par ShellExecute : gère les programmes qui demandent l'élévation (UAC).
func (winSys) Start(path, args, dir string) error { return shellExecute("open", path, args, dir) }

func (winSys) OpenURL(url string) error { return shellExecute("open", url, "", "") }

func hidden(cmd *exec.Cmd) *exec.Cmd {
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	return cmd
}

// Kill ferme proprement (WM_CLOSE), puis force après 8 s si besoin.
func (s winSys) Kill(process string) error {
	_ = hidden(exec.Command("taskkill", "/IM", process)).Run()
	for i := 0; i < 8; i++ {
		time.Sleep(time.Second)
		r, err := s.Running()
		if err == nil && !r[strings.ToLower(process)] {
			return nil
		}
	}
	return hidden(exec.Command("taskkill", "/F", "/IM", process)).Run()
}

func (winSys) FileExists(path string) bool {
	st, err := os.Stat(path)
	return err == nil && !st.IsDir()
}

// openUI ouvre l'interface dans une fenêtre d'application Edge (sans barre d'adresse).
func openUI(url string) {
	for _, p := range []string{
		os.Getenv("ProgramFiles(x86)") + `\Microsoft\Edge\Application\msedge.exe`,
		os.Getenv("ProgramFiles") + `\Microsoft\Edge\Application\msedge.exe`,
	} {
		if (winSys{}).FileExists(p) {
			if shellExecute("open", p, "--app="+url+" --window-size=1180,820", "") == nil {
				return
			}
		}
	}
	_ = shellExecute("open", url, "", "")
}

func messageBox(title, text string) {
	t, _ := syscall.UTF16PtrFromString(title)
	m, _ := syscall.UTF16PtrFromString(text)
	procMessageBoxW.Call(0, uintptr(unsafe.Pointer(m)), uintptr(unsafe.Pointer(t)), 0x30 /*MB_ICONWARNING*/)
}

var (
	comdlg32             = syscall.NewLazyDLL("comdlg32.dll")
	procGetOpenFileNameW = comdlg32.NewProc("GetOpenFileNameW")
	ole32                = syscall.NewLazyDLL("ole32.dll")
	procCoInitializeEx   = ole32.NewProc("CoInitializeEx")
)

type openFileName struct {
	lStructSize       uint32
	hwndOwner         uintptr
	hInstance         uintptr
	lpstrFilter       *uint16
	lpstrCustomFilter *uint16
	nMaxCustFilter    uint32
	nFilterIndex      uint32
	lpstrFile         *uint16
	nMaxFile          uint32
	lpstrFileTitle    *uint16
	nMaxFileTitle     uint32
	lpstrInitialDir   *uint16
	lpstrTitle        *uint16
	Flags             uint32
	nFileOffset       uint16
	nFileExtension    uint16
	lpstrDefExt       *uint16
	lCustData         uintptr
	lpfnHook          uintptr
	lpTemplateName    *uint16
	pvReserved        uintptr
	dwReserved        uint32
	FlagsEx           uint32
}

// pickFile ouvre la boîte de dialogue Windows « Ouvrir » et renvoie le chemin choisi.
func pickFile(initial string) (string, error) {
	ch := make(chan [2]string)
	go func() {
		runtime.LockOSThread()
		defer runtime.UnlockOSThread()
		procCoInitializeEx.Call(0, 2)
		buf := make([]uint16, 1024)
		if initial != "" {
			copy(buf, syscall.StringToUTF16(initial))
		}
		filter := syscall.StringToUTF16(T("pick_exe") + "\x00*.exe\x00" + T("pick_all") + "\x00*.*\x00\x00")
		title, _ := syscall.UTF16PtrFromString(T("pick_title"))
		ofn := openFileName{lpstrFilter: &filter[0], lpstrFile: &buf[0], nMaxFile: uint32(len(buf)), lpstrTitle: title,
			Flags: 0x1000 | 0x800 | 0x8 | 0x80000}
		ofn.lStructSize = uint32(unsafe.Sizeof(ofn))
		r, _, _ := procGetOpenFileNameW.Call(uintptr(unsafe.Pointer(&ofn)))
		if r == 0 {
			ch <- [2]string{"", ""}
			return
		}
		ch <- [2]string{syscall.UTF16ToString(buf), ""}
	}()
	res := <-ch
	return res[0], nil
}

var procGetUserDefaultUILanguage = syscall.NewLazyDLL("kernel32.dll").NewProc("GetUserDefaultUILanguage")

// systemLang renvoie la langue de l'interface Windows (fr, en, pt, es, zh…).
func systemLang() string {
	r, _, _ := procGetUserDefaultUILanguage.Call()
	switch r & 0x3ff { // langue principale
	case 0x0c:
		return "fr"
	case 0x16:
		return "pt"
	case 0x0a:
		return "es"
	case 0x04:
		return "zh"
	case 0x09:
		return "en"
	}
	return envLang()
}
