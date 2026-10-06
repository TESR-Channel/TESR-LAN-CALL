//go:build !windows

package main

import (
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"sync"
)

var (
	inhibMu sync.Mutex
	inhib   *exec.Cmd
)

// ไม่ให้คอมเข้าโหมด Sleep ระหว่างเปิดโปรแกรม (จอดับได้ตามปกติ)
// macOS ใช้ caffeinate, Linux ใช้ systemd-inhibit (ถ้ามี) ผูกกับอายุของโปรแกรมนี้
func preventSleep(on bool) {
	inhibMu.Lock()
	defer inhibMu.Unlock()
	if !on {
		if inhib != nil && inhib.Process != nil {
			_ = inhib.Process.Kill()
		}
		inhib = nil
		return
	}
	if inhib != nil {
		return
	}
	pid := strconv.Itoa(os.Getpid())
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("caffeinate", "-i", "-w", pid)
	default:
		if _, err := exec.LookPath("systemd-inhibit"); err != nil {
			return
		}
		cmd = exec.Command("systemd-inhibit", "--what=idle:sleep", "--who=TESR LAN Call",
			"--why=Waiting for video calls", "--mode=block", "tail", "--pid="+pid, "-f", "/dev/null")
	}
	if err := cmd.Start(); err != nil {
		return
	}
	inhib = cmd
	go func(c *exec.Cmd) {
		_ = c.Wait()
		inhibMu.Lock()
		if inhib == c {
			inhib = nil
		}
		inhibMu.Unlock()
	}(cmd)
}
