package main

import (
	"os/exec"
	"runtime"
	"strings"
)

// ชื่อ Wi-Fi ที่คอมเครื่องนี้ต่ออยู่ ("" = ต่อสาย LAN หรืออ่านไม่ได้)
// ใช้บอกผู้ใช้ว่ามือถือ/iPad ต้องต่อ Wi-Fi ชื่อเดียวกัน
func wifiName() string {
	run := func(name string, args ...string) string {
		cmd := exec.Command(name, args...)
		hide(cmd)
		out, err := cmd.Output()
		if err != nil {
			return ""
		}
		return string(out)
	}
	field := func(out, key, sep string) string {
		for _, l := range strings.Split(out, "\n") {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, key) {
				if i := strings.Index(l, sep); i >= 0 {
					v := strings.TrimSpace(l[i+len(sep):])
					if v != "" && !strings.Contains(strings.ToLower(v), "redacted") {
						return v
					}
				}
			}
		}
		return ""
	}
	switch runtime.GOOS {
	case "windows":
		out := run("netsh", "wlan", "show", "interfaces")
		for _, l := range strings.Split(out, "\n") {
			l = strings.TrimSpace(l)
			if strings.HasPrefix(l, "SSID") && strings.Contains(l, ":") {
				return strings.TrimSpace(l[strings.Index(l, ":")+1:])
			}
		}
	case "darwin":
		if v := field(run("ipconfig", "getsummary", "en0"), "SSID :", ":"); v != "" {
			return v
		}
		return field(run("networksetup", "-getairportnetwork", "en0"), "Current Wi-Fi Network", ":")
	default:
		for _, l := range strings.Split(run("nmcli", "-t", "-f", "active,ssid", "dev", "wifi"), "\n") {
			if strings.HasPrefix(l, "yes:") {
				return strings.TrimPrefix(l, "yes:")
			}
		}
		return strings.TrimSpace(run("iwgetid", "-r"))
	}
	return ""
}
