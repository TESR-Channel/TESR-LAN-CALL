package main

// สถานะการเชื่อมต่อของมือถือ/iPad ที่แสดงบนหน้าจอคอม
// คอมบอกได้ทันทีว่ามือถือติดต่อมาถึงหรือยัง และติดอยู่ขั้นไหน:
//   ไม่มีอะไรเข้ามาเลย  -> มือถือมาไม่ถึงคอม (คนละ Wi-Fi / ไฟร์วอลล์ / คอมหลับ)
//   ติดที่ใบรับรอง     -> มาถึงแล้ว แต่ยังไม่ได้กดยอมรับคำเตือน (หรือยังไม่ติดตั้งใบรับรอง)
//   ใช้งานอยู่         -> เรียบร้อย

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"os/exec"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

type visit struct {
	IP    string
	Kind  string
	Stage string // start | hello | cert | page | online | left
	At    time.Time
}

type diagState struct {
	mu     sync.Mutex
	visits map[string]*visit
}

var diag = &diagState{visits: map[string]*visit{}}

func uaKind(ua string) string {
	switch {
	case strings.Contains(ua, "iPad"):
		return "iPad"
	case strings.Contains(ua, "iPhone"):
		return "iPhone"
	case strings.Contains(ua, "Android"):
		if strings.Contains(ua, "Mobile") {
			return "มือถือ Android"
		}
		return "แท็บเล็ต Android"
	case strings.Contains(ua, "Macintosh") && strings.Contains(ua, "Safari") && !strings.Contains(ua, "Chrome"):
		return "iPad หรือ Mac" // iPad รุ่นใหม่ส่งตัวเองเป็น Mac
	}
	return ""
}

var (
	ownMu  sync.Mutex
	ownIPs map[string]bool
	ownAt  time.Time
)

// IP ของคอมเครื่องนี้เอง (ไม่นับเป็นมือถือ)
func isOwnIP(ip string) bool {
	ownMu.Lock()
	defer ownMu.Unlock()
	if ownIPs == nil || time.Since(ownAt) > 10*time.Second {
		ownIPs, ownAt = map[string]bool{}, time.Now()
		addrs, _ := net.InterfaceAddrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok {
				ownIPs[normIP(n.IP)] = true
			}
		}
	}
	return ownIPs[ip]
}

func (d *diagState) mark(ip, stage, ua string) {
	if ip == "" || net.ParseIP(ip) == nil || net.ParseIP(ip).IsLoopback() || isOwnIP(ip) {
		return
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	v := d.visits[ip]
	if v == nil {
		v = &visit{IP: ip}
		d.visits[ip] = v
	}
	if k := uaKind(ua); k != "" {
		v.Kind = k
	}
	// ไม่ให้สถานะย้อนกลับ: ขั้นที่ไปไกลกว่าแล้วชนะ ยกเว้นเหตุการณ์ที่บอกปัญหาจริง
	cur, recent := v.Stage, time.Since(v.At) < 30*time.Second
	switch stage {
	case "left":
		if cur != "online" {
			return
		}
	case "hello":
		if cur != "" && cur != "start" && cur != "left" {
			v.At = time.Now()
			return
		}
	case "cert":
		if cur == "online" || (cur == "page" && recent) {
			return
		}
	case "page", "start", "profile":
		if cur == "online" {
			return
		}
	}
	v.Stage, v.At = stage, time.Now()
	if len(d.visits) > 64 { // เก็บแค่ล่าสุด
		var old *visit
		for _, x := range d.visits {
			if old == nil || x.At.Before(old.At) {
				old = x
			}
		}
		delete(d.visits, old.IP)
	}
}

func (d *diagState) list() []map[string]any {
	d.mu.Lock()
	defer d.mu.Unlock()
	out := []map[string]any{}
	for _, v := range d.visits {
		age := time.Since(v.At)
		if age > 30*time.Minute {
			continue
		}
		out = append(out, map[string]any{"ip": v.IP, "kind": v.Kind, "stage": v.Stage, "ago": int(age.Seconds()), "at": v.At.UnixMilli()})
	}
	sort.Slice(out, func(i, j int) bool { return out[i]["at"].(int64) > out[j]["at"].(int64) })
	return out
}

// อ่านข้อความ error ของ HTTPS server: "http: TLS handshake error from 192.168.1.50:51234: remote error: tls: unknown certificate"
var tlsErrRe = regexp.MustCompile(`TLS handshake error from \[?([0-9a-fA-F.:]+?)\]?:\d+: (.*)`)

type tlsErrLog struct{}

func (tlsErrLog) Write(p []byte) (int, error) {
	if m := tlsErrRe.FindSubmatch(p); m != nil {
		ip := string(m[1])
		if x := net.ParseIP(ip); x != nil && x.To4() != nil {
			ip = x.To4().String()
		}
		msg := strings.ToLower(string(m[2]))
		// มือถือปิดการเชื่อมต่อเพราะไม่ยอมรับใบรับรอง (Safari ขึ้นหน้าเตือน) หรือยกเลิกกลางคัน
		if strings.Contains(msg, "certificate") || strings.Contains(msg, "eof") || strings.Contains(msg, "reset") || strings.Contains(msg, "bad") {
			diag.mark(ip, "cert", "")
		}
	}
	return len(p), nil
}

// ======================================================================== ไฟร์วอลล์

type fwState struct {
	Checked  bool     `json:"checked"`
	Problems []string `json:"problems"`
	Notes    []string `json:"notes"`
}

var (
	fwMu   sync.Mutex
	fwLast = fwState{Problems: []string{}, Notes: []string{}}
	fwAt   time.Time
)

// แปลงค่า JSON ที่อาจเป็น string เดี่ยว หรือ array ของ string (PowerShell 5 ทำแบบนี้)
type strList []string

func (s *strList) UnmarshalJSON(b []byte) error {
	var one string
	if json.Unmarshal(b, &one) == nil {
		if one != "" {
			*s = []string{one}
		}
		return nil
	}
	var many []string
	if err := json.Unmarshal(b, &many); err != nil {
		*s = nil
		return nil
	}
	*s = many
	return nil
}

const winFwCheck = `$ErrorActionPreference='SilentlyContinue'
$exe='%s'
$o=[ordered]@{on=@();block=0;allow=0;ports=0;third=@();cats=@()}
$o.on=@(Get-NetFirewallProfile | Where-Object { "$($_.Enabled)" -eq 'True' } | ForEach-Object { "$($_.Name)" })
$r=@(Get-NetFirewallApplicationFilter | Where-Object { $_.Program -and ($_.Program -ieq $exe -or $_.Program -like '*TESR-LAN-Call*') } | Get-NetFirewallRule | Where-Object { "$($_.Enabled)" -eq 'True' -and "$($_.Direction)" -eq 'Inbound' })
$o.block=@($r | Where-Object { "$($_.Action)" -eq 'Block' }).Count
$o.allow=@($r | Where-Object { "$($_.Action)" -eq 'Allow' }).Count
$o.ports=@(Get-NetFirewallRule -DisplayName 'TESR LAN Call (TCP)' | Where-Object { "$($_.Enabled)" -eq 'True' -and "$($_.Action)" -eq 'Allow' }).Count
$o.third=@(Get-CimInstance -Namespace root/SecurityCenter2 -ClassName FirewallProduct | ForEach-Object { "$($_.displayName)" })
$o.cats=@(Get-NetConnectionProfile | ForEach-Object { "$($_.InterfaceAlias)=$($_.NetworkCategory)" })
$o | ConvertTo-Json -Compress`

func checkFirewall() fwState {
	st := fwState{Checked: runtime.GOOS == "windows" || runtime.GOOS == "darwin", Problems: []string{}, Notes: []string{}}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	defer cancel()
	run := func(name string, args ...string) (string, error) {
		cmd := exec.CommandContext(ctx, name, args...)
		hide(cmd)
		out, err := cmd.Output()
		return string(out), err
	}
	switch runtime.GOOS {
	case "windows":
		ps := fmt.Sprintf(winFwCheck, strings.ReplaceAll(selfExe(), "'", "''"))
		out, err := run("powershell", "-NoProfile", "-NonInteractive", "-ExecutionPolicy", "Bypass", "-Command", ps)
		i := strings.Index(out, "{")
		if err != nil || i < 0 {
			st.Checked = false
			return st
		}
		var r struct {
			On    strList `json:"on"`
			Block int     `json:"block"`
			Allow int     `json:"allow"`
			Ports int     `json:"ports"`
			Third strList `json:"third"`
			Cats  strList `json:"cats"`
		}
		if json.Unmarshal([]byte(out[i:]), &r) != nil {
			st.Checked = false
			return st
		}
		if len(r.On) > 0 {
			if r.Block > 0 {
				st.Problems = append(st.Problems, "ไฟร์วอลล์ของ Windows กำลังบล็อกโปรแกรมนี้ มือถือ/iPad จึงเข้าไม่ได้")
			} else if r.Allow == 0 && r.Ports == 0 {
				st.Problems = append(st.Problems, "ยังไม่ได้เปิดไฟร์วอลล์ของ Windows ให้มือถือ/iPad เข้าคอมนี้")
			}
		}
		for _, t := range r.Third {
			if t = strings.TrimSpace(t); t != "" {
				st.Notes = append(st.Notes, "คอมนี้มีไฟร์วอลล์ของ "+t+" ถ้ามือถือยังเข้าไม่ได้ ให้เปิดโปรแกรมนั้นแล้วอนุญาต TESR LAN Call (หรือพอร์ต 47800, 47843)")
			}
		}
	case "darwin":
		fw := "/usr/libexec/ApplicationFirewall/socketfilterfw"
		g, _ := run(fw, "--getglobalstate")
		if !strings.Contains(strings.ToLower(g), "enabled") {
			return st
		}
		if b, _ := run(fw, "--getblockall"); strings.Contains(strings.ToLower(b), "block all") && !strings.Contains(strings.ToLower(b), "disabled") {
			st.Problems = append(st.Problems, "ไฟร์วอลล์ของ Mac ตั้งให้บล็อกการเชื่อมต่อขาเข้าทั้งหมด มือถือ/iPad จึงเข้าไม่ได้")
			break
		}
		a, _ := run(fw, "--getappblocked", selfExe())
		la := strings.ToLower(a)
		if strings.Contains(la, "blocked") && !strings.Contains(la, "permitted") {
			st.Problems = append(st.Problems, "ไฟร์วอลล์ของ Mac กำลังบล็อกโปรแกรมนี้ มือถือ/iPad จึงเข้าไม่ได้")
		} else if !strings.Contains(la, "permitted") {
			st.Notes = append(st.Notes, "ไฟร์วอลล์ของ Mac เปิดอยู่ ถ้ามือถือเข้าไม่ได้ ให้กดปุ่มซ่อมการเชื่อมต่อด้านล่าง")
		}
	}
	return st
}

// ผลตรวจล่าสุด (ตรวจใหม่เบื้องหลังทุก 3 นาที หรือเมื่อ force)
func firewallStatus(force bool) fwState {
	fwMu.Lock()
	st, stale := fwLast, time.Since(fwAt) > 3*time.Minute
	if force || (stale && !fwAt.IsZero()) || fwAt.IsZero() {
		fwAt = time.Now()
		if force {
			fwMu.Unlock()
			st = checkFirewall()
			fwMu.Lock()
			fwLast = st
		} else {
			go func() {
				r := checkFirewall()
				fwMu.Lock()
				fwLast = r
				fwMu.Unlock()
			}()
		}
	}
	fwMu.Unlock()
	return st
}
