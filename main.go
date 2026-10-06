// TESR LAN Call - โทรวิดีโอในวง LAN สำหรับ Windows / macOS / Linux / Raspberry Pi / มือถือ / iPad
//
// เครื่องคอม: หน้าโปรแกรมเปิดเป็นหน้าต่างแอปที่ http://localhost:47800
// มือถือ/iPad: เปิด https://<IP ของคอม>:47843 (สแกน QR ในหน้าโปรแกรม) ใช้ผ่านคอมเครื่องนั้น
// ค้นหาเครื่องด้วย UDP broadcast, ส่งสัญญาณโทรผ่าน HTTP, ภาพ/เสียงวิ่งตรงด้วย WebRTC
package main

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"embed"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

const (
	appName      = "TESR LAN Call"
	version      = "2.2.0"
	peerTimeout  = 8 * time.Second
	mobileGrace  = 45 * time.Second
	pendingTTL   = 30 * time.Second
	defaultPort  = 47800
	defaultHTTPS = 47843
	defaultDisc  = 47801
)

//go:embed web/index.html
var pageHTML []byte

//go:embed assets
var assetsFS embed.FS

var dataDir string

// ======================================================================== config

type Config struct {
	ID         string   `json:"id"`
	Name       string   `json:"name"`
	AutoAnswer bool     `json:"auto_answer"`
	RobotMode  bool     `json:"robot_mode"`
	ManualIPs  []string `json:"manual_ips"`
}

func loadConfig() Config {
	var c Config
	if b, err := os.ReadFile(filepath.Join(dataDir, "config.json")); err == nil {
		_ = json.Unmarshal(b, &c)
	}
	if c.ID == "" {
		b := make([]byte, 6)
		_, _ = rand.Read(b)
		c.ID = hex.EncodeToString(b)
	}
	if c.Name == "" {
		c.Name, _ = os.Hostname()
		c.Name = strings.TrimSuffix(c.Name, ".local")
	}
	if c.ManualIPs == nil {
		c.ManualIPs = []string{}
	}
	return c
}

func saveConfig(c Config) {
	b, _ := json.MarshalIndent(c, "", "  ")
	if err := os.WriteFile(filepath.Join(dataDir, "config.json"), b, 0o644); err != nil {
		log.Println("[config] save failed:", err)
	}
}

func osName() string {
	switch runtime.GOOS {
	case "windows":
		return "Windows"
	case "darwin":
		return "macOS"
	case "linux":
		if runtime.GOARCH == "arm64" || runtime.GOARCH == "arm" {
			return "Linux ARM"
		}
		return "Linux"
	}
	return runtime.GOOS
}

func primaryIP() string {
	c, err := net.Dial("udp4", "10.255.255.255:1")
	if err != nil {
		return "127.0.0.1"
	}
	defer c.Close()
	return c.LocalAddr().(*net.UDPAddr).IP.String()
}

// การ์ดเครือข่ายเสมือน (VPN, WSL, Docker, VM) ที่มือถือเข้าถึงไม่ได้
var virtualIf = regexp.MustCompile(`(?i)vethernet|virtualbox|vmware|vmnet|hyper-v|docker|podman|virbr|lxd|cni|flannel|veth|wsl|tailscale|zerotier|^zt|^utun|^tun|^tap|^wg|bluetooth|^llw|^awdl|^anpi|^bridge|^br-`)

// IP ในวง LAN ที่มือถือใช้ได้ เรียงจากที่น่าจะถูกที่สุด (การ์ดที่ออกเน็ตก่อน)
func lanIPs() []string {
	primary := primaryIP()
	var good []string
	ifs, _ := net.Interfaces()
	for _, ifc := range ifs {
		if ifc.Flags&net.FlagUp == 0 || ifc.Flags&net.FlagLoopback != 0 || virtualIf.MatchString(ifc.Name) {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			if n, ok := a.(*net.IPNet); ok {
				if ip := n.IP.To4(); ip != nil && ip.IsPrivate() {
					good = append(good, ip.String())
				}
			}
		}
	}
	out := []string{}
	for _, g := range good {
		if g == primary {
			out = append(out, g)
		}
	}
	for _, g := range good {
		if g != primary {
			out = append(out, g)
		}
	}
	if len(out) == 0 {
		out = append(out, primary)
	}
	return out
}

func lanIP() string { return lanIPs()[0] }

// ======================================================================== state

type Endpoint struct {
	ID, Name, Kind, Dev string
	clients             map[chan []byte]struct{}
	Busy                bool
	Left                time.Time
	pending             []byte // สายที่เข้ามาตอนหน้าต่างยังไม่เปิด
	pendingFrom         string
	pendingAt           time.Time
}

func newEndpoint(id, name, kind, dev string) *Endpoint {
	return &Endpoint{ID: id, Name: name, Kind: kind, Dev: dev, clients: map[chan []byte]struct{}{}, Left: time.Now()}
}

type Peer struct {
	ID, Name, IP, OS, Kind string
	Port                   int
	Busy, UI               bool
	Last                   time.Time
}

type State struct {
	mu                sync.Mutex
	cfg               Config
	port, https, disc int
	httpsOK           bool
	httpsErr          string
	local             *Endpoint
	endpoints         map[string]*Endpoint
	peers             map[string]*Peer
	canWake           bool
	kiosk             bool
	lastWake          time.Time
	kick              chan struct{}
}

func (s *State) info(ep *Endpoint) map[string]any {
	return map[string]any{"app": "tesr-lan-call", "v": 2, "id": ep.ID, "name": ep.Name, "port": s.port,
		"os": ep.Dev, "kind": ep.Kind, "busy": ep.Busy, "ui": len(ep.clients) > 0 || (ep == s.local && s.canWake)}
}

func (s *State) mobileURLs() []string {
	out := []string{}
	if s.httpsOK {
		for _, ip := range lanIPs() {
			out = append(out, fmt.Sprintf("https://%s:%d/", ip, s.https))
		}
	}
	return out
}

func (s *State) mobileURL() string {
	if u := s.mobileURLs(); len(u) > 0 {
		return u[0]
	}
	return ""
}

func (s *State) settingsFor(ep *Endpoint) map[string]any {
	m := map[string]any{"id": ep.ID, "name": ep.Name, "kind": ep.Kind, "version": version,
		"host": s.local.Name, "ip": lanIP()}
	if ep == s.local {
		m["auto_answer"] = s.cfg.AutoAnswer
		m["robot_mode"] = s.cfg.RobotMode
		m["autostart"] = autostartEnabled()
		m["port"] = s.port
		m["mobile_url"] = s.mobileURL()
		m["mobile_urls"] = s.mobileURLs()
		m["os"] = runtime.GOOS
		m["qr"] = s.httpsOK
		m["https_error"] = s.httpsErr
	}
	return m
}

func send(ep *Endpoint, v any) {
	b, err := json.Marshal(v)
	if err != nil {
		return
	}
	for ch := range ep.clients {
		select {
		case ch <- b:
		default:
		}
	}
}

// เรียกขณะถือ lock
func (s *State) pushPeersLocked() {
	for _, ep := range s.endpoints {
		if len(ep.clients) > 0 {
			send(ep, map[string]any{"type": "peers", "peers": s.peerListLocked(ep)})
		}
	}
}

func (s *State) pushPeers() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pushPeersLocked()
}

func (s *State) peerListLocked(me *Endpoint) []map[string]any {
	now, ip := time.Now(), lanIP()
	out := []map[string]any{}
	for _, p := range s.peers {
		out = append(out, map[string]any{"id": p.ID, "name": p.Name, "ip": p.IP, "port": p.Port, "os": p.OS,
			"kind": p.Kind, "busy": p.Busy, "online": now.Sub(p.Last) < peerTimeout && p.UI,
			"here": false, "ago": int(now.Sub(p.Last).Seconds())})
	}
	for _, ep := range s.endpoints {
		if ep == me {
			continue
		}
		on := len(ep.clients) > 0 || (ep == s.local && s.canWake)
		ago := 0
		if !on {
			ago = int(now.Sub(ep.Left).Seconds())
		}
		out = append(out, map[string]any{"id": ep.ID, "name": ep.Name, "ip": ip, "port": s.port, "os": ep.Dev,
			"kind": ep.Kind, "busy": ep.Busy, "online": on, "here": true, "ago": ago})
	}
	sort.Slice(out, func(i, j int) bool {
		oi, oj := out[i]["online"].(bool), out[j]["online"].(bool)
		if oi != oj {
			return oi
		}
		return strings.ToLower(out[i]["name"].(string)) < strings.ToLower(out[j]["name"].(string))
	})
	return out
}

func str(m map[string]any, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}

func trunc(v string, n int) string {
	r := []rune(v)
	if len(r) > n {
		return string(r[:n])
	}
	return v
}

// รับข้อมูลเครื่องอื่น คืนค่า true ถ้าเพิ่งเจอครั้งแรก
func (s *State) upsert(info map[string]any, ip string) bool {
	if str(info, "app") != "tesr-lan-call" {
		return false
	}
	id := str(info, "id")
	s.mu.Lock()
	defer s.mu.Unlock()
	if id == "" || s.endpoints[id] != nil {
		return false
	}
	port := defaultPort
	if f, ok := info["port"].(float64); ok {
		port = int(f)
	}
	kind := "desktop"
	if str(info, "kind") == "mobile" {
		kind = "mobile"
	}
	ui := true
	if b, ok := info["ui"].(bool); ok {
		ui = b
	}
	busy, _ := info["busy"].(bool)
	n := &Peer{ID: id, Name: trunc(str(info, "name"), 60), IP: ip, Port: port, OS: trunc(str(info, "os"), 30),
		Kind: kind, Busy: busy, UI: ui, Last: time.Now()}
	old := s.peers[id]
	changed := old == nil || time.Since(old.Last) >= peerTimeout || old.Name != n.Name || old.IP != n.IP ||
		old.Port != n.Port || old.Busy != n.Busy || old.UI != n.UI
	s.peers[id] = n
	if changed {
		s.pushPeersLocked()
	}
	return old == nil
}

// ส่งข้อความถึง endpoint ในเครื่องนี้ ถ้าหน้าต่างปิดอยู่และเป็นสายเรียกเข้า จะเปิดหน้าต่างให้เอง
func (s *State) deliver(target string, ev map[string]any) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	ep := s.endpoints[target]
	if ep == nil {
		return false
	}
	if len(ep.clients) > 0 {
		send(ep, ev)
		return true
	}
	if ep != s.local || !s.canWake {
		return false
	}
	msg, _ := ev["msg"].(map[string]any)
	t := str(msg, "t")
	from := str(ev, "from")
	switch t {
	case "ring":
		ep.pending, _ = json.Marshal(ev)
		ep.pendingFrom, ep.pendingAt = from, time.Now()
		if time.Since(s.lastWake) > 8*time.Second {
			s.lastWake = time.Now()
			go openWindow(fmt.Sprintf("http://localhost:%d/", s.port), s.kiosk)
		}
		return true
	case "cancel", "hangup":
		if ep.pendingFrom == from {
			ep.pending = nil
		}
		return true
	}
	return false
}

// ======================================================================== discovery

func (s *State) announcer() {
	conn, err := net.ListenUDP("udp4", nil)
	if err != nil {
		log.Println("[discovery] announcer:", err)
		return
	}
	for {
		targets := map[string]bool{"255.255.255.255": true}
		for _, ip := range lanIPs() {
			if ip != "127.0.0.1" {
				targets[ip[:strings.LastIndex(ip, ".")]+".255"] = true
			}
		}
		s.mu.Lock()
		for _, m := range s.cfg.ManualIPs {
			targets[strings.Split(m, ":")[0]] = true
		}
		var packets [][]byte
		for _, ep := range s.endpoints {
			if ep.Kind == "mobile" && len(ep.clients) == 0 {
				continue
			}
			b, _ := json.Marshal(s.info(ep))
			packets = append(packets, b)
		}
		s.mu.Unlock()
		for t := range targets {
			addr := &net.UDPAddr{IP: net.ParseIP(t), Port: s.disc}
			for _, p := range packets {
				_, _ = conn.WriteToUDP(p, addr)
			}
		}
		select {
		case <-time.After(2 * time.Second):
		case <-s.kick:
		}
	}
}

// ประกาศสถานะใหม่ทันที (เช่น ว่าง/ไม่ว่าง)
func (s *State) announceNow() {
	select {
	case s.kick <- struct{}{}:
	default:
	}
}

func (s *State) listener() {
	conn, err := net.ListenUDP("udp4", &net.UDPAddr{Port: s.disc})
	if err != nil {
		log.Println("[discovery] listen:", err)
		return
	}
	buf := make([]byte, 4096)
	for {
		n, addr, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}
		var info map[string]any
		if json.Unmarshal(buf[:n], &info) != nil {
			continue
		}
		if s.upsert(info, addr.IP.String()) {
			s.mu.Lock()
			b, _ := json.Marshal(s.info(s.local))
			s.mu.Unlock()
			_, _ = conn.WriteToUDP(b, &net.UDPAddr{IP: addr.IP, Port: s.disc})
		}
	}
}

func (s *State) watcher() {
	last := ""
	for {
		time.Sleep(2 * time.Second)
		s.mu.Lock()
		for id, ep := range s.endpoints {
			if ep.Kind == "mobile" && len(ep.clients) == 0 && time.Since(ep.Left) > mobileGrace {
				delete(s.endpoints, id)
			}
		}
		var keys []string
		for _, p := range s.peerListLocked(nil) {
			keys = append(keys, fmt.Sprint(p["id"], p["online"]))
		}
		sort.Strings(keys)
		snap := strings.Join(keys, ",")
		if snap != last {
			s.pushPeersLocked()
			last = snap
		}
		s.mu.Unlock()
	}
}

var httpClient = &http.Client{Timeout: 3 * time.Second}

func httpJSON(url string, body any) (map[string]any, int, error) {
	var resp *http.Response
	var err error
	if body == nil {
		resp, err = httpClient.Get(url)
	} else {
		b, _ := json.Marshal(body)
		resp, err = httpClient.Post(url, "application/json", strings.NewReader(string(b)))
	}
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(io.LimitReader(resp.Body, 1<<20)).Decode(&out)
	return out, resp.StatusCode, nil
}

// ======================================================================== https cert

func ensureCert(ips []string) (tls.Certificate, error) {
	ip := strings.Join(ips, ",")
	certP, keyP, metaP := filepath.Join(dataDir, "cert.pem"), filepath.Join(dataDir, "key.pem"), filepath.Join(dataDir, "cert-ip.txt")
	if m, err := os.ReadFile(metaP); err == nil && strings.TrimSpace(string(m)) == ip {
		if c, err := tls.LoadX509KeyPair(certP, keyP); err == nil {
			return c, nil
		}
	}
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return tls.Certificate{}, err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 62))
	tpl := x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: appName, Organization: []string{"TESR Co., Ltd."}},
		NotBefore:    time.Now().Add(-24 * time.Hour),
		NotAfter:     time.Now().Add(800 * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
	}
	for _, a := range ips {
		if p := net.ParseIP(a); p != nil {
			tpl.IPAddresses = append(tpl.IPAddresses, p)
		}
	}
	der, err := x509.CreateCertificate(rand.Reader, &tpl, &tpl, &key.PublicKey, key)
	if err != nil {
		return tls.Certificate{}, err
	}
	kb, _ := x509.MarshalECPrivateKey(key)
	_ = os.WriteFile(certP, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o644)
	_ = os.WriteFile(keyP, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: kb}), 0o600)
	_ = os.WriteFile(metaP, []byte(ip), 0o644)
	return tls.LoadX509KeyPair(certP, keyP)
}

// ======================================================================== http

var idRe = regexp.MustCompile(`^[a-f0-9]{8,32}$`)

func isLocal(r *http.Request) bool {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func remoteIP(r *http.Request) string {
	host, _, _ := net.SplitHostPort(r.RemoteAddr)
	if ip := net.ParseIP(host); ip != nil && ip.To4() != nil {
		return ip.To4().String()
	}
	return host
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func readBody(r *http.Request) map[string]any {
	var m map[string]any
	_ = json.NewDecoder(io.LimitReader(r.Body, 200_000)).Decode(&m)
	if m == nil {
		m = map[string]any{}
	}
	return m
}

// local page -> เครื่องนี้, มือถือ -> endpoint ของตัวเองเท่านั้น
func (s *State) epFor(r *http.Request, id string) *Endpoint {
	s.mu.Lock()
	defer s.mu.Unlock()
	if isLocal(r) && (id == "" || id == s.local.ID) {
		return s.local
	}
	if ep := s.endpoints[id]; ep != nil && ep.Kind == "mobile" {
		return ep
	}
	return nil
}

func (s *State) routes() http.Handler {
	mux := http.NewServeMux()
	sub, _ := fs.Sub(assetsFS, "assets")
	files := http.StripPrefix("/assets/", http.FileServer(http.FS(sub)))
	mux.HandleFunc("/assets/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "max-age=86400")
		files.ServeHTTP(w, r)
	})
	mux.HandleFunc("/manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json")
		_ = json.NewEncoder(w).Encode(map[string]any{"name": appName, "short_name": "TESR Call", "start_url": "/",
			"display": "standalone", "background_color": "#0A0A0A", "theme_color": "#0A0A0A", "lang": "th",
			"icons": []map[string]string{
				{"src": "/assets/icon-192.png", "sizes": "192x192", "type": "image/png"},
				{"src": "/assets/icon-512.png", "sizes": "512x512", "type": "image/png"}}})
	})
	mux.HandleFunc("/peer/info", func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		i := s.info(s.local)
		s.mu.Unlock()
		writeJSON(w, 200, i)
	})
	mux.HandleFunc("/peer/msg", s.peerMsg)
	mux.HandleFunc("/api/events", s.events)
	mux.HandleFunc("/api/qr.png", func(w http.ResponseWriter, r *http.Request) {
		u := s.mobileURL()
		if !isLocal(r) || u == "" {
			http.NotFound(w, r)
			return
		}
		png, err := qrcode.Encode(u, qrcode.Medium, 320)
		if err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		w.Header().Set("Content-Type", "image/png")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(png)
	})
	mux.HandleFunc("/api/", s.api)
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/index.html" {
			http.NotFound(w, r)
			return
		}
		if !isLocal(r) && r.TLS == nil { // มือถือเข้าผ่าน http -> ส่งไป https (กล้องต้องใช้ https)
			host, _, err := net.SplitHostPort(r.Host)
			if err != nil {
				host = r.Host
			}
			if s.httpsOK {
				http.Redirect(w, r, fmt.Sprintf("https://%s:%d/", host, s.https), http.StatusFound)
				return
			}
			http.Error(w, "เครื่องนี้ยังเปิดโหมดมือถือไม่ได้", 503)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.Header().Set("Cache-Control", "no-store")
		_, _ = w.Write(pageHTML)
	})
	return mux
}

func (s *State) events(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	var ep *Endpoint
	if q.Get("kind") == "mobile" {
		id := q.Get("ep")
		s.mu.Lock()
		bad := !idRe.MatchString(id) || id == s.local.ID || s.peers[id] != nil
		if !bad {
			name := strings.TrimSpace(trunc(q.Get("name"), 60))
			if name == "" {
				name = "มือถือ"
			}
			ep = s.endpoints[id]
			if ep == nil {
				ep = newEndpoint(id, name, "mobile", trunc(q.Get("dev"), 20))
				s.endpoints[id] = ep
			}
			ep.Name = name
		}
		s.mu.Unlock()
		if bad {
			writeJSON(w, 400, map[string]any{"error": "bad id"})
			return
		}
	} else if isLocal(r) {
		ep = s.local
	} else {
		writeJSON(w, 403, map[string]any{"error": "local only"})
		return
	}
	fl, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", 500)
		return
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	ch := make(chan []byte, 64)

	s.mu.Lock()
	ep.clients[ch] = struct{}{}
	hello, _ := json.Marshal(map[string]any{"type": "hello", "settings": s.settingsFor(ep), "peers": s.peerListLocked(ep)})
	var pending []byte
	if ep.pending != nil && time.Since(ep.pendingAt) < pendingTTL {
		pending = ep.pending
	}
	ep.pending = nil
	s.pushPeersLocked()
	s.mu.Unlock()

	fmt.Fprintf(w, "data: %s\n\n", hello)
	if pending != nil {
		fmt.Fprintf(w, "data: %s\n\n", pending)
	}
	fl.Flush()
	ping := time.NewTicker(15 * time.Second)
	defer ping.Stop()
	defer func() {
		s.mu.Lock()
		delete(ep.clients, ch)
		if len(ep.clients) == 0 {
			ep.Busy = false
			ep.Left = time.Now()
		}
		s.pushPeersLocked()
		s.mu.Unlock()
	}()
	for {
		select {
		case b := <-ch:
			if _, err := fmt.Fprintf(w, "data: %s\n\n", b); err != nil {
				return
			}
			fl.Flush()
			if strings.Contains(string(b), `"type":"quit"`) {
				return
			}
		case <-ping.C:
			if _, err := io.WriteString(w, ": ping\n\n"); err != nil {
				return
			}
			fl.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func (s *State) peerMsg(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	d := readBody(r)
	info, _ := d["info"].(map[string]any)
	if info == nil {
		info = map[string]any{}
	}
	s.upsert(info, remoteIP(r))
	target := str(d, "to")
	if target == "" {
		target = s.local.ID
	}
	ev := map[string]any{"type": "msg", "from": str(info, "id"), "from_name": str(info, "name"), "msg": d["msg"]}
	if !s.deliver(target, ev) {
		writeJSON(w, 503, map[string]any{"ok": false, "error": "ปลายทางไม่ได้เปิดโปรแกรมไว้"})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *State) api(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.NotFound(w, r)
		return
	}
	d := readBody(r)
	ep := s.epFor(r, str(d, "ep"))
	if ep == nil {
		writeJSON(w, 403, map[string]any{"ok": false, "error": "ไม่มีสิทธิ์"})
		return
	}
	switch r.URL.Path {
	case "/api/send":
		s.apiSend(w, ep, d)
	case "/api/status":
		s.mu.Lock()
		ep.Busy, _ = d["busy"].(bool)
		s.pushPeersLocked()
		s.mu.Unlock()
		s.announceNow()
		writeJSON(w, 200, map[string]any{"ok": true})
	case "/api/settings":
		s.apiSettings(w, ep, d)
	case "/api/add":
		if ep != s.local {
			writeJSON(w, 403, map[string]any{"ok": false})
			return
		}
		s.apiAdd(w, d)
	case "/api/fix-network":
		if ep != s.local {
			writeJSON(w, 403, map[string]any{"ok": false})
			return
		}
		msg, err := fixNetwork(s.port, s.https, s.disc)
		if err != nil {
			writeJSON(w, 200, map[string]any{"ok": false, "error": err.Error()})
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "msg": msg})
	case "/api/quit":
		if ep != s.local {
			writeJSON(w, 403, map[string]any{"ok": false})
			return
		}
		s.mu.Lock()
		for _, e := range s.endpoints {
			send(e, map[string]any{"type": "quit"})
		}
		s.mu.Unlock()
		time.AfterFunc(600*time.Millisecond, func() { os.Exit(0) })
		writeJSON(w, 200, map[string]any{"ok": true})
	default:
		http.NotFound(w, r)
	}
}

func (s *State) apiSend(w http.ResponseWriter, ep *Endpoint, d map[string]any) {
	to := str(d, "to")
	s.mu.Lock()
	ev := map[string]any{"type": "msg", "from": ep.ID, "from_name": ep.Name, "msg": d["msg"]}
	info := s.info(ep)
	_, isHere := s.endpoints[to]
	p := s.peers[to]
	var url string
	if p != nil {
		url = fmt.Sprintf("http://%s:%d/peer/msg", p.IP, p.Port)
	}
	s.mu.Unlock()
	if isHere {
		if s.deliver(to, ev) {
			writeJSON(w, 200, map[string]any{"ok": true})
		} else {
			writeJSON(w, 200, map[string]any{"ok": false, "error": "ปลายทางไม่ได้เปิดโปรแกรมไว้"})
		}
		return
	}
	if p == nil {
		writeJSON(w, 200, map[string]any{"ok": false, "error": "ไม่พบเครื่องปลายทาง"})
		return
	}
	res, code, err := httpJSON(url, map[string]any{"info": info, "to": to, "msg": d["msg"]})
	switch {
	case err != nil:
		writeJSON(w, 200, map[string]any{"ok": false, "error": "ติดต่อปลายทางไม่ได้ (ตรวจไฟร์วอลล์หรือเครือข่าย)"})
	case code == 503:
		writeJSON(w, 200, map[string]any{"ok": false, "error": "ปลายทางไม่ได้เปิดโปรแกรมไว้"})
	default:
		writeJSON(w, 200, res)
	}
}

func (s *State) apiSettings(w http.ResponseWriter, ep *Endpoint, d map[string]any) {
	s.mu.Lock()
	if n, ok := d["name"].(string); ok {
		if n = strings.TrimSpace(trunc(n, 60)); n != "" {
			ep.Name = n
			if ep == s.local {
				s.cfg.Name = n
			}
		}
	}
	var errMsg string
	if ep == s.local {
		if v, ok := d["auto_answer"].(bool); ok {
			s.cfg.AutoAnswer = v
		}
		if v, ok := d["robot_mode"].(bool); ok {
			s.cfg.RobotMode = v
			if v {
				s.cfg.AutoAnswer = true
				if err := setAutostart(true, true); err != nil {
					errMsg = "ตั้งค่าเปิดอัตโนมัติไม่ได้: " + err.Error()
				}
			}
		}
		if v, ok := d["autostart"].(bool); ok {
			if err := setAutostart(v, s.cfg.RobotMode); err != nil {
				errMsg = "ตั้งค่าเปิดอัตโนมัติไม่ได้: " + err.Error()
			}
		}
		saveConfig(s.cfg)
	}
	send(ep, map[string]any{"type": "settings", "settings": s.settingsFor(ep)})
	s.pushPeersLocked()
	s.mu.Unlock()
	if errMsg != "" {
		writeJSON(w, 200, map[string]any{"ok": false, "error": errMsg})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true})
}

func (s *State) apiAdd(w http.ResponseWriter, d map[string]any) {
	target := strings.TrimSpace(str(d, "ip"))
	host, port := target, s.port
	if h, p, err := net.SplitHostPort(target); err == nil {
		host = h
		port, _ = strconv.Atoi(p)
	}
	if host == "" {
		writeJSON(w, 200, map[string]any{"ok": false, "error": "กรุณาใส่ IP"})
		return
	}
	info, _, err := httpJSON(fmt.Sprintf("http://%s:%d/peer/info", host, port), nil)
	if err != nil || info == nil {
		writeJSON(w, 200, map[string]any{"ok": false, "error": fmt.Sprintf("ติดต่อ %s:%d ไม่ได้", host, port)})
		return
	}
	s.upsert(info, host)
	s.mu.Lock()
	found := false
	for _, m := range s.cfg.ManualIPs {
		found = found || m == host
	}
	if !found {
		s.cfg.ManualIPs = append(s.cfg.ManualIPs, host)
		saveConfig(s.cfg)
	}
	s.mu.Unlock()
	writeJSON(w, 200, map[string]any{"ok": true, "name": str(info, "name")})
}

// ======================================================================== main

func setupLogging() {
	home, _ := os.UserHomeDir()
	dataDir = filepath.Join(home, ".tesr_lan_call")
	_ = os.MkdirAll(dataDir, 0o755)
	lp := filepath.Join(dataDir, "log.txt")
	if st, err := os.Stat(lp); err == nil && st.Size() > 1<<20 {
		_ = os.Remove(lp)
	}
	if f, err := os.OpenFile(lp, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644); err == nil {
		log.SetOutput(io.MultiWriter(f, os.Stderr))
	}
}

func main() {
	setupLogging()
	// macOS อาจส่ง -psn_xxx มาตอนเปิดแอป: ตัดทิ้งก่อน parse
	var args []string
	for _, a := range os.Args[1:] {
		if !strings.HasPrefix(a, "-psn_") {
			args = append(args, a)
		}
	}
	fset := flag.NewFlagSet(appName, flag.ContinueOnError)
	port := fset.Int("port", defaultPort, "พอร์ต TCP")
	httpsPort := fset.Int("https-port", defaultHTTPS, "พอร์ต HTTPS สำหรับมือถือ/iPad")
	disc := fset.Int("discovery-port", defaultDisc, "พอร์ต UDP สำหรับค้นหาเครื่อง")
	name := fset.String("name", "", "ตั้งชื่อเครื่อง")
	kiosk := fset.Bool("kiosk", false, "เปิดเต็มจอและอนุญาตกล้อง/ไมค์อัตโนมัติ (จอหุ่นยนต์)")
	background := fset.Bool("background", false, "ทำงานเบื้องหลัง เปิดหน้าต่างเองเมื่อมีสายเข้า")
	noWindow := fset.Bool("no-window", false, "ไม่เปิดหน้าต่างเลย (server อย่างเดียว)")
	robot := fset.Bool("robot", false, "เปิดโหมดหุ่นยนต์: เต็มจอ + รับสายอัตโนมัติ + เปิดเองตอนเปิดเครื่อง")
	fixFW := fset.Bool("fix-firewall", false, "เปิดสิทธิ์ไฟร์วอลล์ Windows ให้มือถือเข้าได้ (ต้องรันแบบผู้ดูแลระบบ)")
	_ = fset.Parse(args)
	if *fixFW {
		os.Exit(runFirewallScript(*port, *httpsPort, *disc))
	}

	_, statErr := os.Stat(filepath.Join(dataDir, "config.json"))
	firstRun := statErr != nil
	cfg := loadConfig()
	if *name != "" {
		cfg.Name = *name
	}
	if *robot {
		cfg.RobotMode, cfg.AutoAnswer = true, true
	}
	saveConfig(cfg)
	// ครั้งแรก: ให้เปิดเองตอนเปิดเครื่อง (ทำงานเบื้องหลัง) เพื่อให้รับสายได้ตลอด ปิดได้ในหน้าตั้งค่า
	if (firstRun || *robot) && !*noWindow {
		if err := setAutostart(true, cfg.RobotMode); err != nil {
			log.Println("[autostart]", err)
		}
	}
	isKiosk := *kiosk || cfg.RobotMode
	localURL := fmt.Sprintf("http://localhost:%d/", *port)

	ln, err := net.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", *port))
	if err != nil {
		log.Printf("[%s] เปิดอยู่แล้ว -> เปิดหน้าต่างเดิม", appName)
		if !*noWindow {
			openWindow(localURL, isKiosk)
		}
		return
	}

	s := &State{cfg: cfg, port: *port, https: *httpsPort, disc: *disc, peers: map[string]*Peer{},
		canWake: !*noWindow, kiosk: isKiosk, kick: make(chan struct{}, 1)}
	s.local = newEndpoint(cfg.ID, cfg.Name, "desktop", osName())
	s.endpoints = map[string]*Endpoint{s.local.ID: s.local}
	h := s.routes()

	if cert, err := ensureCert(lanIPs()); err == nil {
		tln, err := tls.Listen("tcp", fmt.Sprintf("0.0.0.0:%d", *httpsPort), &tls.Config{Certificates: []tls.Certificate{cert}})
		if err == nil {
			s.httpsOK = true
			srv := &http.Server{Handler: h, ErrorLog: log.New(io.Discard, "", 0)}
			go func() { _ = srv.Serve(tln) }()
		} else {
			s.httpsErr = "เปิดพอร์ตมือถือไม่ได้: " + err.Error()
		}
	} else {
		s.httpsErr = "สร้างใบรับรองไม่ได้: " + err.Error()
	}

	go s.announcer()
	go s.listener()
	go s.watcher()

	log.Printf("%s %s | %s (%s, %s) | %s | mobile %s", appName, version, cfg.Name, lanIP(), osName(), localURL, s.mobileURL())
	if !*noWindow && !*background {
		time.AfterFunc(400*time.Millisecond, func() { openWindow(localURL, isKiosk) })
	}
	srv := &http.Server{Handler: h, ErrorLog: log.New(io.Discard, "", 0)}
	if err := srv.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Println("[http]", err)
	}
}
