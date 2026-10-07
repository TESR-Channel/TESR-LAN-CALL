package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// ======================================================================== app window

func findChromium() string {
	var cands []string
	switch runtime.GOOS {
	case "windows":
		pf, pf86, lad := os.Getenv("ProgramFiles"), os.Getenv("ProgramFiles(x86)"), os.Getenv("LocalAppData")
		cands = []string{
			filepath.Join(pf86, `Microsoft\Edge\Application\msedge.exe`),
			filepath.Join(pf, `Microsoft\Edge\Application\msedge.exe`),
			filepath.Join(pf, `Google\Chrome\Application\chrome.exe`),
			filepath.Join(pf86, `Google\Chrome\Application\chrome.exe`),
			filepath.Join(lad, `Google\Chrome\Application\chrome.exe`),
		}
	case "darwin":
		home, _ := os.UserHomeDir()
		for _, base := range []string{"/Applications", filepath.Join(home, "Applications")} {
			cands = append(cands,
				filepath.Join(base, "Google Chrome.app/Contents/MacOS/Google Chrome"),
				filepath.Join(base, "Microsoft Edge.app/Contents/MacOS/Microsoft Edge"),
				filepath.Join(base, "Chromium.app/Contents/MacOS/Chromium"))
		}
	default:
		for _, n := range []string{"google-chrome", "google-chrome-stable", "chromium", "chromium-browser", "microsoft-edge"} {
			if p, err := exec.LookPath(n); err == nil {
				cands = append(cands, p)
			}
		}
	}
	for _, c := range cands {
		if st, err := os.Stat(c); err == nil && !st.IsDir() {
			return c
		}
	}
	return ""
}

// เปิดหน้าโปรแกรมเป็นหน้าต่างแอป (ไม่มีแถบเบราว์เซอร์) ด้วย Edge/Chrome ที่มีในเครื่อง
// เป็นหน้าต่างปกติที่ย่อ/ขยาย/พับเก็บ/ปิดได้เสมอ (ไม่ใช้ --kiosk แล้ว)
// fullscreen=true (โหมดหุ่นยนต์): เปิดเต็มจอ แต่ออกได้ด้วย Esc หรือปุ่ม "ออกจากเต็มจอ"
func openWindow(port int, fullscreen bool) {
	url := fmt.Sprintf("http://localhost:%d/", port)
	// หน้าต่างเปิดอยู่แล้ว (อาจพับเก็บไว้) ให้ดึงขึ้นมาแทนการเปิดซ้ำ
	if _, err := windowDo(port, "front", nil); err == nil {
		if fullscreen {
			_, _ = windowDo(port, "fullscreen", nil)
		}
		return
	} else if !errors.Is(err, errNoWindow) {
		_ = os.Remove(devtoolsFile()) // ไฟล์ค้างจากรอบก่อน
	}
	if exe := findChromium(); exe != "" {
		args := []string{"--app=" + url, "--user-data-dir=" + profileDir(), "--remote-debugging-port=0",
			"--autoplay-policy=no-user-gesture-required", "--no-first-run", "--no-default-browser-check"}
		// ครั้งแรกกำหนดขนาด ครั้งต่อไปให้เบราว์เซอร์จำขนาด/ตำแหน่งที่ผู้ใช้ปรับไว้เอง
		if _, err := os.Stat(filepath.Join(profileDir(), "Default", "Preferences")); err != nil {
			args = append(args, "--window-size=1200,800")
		}
		if fullscreen {
			args = append(args, "--start-fullscreen", "--use-fake-ui-for-media-stream")
		}
		cmd := exec.Command(exe, args...)
		if err := cmd.Start(); err == nil {
			go func() { _ = cmd.Wait() }()
			if fullscreen {
				go ensureFullscreen(port)
			}
			return
		}
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	hide(cmd)
	if err := cmd.Start(); err != nil {
		log.Println("[window]", err)
	}
}

// ======================================================================== autostart

func exePath() string {
	p, err := os.Executable()
	if err != nil {
		return os.Args[0]
	}
	if r, err := filepath.EvalSymlinks(p); err == nil {
		return r
	}
	return p
}

const winRunKey = `HKCU\Software\Microsoft\Windows\CurrentVersion\Run`

func autostartFile() string {
	home, _ := os.UserHomeDir()
	if runtime.GOOS == "darwin" {
		return filepath.Join(home, "Library/LaunchAgents/com.tesr.lancall.plist")
	}
	return filepath.Join(home, ".config/autostart/tesr-lan-call.desktop")
}

func autostartEnabled() bool {
	if runtime.GOOS == "windows" {
		cmd := exec.Command("reg", "query", winRunKey, "/v", "TESR LAN Call")
		hide(cmd)
		return cmd.Run() == nil
	}
	_, err := os.Stat(autostartFile())
	return err == nil
}

// robot=true: เปิดเต็มจอ (kiosk) ตอนเปิดเครื่อง, ไม่งั้นทำงานเบื้องหลังแล้วเด้งหน้าต่างเมื่อมีสายเข้า
func setAutostart(enable, robot bool) error {
	mode := "--background"
	if robot {
		mode = "--robot"
	}
	exe := exePath()
	switch runtime.GOOS {
	case "windows":
		var cmd *exec.Cmd
		if enable {
			cmd = exec.Command("reg", "add", winRunKey, "/v", "TESR LAN Call", "/t", "REG_SZ", "/d", fmt.Sprintf(`"%s" %s`, exe, mode), "/f")
		} else {
			cmd = exec.Command("reg", "delete", winRunKey, "/v", "TESR LAN Call", "/f")
		}
		hide(cmd)
		if err := cmd.Run(); err != nil && enable {
			return err
		}
		return nil
	case "darwin":
		f := autostartFile()
		if !enable {
			_ = os.Remove(f)
			return nil
		}
		_ = os.MkdirAll(filepath.Dir(f), 0o755)
		plist := `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0"><dict><key>Label</key><string>com.tesr.lancall</string>
<key>ProgramArguments</key><array><string>` + exe + `</string><string>` + mode + `</string></array>
<key>RunAtLoad</key><true/></dict></plist>
`
		return os.WriteFile(f, []byte(plist), 0o644)
	default:
		f := autostartFile()
		if !enable {
			_ = os.Remove(f)
			return nil
		}
		_ = os.MkdirAll(filepath.Dir(f), 0o755)
		desk := "[Desktop Entry]\nType=Application\nName=TESR LAN Call\nExec=\"" + strings.ReplaceAll(exe, `"`, `\"`) + "\" " + mode +
			"\nIcon=tesr-lan-call\nX-GNOME-Autostart-enabled=true\n"
		return os.WriteFile(f, []byte(desk), 0o644)
	}
}
