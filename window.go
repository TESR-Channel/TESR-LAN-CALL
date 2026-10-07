package main

// ควบคุมหน้าต่างโปรแกรม (Edge/Chrome แบบ --app) ผ่าน Chrome DevTools Protocol ที่เปิดเฉพาะ 127.0.0.1
// ใช้ได้เหมือนกันทุกระบบ: Windows, macOS, Linux, Raspberry Pi
// ทำได้: พับเก็บ, หน้าต่างเล็ก, ขยาย, เต็มจอ, กลับขนาดปกติ, ปิดหน้าต่าง, ดึงหน้าต่างขึ้นมาตอนมีสายเข้า

import (
	"bufio"
	"crypto/rand"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

var errNoWindow = errors.New("ไม่พบหน้าต่างโปรแกรม")

func profileDir() string   { return filepath.Join(dataDir, "window") }
func devtoolsFile() string { return filepath.Join(profileDir(), "DevToolsActivePort") }

// ------------------------------------------------------------------ websocket client ขนาดเล็ก (เฉพาะ localhost)

type cdpConn struct {
	c  net.Conn
	br *bufio.Reader
	id int
}

func cdpDial() (*cdpConn, error) {
	b, err := os.ReadFile(devtoolsFile())
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSpace(string(b)), "\n")
	if len(lines) < 2 {
		return nil, errors.New("DevToolsActivePort ไม่สมบูรณ์")
	}
	addr := "127.0.0.1:" + strings.TrimSpace(lines[0])
	path := strings.TrimSpace(lines[1])
	if !strings.HasPrefix(path, "/devtools/browser/") {
		return nil, errors.New("DevToolsActivePort ไม่ถูกต้อง")
	}
	c, err := net.DialTimeout("tcp", addr, 800*time.Millisecond)
	if err != nil {
		return nil, err
	}
	_ = c.SetDeadline(time.Now().Add(5 * time.Second))
	key := make([]byte, 16)
	_, _ = rand.Read(key)
	req := fmt.Sprintf("GET %s HTTP/1.1\r\nHost: %s\r\nUpgrade: websocket\r\nConnection: Upgrade\r\n"+
		"Sec-WebSocket-Key: %s\r\nSec-WebSocket-Version: 13\r\n\r\n", path, addr, base64.StdEncoding.EncodeToString(key))
	if _, err := io.WriteString(c, req); err != nil {
		c.Close()
		return nil, err
	}
	br := bufio.NewReader(c)
	resp, err := http.ReadResponse(br, nil)
	if err != nil {
		c.Close()
		return nil, err
	}
	if resp.StatusCode != http.StatusSwitchingProtocols {
		c.Close()
		return nil, fmt.Errorf("devtools handshake: %s", resp.Status)
	}
	return &cdpConn{c: c, br: br}, nil
}

func (w *cdpConn) close() { _ = w.c.Close() }

func (w *cdpConn) send(p []byte) error {
	h := []byte{0x81} // FIN + text
	n := len(p)
	switch {
	case n < 126:
		h = append(h, 0x80|byte(n))
	case n < 1<<16:
		h = append(h, 0x80|126, byte(n>>8), byte(n))
	default:
		h = append(h, 0x80|127)
		var l [8]byte
		binary.BigEndian.PutUint64(l[:], uint64(n))
		h = append(h, l[:]...)
	}
	mask := make([]byte, 4)
	_, _ = rand.Read(mask)
	h = append(h, mask...)
	m := make([]byte, n)
	for i := range p {
		m[i] = p[i] ^ mask[i%4]
	}
	_, err := w.c.Write(append(h, m...))
	return err
}

func (w *cdpConn) recv() ([]byte, error) {
	var msg []byte
	for {
		var h [2]byte
		if _, err := io.ReadFull(w.br, h[:]); err != nil {
			return nil, err
		}
		fin, op := h[0]&0x80 != 0, h[0]&0x0f
		n := uint64(h[1] & 0x7f)
		switch n {
		case 126:
			var e [2]byte
			if _, err := io.ReadFull(w.br, e[:]); err != nil {
				return nil, err
			}
			n = uint64(binary.BigEndian.Uint16(e[:]))
		case 127:
			var e [8]byte
			if _, err := io.ReadFull(w.br, e[:]); err != nil {
				return nil, err
			}
			n = binary.BigEndian.Uint64(e[:])
		}
		var mask []byte
		if h[1]&0x80 != 0 {
			mask = make([]byte, 4)
			if _, err := io.ReadFull(w.br, mask); err != nil {
				return nil, err
			}
		}
		if n > 32<<20 {
			return nil, errors.New("devtools frame too large")
		}
		p := make([]byte, n)
		if _, err := io.ReadFull(w.br, p); err != nil {
			return nil, err
		}
		if mask != nil {
			for i := range p {
				p[i] ^= mask[i%4]
			}
		}
		switch op {
		case 8:
			return nil, io.EOF
		case 9, 10:
			continue
		}
		msg = append(msg, p...)
		if fin {
			return msg, nil
		}
	}
}

func (w *cdpConn) call(method string, params map[string]any, out any) error {
	if params == nil {
		params = map[string]any{}
	}
	w.id++
	b, _ := json.Marshal(map[string]any{"id": w.id, "method": method, "params": params})
	if err := w.send(b); err != nil {
		return err
	}
	for {
		m, err := w.recv()
		if err != nil {
			return err
		}
		var r struct {
			ID     int             `json:"id"`
			Result json.RawMessage `json:"result"`
			Error  *struct {
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(m, &r) != nil || r.ID != w.id {
			continue // event หรือข้อความอื่น
		}
		if r.Error != nil {
			return errors.New(r.Error.Message)
		}
		if out != nil {
			return json.Unmarshal(r.Result, out)
		}
		return nil
	}
}

// ------------------------------------------------------------------ คำสั่งหน้าต่าง

type winBounds struct {
	Left        int    `json:"left"`
	Top         int    `json:"top"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	WindowState string `json:"windowState,omitempty"`
}

var (
	winMu   sync.Mutex
	winLast = "normal" // สถานะล่าสุดที่ไม่ใช่ "พับเก็บ" ใช้คืนค่าตอนดึงหน้าต่างขึ้นมา
)

// หาแท็บของโปรแกรม (http://localhost:<port>/) ในเบราว์เซอร์ที่เราเปิดไว้
func appTarget(w *cdpConn, port int) (string, error) {
	var r struct {
		TargetInfos []struct {
			TargetID string `json:"targetId"`
			Type     string `json:"type"`
			URL      string `json:"url"`
		} `json:"targetInfos"`
	}
	if err := w.call("Target.getTargets", nil, &r); err != nil {
		return "", err
	}
	for _, t := range r.TargetInfos {
		if t.Type != "page" {
			continue
		}
		for _, h := range []string{"localhost", "127.0.0.1"} {
			if strings.HasPrefix(t.URL, fmt.Sprintf("http://%s:%d", h, port)) {
				return t.TargetID, nil
			}
		}
	}
	return "", errNoWindow
}

// windowDo สั่งหน้าต่าง action: state | minimize | normal | maximize | fullscreen | small | front | close | quit
// คืนค่าสถานะหน้าต่างหลังทำเสร็จ: normal | minimized | maximized | fullscreen | closed
func windowDo(port int, action string, small *winBounds) (string, error) {
	winMu.Lock()
	defer winMu.Unlock()
	w, err := cdpDial()
	if err != nil {
		return "", err
	}
	defer w.close()
	if action == "quit" { // ปิดเบราว์เซอร์ทั้งชุด (เป็นโปรไฟล์ของโปรแกรมนี้เท่านั้น)
		_ = w.call("Browser.close", nil, nil)
		return "closed", nil
	}
	tid, err := appTarget(w, port)
	if err != nil {
		return "", err
	}
	if action == "close" {
		if err := w.call("Target.closeTarget", map[string]any{"targetId": tid}, nil); err != nil {
			return "", err
		}
		return "closed", nil
	}
	var win struct {
		WindowID int       `json:"windowId"`
		Bounds   winBounds `json:"bounds"`
	}
	get := func() (string, error) {
		if err := w.call("Browser.getWindowForTarget", map[string]any{"targetId": tid}, &win); err != nil {
			return "", err
		}
		return win.Bounds.WindowState, nil
	}
	set := func(b map[string]any) error {
		return w.call("Browser.setWindowBounds", map[string]any{"windowId": win.WindowID, "bounds": b}, nil)
	}
	waitFor := func(want string) string { // หน้าต่างเปลี่ยนสถานะไม่ทันที รอสั้นๆ
		st, _ := get()
		for i := 0; i < 15 && st != want; i++ {
			time.Sleep(80 * time.Millisecond)
			st, _ = get()
		}
		return st
	}
	cur, err := get()
	if err != nil {
		return "", err
	}

	target := ""
	switch action {
	case "state":
	case "minimize":
		target = "minimized"
	case "maximize":
		target = "maximized"
	case "normal", "fullscreen":
		target = action
	case "small":
		target = "normal"
	case "front":
		if cur == "minimized" {
			target = winLast
		}
	default:
		return "", errors.New("unknown window action")
	}
	if target != "" && target != cur {
		if cur != "normal" { // Chrome เปลี่ยนจากสถานะพิเศษไปสถานะพิเศษอื่นตรงๆ ไม่ได้ ต้องผ่าน normal ก่อน
			_ = set(map[string]any{"windowState": "normal"})
			cur = waitFor("normal")
		}
		if target != "normal" {
			if err := set(map[string]any{"windowState": target}); err != nil {
				return cur, err
			}
			cur = waitFor(target)
		}
	}
	if action == "small" && small != nil && small.Width >= 320 && small.Height >= 320 {
		_ = set(map[string]any{"left": small.Left, "top": small.Top, "width": small.Width, "height": small.Height})
		cur, _ = get()
	}
	if action == "front" {
		_ = w.call("Target.activateTarget", map[string]any{"targetId": tid}, nil)
	}
	if cur != "minimized" && cur != "" {
		winLast = cur
	}
	return cur, nil
}

// รอหน้าต่างเปิดเสร็จแล้วตั้งให้เต็มจอ (ใช้กับโหมดหุ่นยนต์ เผื่อเบราว์เซอร์ไม่รับ --start-fullscreen)
func ensureFullscreen(port int) {
	for i := 0; i < 60; i++ {
		time.Sleep(250 * time.Millisecond)
		st, err := windowDo(port, "state", nil)
		if err != nil {
			continue
		}
		if st != "fullscreen" {
			_, _ = windowDo(port, "fullscreen", nil)
		}
		return
	}
}
