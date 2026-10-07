package main

// ใบรับรอง HTTPS สำหรับมือถือ/iPad
//
//   - CA ประจำคอมเครื่องนี้ สร้างครั้งเดียว อายุ 10 ปี และจำกัดให้ใช้ได้เฉพาะ IP ภายใน (192.168.x, 10.x, 172.16-31.x)
//     กับชื่อ .local เท่านั้น จึงเอาไปปลอมเว็บไซต์ภายนอกไม่ได้
//   - ใบรับรองของเซิร์ฟเวอร์ออกโดย CA นี้ ครอบคลุมทุก IP ที่คอมเคยใช้ และออกใหม่เองทันทีเมื่อเจอ IP ใหม่
//
// iPad/iPhone ที่ติดตั้ง CA (ไฟล์ .mobileconfig) แล้ว จะไม่ขึ้นคำเตือนอีกเลย แม้ IP ของคอมเปลี่ยน
// และไอคอนบนหน้าจอโฮมก็เปิดได้ เครื่องที่ไม่ติดตั้งก็ยังใช้ได้ด้วยการกดยอมรับคำเตือนใน Safari ครั้งเดียว

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const (
	leafDays  = 800 // Apple ยอมรับใบรับรองเซิร์ฟเวอร์อายุไม่เกิน 825 วัน
	maxLeafIP = 24
)

var privateNets = func() []*net.IPNet {
	var out []*net.IPNet
	for _, c := range []string{"10.0.0.0/8", "172.16.0.0/12", "192.168.0.0/16", "127.0.0.0/8", "169.254.0.0/16", "100.64.0.0/10"} {
		_, n, _ := net.ParseCIDR(c)
		out = append(out, n)
	}
	return out
}()

// IP ที่ใส่ในใบรับรองได้ (IPv4 ภายในเท่านั้น)
func certIP(s string) net.IP {
	ip := net.ParseIP(s).To4()
	if ip == nil {
		return nil
	}
	for _, n := range privateNets {
		if n.Contains(ip) {
			return ip
		}
	}
	return nil
}

type certStore struct {
	dir    string
	mu     sync.Mutex
	caCert *x509.Certificate
	caKey  *ecdsa.PrivateKey
	leaf   *tls.Certificate
	ips    []string // IP ที่ใบรับรองปัจจุบันครอบคลุม
	until  time.Time
}

func writePEM(path, typ string, der []byte, mode os.FileMode) error {
	return os.WriteFile(path, pem.EncodeToMemory(&pem.Block{Type: typ, Bytes: der}), mode)
}

func readPEM(path string) []byte {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	if blk, _ := pem.Decode(b); blk != nil {
		return blk.Bytes
	}
	return nil
}

func newCertStore(dir, pcName string) (*certStore, error) {
	cs := &certStore{dir: dir}
	caP, keyP := filepath.Join(dir, "ca.pem"), filepath.Join(dir, "ca-key.pem")
	if der, kder := readPEM(caP), readPEM(keyP); der != nil && kder != nil {
		c, err1 := x509.ParseCertificate(der)
		k, err2 := x509.ParseECPrivateKey(kder)
		if err1 == nil && err2 == nil && time.Until(c.NotAfter) > 400*24*time.Hour {
			cs.caCert, cs.caKey = c, k
		}
	}
	if cs.caCert == nil {
		k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
		if err != nil {
			return nil, err
		}
		pub, _ := x509.MarshalPKIXPublicKey(&k.PublicKey)
		ski := sha1.Sum(pub)
		serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
		cn := "TESR LAN Call"
		if n := strings.TrimSpace(trunc(pcName, 40)); n != "" {
			cn += " - " + n
		}
		var permitted []*net.IPNet
		for _, n := range privateNets {
			permitted = append(permitted, n)
		}
		tpl := &x509.Certificate{
			SerialNumber:          serial,
			Subject:               pkix.Name{CommonName: cn, Organization: []string{"TESR Co., Ltd."}},
			NotBefore:             time.Now().Add(-24 * time.Hour),
			NotAfter:              time.Now().Add(10 * 365 * 24 * time.Hour),
			KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign | x509.KeyUsageDigitalSignature,
			BasicConstraintsValid: true,
			IsCA:                  true,
			MaxPathLenZero:        true,
			SubjectKeyId:          ski[:],
			// ใช้ได้เฉพาะเครือข่ายภายใน
			PermittedIPRanges:   permitted,
			PermittedDNSDomains: []string{"localhost", "local"},
		}
		der, err := x509.CreateCertificate(rand.Reader, tpl, tpl, &k.PublicKey, k)
		if err != nil {
			return nil, err
		}
		kb, _ := x509.MarshalECPrivateKey(k)
		if err := writePEM(keyP, "EC PRIVATE KEY", kb, 0o600); err != nil {
			return nil, err
		}
		if err := writePEM(caP, "CERTIFICATE", der, 0o644); err != nil {
			return nil, err
		}
		cs.caCert, _ = x509.ParseCertificate(der)
		cs.caKey = k
		// ใบรับรองเก่าที่ออกโดย CA เดิม (ถ้ามี) ใช้ไม่ได้แล้ว
		_ = os.Remove(filepath.Join(dir, "server.pem"))
	}
	// ใบรับรองเซิร์ฟเวอร์ที่ออกไว้แล้ว
	if der, kder := readPEM(filepath.Join(dir, "server.pem")), readPEM(filepath.Join(dir, "server-key.pem")); der != nil && kder != nil {
		if c, err := x509.ParseCertificate(der); err == nil && c.CheckSignatureFrom(cs.caCert) == nil {
			if k, err := x509.ParseECPrivateKey(kder); err == nil {
				cs.leaf = &tls.Certificate{Certificate: [][]byte{der, cs.caCert.Raw}, PrivateKey: k, Leaf: c}
				cs.until = c.NotAfter
				for _, ip := range c.IPAddresses {
					if ip.To4() != nil && !ip.IsLoopback() {
						cs.ips = append(cs.ips, ip.String())
					}
				}
			}
		}
	}
	// ไฟล์จากเวอร์ชันก่อน (ใบรับรองแบบ self-signed) ไม่ใช้แล้ว
	for _, f := range []string{"cert.pem", "key.pem", "cert-ip.txt"} {
		_ = os.Remove(filepath.Join(dir, f))
	}
	return cs, nil
}

func (cs *certStore) covers(ip string) bool {
	for _, x := range cs.ips {
		if x == ip {
			return true
		}
	}
	return false
}

// ensure ทำให้ใบรับรองครอบคลุม IP ที่ให้มาทั้งหมด (ออกใหม่เมื่อจำเป็น) ต้องถือ cs.mu อยู่
func (cs *certStore) ensureLocked(want []string) error {
	need := cs.leaf == nil || time.Until(cs.until) < 30*24*time.Hour
	var add []string
	for _, s := range want {
		if ip := certIP(s); ip != nil && !ip.IsLoopback() && !cs.covers(ip.String()) {
			add = append(add, ip.String())
		}
	}
	if !need && len(add) == 0 {
		return nil
	}
	// IP ใหม่ไว้หน้า แล้วตามด้วย IP เดิม (เก็บไว้ ไม่ให้มือถือต้องกดยอมรับใหม่เมื่อสลับเครือข่ายไปมา)
	ips := append(add, cs.ips...)
	if len(ips) > maxLeafIP {
		ips = ips[:maxLeafIP]
	}
	k, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return err
	}
	serial, _ := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 120))
	tpl := &x509.Certificate{
		SerialNumber: serial,
		Subject:      pkix.Name{CommonName: "TESR LAN Call", Organization: []string{"TESR Co., Ltd."}},
		NotBefore:    time.Now().Add(-24 * time.Hour),
		NotAfter:     time.Now().Add(leafDays * 24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		DNSNames:     []string{"localhost"},
		IPAddresses:  []net.IP{net.IPv4(127, 0, 0, 1).To4()},
	}
	for _, s := range ips {
		tpl.IPAddresses = append(tpl.IPAddresses, net.ParseIP(s).To4())
	}
	der, err := x509.CreateCertificate(rand.Reader, tpl, cs.caCert, &k.PublicKey, cs.caKey)
	if err != nil {
		return err
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		return err
	}
	kb, _ := x509.MarshalECPrivateKey(k)
	_ = writePEM(filepath.Join(cs.dir, "server-key.pem"), "EC PRIVATE KEY", kb, 0o600)
	_ = writePEM(filepath.Join(cs.dir, "server.pem"), "CERTIFICATE", der, 0o644)
	cs.leaf = &tls.Certificate{Certificate: [][]byte{der, cs.caCert.Raw}, PrivateKey: k, Leaf: leaf}
	cs.ips, cs.until = ips, leaf.NotAfter
	return nil
}

func (cs *certStore) ensure(want []string) error {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	return cs.ensureLocked(want)
}

// ใช้กับ tls.Config.GetCertificate: เลือก/ออกใบรับรองให้ตรงกับ IP ที่มือถือเรียกเข้ามา
func (cs *certStore) get(hello *tls.ClientHelloInfo) (*tls.Certificate, error) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	want := []string{}
	if hello != nil && hello.Conn != nil {
		if a, ok := hello.Conn.LocalAddr().(*net.TCPAddr); ok {
			want = append(want, a.IP.String())
		}
	}
	if err := cs.ensureLocked(want); err != nil && cs.leaf == nil {
		return nil, err
	}
	if cs.leaf == nil {
		return nil, errors.New("no certificate")
	}
	return cs.leaf, nil
}

func (cs *certStore) caDER() []byte { return cs.caCert.Raw }

// ชื่อ CA ที่จะเห็นใน "การตั้งค่าความเชื่อถือใบรับรอง" บน iPhone/iPad
func (cs *certStore) caName() string { return cs.caCert.Subject.CommonName }

// โปรไฟล์สำหรับ iPhone/iPad (ติดตั้งครั้งเดียว ไม่ต้องกดยอมรับคำเตือนอีก)
func (cs *certStore) mobileconfig() []byte {
	sum := sha256.Sum256(cs.caCert.Raw)
	uuid := func(salt byte) string {
		h := sha256.Sum256(append(sum[:], salt))
		b := h[:16]
		b[6] = b[6]&0x0f | 0x50
		b[8] = b[8]&0x3f | 0x80
		return fmt.Sprintf("%X-%X-%X-%X-%X", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
	}
	esc := func(s string) string {
		var b bytes.Buffer
		_ = xml.EscapeText(&b, []byte(s))
		return b.String()
	}
	id := fmt.Sprintf("%x", sum[:6])
	name := esc(cs.caName())
	return []byte(`<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
	<key>PayloadContent</key>
	<array>
		<dict>
			<key>PayloadCertificateFileName</key>
			<string>tesr-lan-call-ca.cer</string>
			<key>PayloadContent</key>
			<data>` + base64.StdEncoding.EncodeToString(cs.caCert.Raw) + `</data>
			<key>PayloadDescription</key>
			<string>ใบรับรองสำหรับเชื่อมต่อ TESR LAN Call ภายในเครือข่ายเท่านั้น</string>
			<key>PayloadDisplayName</key>
			<string>` + name + `</string>
			<key>PayloadIdentifier</key>
			<string>com.tesr.lancall.ca.` + id + `</string>
			<key>PayloadType</key>
			<string>com.apple.security.root</string>
			<key>PayloadUUID</key>
			<string>` + uuid(1) + `</string>
			<key>PayloadVersion</key>
			<integer>1</integer>
		</dict>
	</array>
	<key>PayloadDescription</key>
	<string>ให้ iPad/iPhone เปิด TESR LAN Call ได้ทันทีโดยไม่ขึ้นคำเตือนความปลอดภัย ใช้ได้เฉพาะเครือข่ายภายใน</string>
	<key>PayloadDisplayName</key>
	<string>` + name + `</string>
	<key>PayloadIdentifier</key>
	<string>com.tesr.lancall.` + id + `</string>
	<key>PayloadOrganization</key>
	<string>TESR Co., Ltd.</string>
	<key>PayloadRemovalDisallowed</key>
	<false/>
	<key>PayloadType</key>
	<string>Configuration</string>
	<key>PayloadUUID</key>
	<string>` + uuid(2) + `</string>
	<key>PayloadVersion</key>
	<integer>1</integer>
</dict>
</plist>
`)
}
