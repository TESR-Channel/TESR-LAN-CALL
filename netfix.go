package main

import (
	_ "embed"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

//go:embed packaging/windows/firewall.ps1
var firewallPS1 []byte

func selfExe() string {
	exe, _ := os.Executable()
	if p, err := filepath.EvalSymlinks(exe); err == nil {
		exe = p
	}
	return exe
}

// Windows: รันสคริปต์ไฟร์วอลล์ (โปรเซสนี้ต้องเป็นผู้ดูแลระบบอยู่แล้ว)
func runFirewallScript(port, https, disc int) int {
	if runtime.GOOS != "windows" {
		return 0
	}
	path := filepath.Join(os.TempDir(), "tesr-lan-call-firewall.ps1")
	if err := os.WriteFile(path, firewallPS1, 0o644); err != nil {
		return 3
	}
	defer os.Remove(path)
	cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-File", path,
		"-Exe", selfExe(), "-Tcp", fmt.Sprintf("%d,%d", port, https), "-Udp", fmt.Sprint(disc))
	hide(cmd)
	if err := cmd.Run(); err != nil {
		var ee *exec.ExitError
		if errors.As(err, &ee) {
			return ee.ExitCode()
		}
		return 1
	}
	return 0
}

// ปุ่ม "ซ่อมการเชื่อมต่อมือถือ": ขอสิทธิ์ผู้ดูแลระบบแล้วเปิดไฟร์วอลล์ให้พอร์ตของโปรแกรม
func fixNetwork(port, https, disc int) (string, error) {
	exe := selfExe()
	ok := "เปิดสิทธิ์ให้มือถือเชื่อมต่อแล้ว ลองสแกน QR ใหม่"
	switch runtime.GOOS {
	case "windows":
		ps := fmt.Sprintf(`try { $p = Start-Process -FilePath '%s' -ArgumentList '--fix-firewall','--port','%d','--https-port','%d','--discovery-port','%d' -Verb RunAs -Wait -PassThru; exit $p.ExitCode } catch { exit 99 }`,
			strings.ReplaceAll(exe, "'", "''"), port, https, disc)
		cmd := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command", ps)
		hide(cmd)
		if err := cmd.Run(); err != nil {
			var ee *exec.ExitError
			if errors.As(err, &ee) && ee.ExitCode() == 99 {
				return "", errors.New("ยังไม่ได้กด \"ใช่\" ในหน้าต่างขอสิทธิ์ ลองกดปุ่มอีกครั้ง")
			}
			return "", fmt.Errorf("ตั้งค่าไฟร์วอลล์ไม่สำเร็จ (%v) ลองเปิดโปรแกรมแบบ Run as administrator", err)
		}
		return ok, nil
	case "darwin":
		q := strings.ReplaceAll(exe, `'`, `'\''`)
		fw := "/usr/libexec/ApplicationFirewall/socketfilterfw"
		script := fmt.Sprintf(`do shell script "%s --add '%s'; %s --unblockapp '%s'" with administrator privileges`, fw, q, fw, q)
		if out, err := exec.Command("osascript", "-e", script).CombinedOutput(); err != nil {
			if strings.Contains(string(out), "-128") {
				return "", errors.New("ยกเลิกการใส่รหัสผ่าน ลองกดปุ่มอีกครั้ง")
			}
			return "", fmt.Errorf("ตั้งค่าไฟร์วอลล์ไม่สำเร็จ: %s", strings.TrimSpace(string(out)))
		}
		return ok, nil
	default:
		if _, err := exec.LookPath("ufw"); err != nil {
			return "เครื่องนี้ไม่ได้เปิดไฟร์วอลล์ ถ้ายังเข้าไม่ได้ ให้ตรวจว่ามือถือต่อ Wi-Fi วงเดียวกัน", nil
		}
		sh := fmt.Sprintf("ufw allow %d/tcp; ufw allow %d/tcp; ufw allow %d/udp", port, https, disc)
		if out, err := exec.Command("pkexec", "sh", "-c", sh).CombinedOutput(); err != nil {
			return "", fmt.Errorf("ตั้งค่า ufw ไม่สำเร็จ: %s", strings.TrimSpace(string(out)))
		}
		return ok, nil
	}
}
