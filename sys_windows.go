//go:build windows

package main

import (
	"log"
	"os"
	"os/exec"
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

// pickFile ouvre la boîte de dialogue « Ouvrir » et renvoie le chemin choisi ("" si annulé).
// Elle tourne dans un PowerShell séparé (OpenFileDialog de .NET) : un souci dans la
// boîte de dialogue ou une extension de l'Explorateur ne peut pas faire tomber SimLauncher.
func pickFile(initial string, image bool) (string, error) {
	filter := T("pick_exe") + "|*.exe|" + T("pick_all") + "|*.*"
	title := T("pick_title")
	if image {
		filter = T("pick_image") + "|*.jpg;*.jpeg;*.png;*.webp;*.gif;*.bmp|" + T("pick_all") + "|*.*"
		title = T("pick_image_title")
	}
	const script = `[Console]::OutputEncoding=[Text.Encoding]::UTF8
Add-Type -AssemblyName System.Windows.Forms
$owner=New-Object System.Windows.Forms.Form -Property @{TopMost=$true;ShowInTaskbar=$false}
$d=New-Object System.Windows.Forms.OpenFileDialog
$d.Filter=$env:SL_FILTER; $d.Title=$env:SL_TITLE; $d.CheckFileExists=$true
if($env:SL_INITIAL){ try{ $d.InitialDirectory=[IO.Path]::GetDirectoryName($env:SL_INITIAL); $d.FileName=[IO.Path]::GetFileName($env:SL_INITIAL) }catch{} }
if($d.ShowDialog($owner) -eq [System.Windows.Forms.DialogResult]::OK){ [Console]::Out.Write($d.FileName) }`
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-STA", "-ExecutionPolicy", "Bypass", "-Command", script)
	cmd.Env = append(os.Environ(), "SL_FILTER="+filter, "SL_TITLE="+title, "SL_INITIAL="+initial)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true, CreationFlags: 0x08000000}
	out, err := cmd.Output()
	if err != nil {
		log.Printf("Boîte « Ouvrir » : %v", err)
		return "", TE("pick_fail", err)
	}
	return strings.TrimSpace(string(out)), nil
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
