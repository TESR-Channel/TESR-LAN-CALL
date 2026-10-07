package main

import (
	"crypto/tls"
	"crypto/x509"
	"net"
	"os"
	"strings"
	"testing"
)

func TestOrderNetsPrefersWiFi(t *testing.T) {
	good := []lanNet{{"10.0.0.5", "Ethernet"}, {"192.168.1.182", "Wi-Fi"}, {"172.16.0.9", "Ethernet 2"}}
	got := orderNets(good, "10.0.0.5")
	if got[0].IP != "192.168.1.182" || got[1].IP != "10.0.0.5" || len(got) != 3 {
		t.Fatalf("Wi-Fi should come first, then the default-route card: %+v", got)
	}
	if got := orderNets([]lanNet{{"192.168.1.5", "eth0"}, {"192.168.9.9", "eth1"}}, "192.168.9.9"); got[0].IP != "192.168.9.9" {
		t.Fatalf("without Wi-Fi the default-route card comes first: %+v", got)
	}
	if got := orderNets(nil, "192.0.2.2"); len(got) != 1 || got[0].IP != "192.0.2.2" {
		t.Fatalf("fallback: %+v", got)
	}
}

func TestCertStoreKeepsCAAndCoversNewIPs(t *testing.T) {
	dir := t.TempDir()
	cs, err := newCertStore(dir, "Test PC")
	if err != nil {
		t.Fatal(err)
	}
	if err := cs.ensure([]string{"192.168.1.10", "8.8.8.8"}); err != nil {
		t.Fatal(err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(cs.caCert)
	verify := func(c *tls.Certificate, ip string) error {
		_, err := c.Leaf.Verify(x509.VerifyOptions{Roots: pool, DNSName: ip})
		return err
	}
	if err := verify(cs.leaf, "192.168.1.10"); err != nil {
		t.Fatalf("leaf must verify for LAN IP: %v", err)
	}
	for _, ip := range cs.leaf.Leaf.IPAddresses {
		if ip.String() == "8.8.8.8" {
			t.Fatal("public IP must never be put in the certificate")
		}
	}
	// a phone connects on an IP the certificate does not cover yet
	conn := fakeConn{local: &net.TCPAddr{IP: net.ParseIP("10.1.2.3"), Port: 47843}}
	c, err := cs.get(&tls.ClientHelloInfo{Conn: conn})
	if err != nil || verify(c, "10.1.2.3") != nil || verify(c, "192.168.1.10") != nil {
		t.Fatalf("new IP must be added and old IP kept: %v", err)
	}
	// restart: same CA, same leaf
	ca := cs.caCert.Raw
	cs2, err := newCertStore(dir, "Test PC")
	if err != nil || string(cs2.caCert.Raw) != string(ca) || !cs2.covers("10.1.2.3") {
		t.Fatalf("CA and certificate must survive a restart: %v", err)
	}
	if b, _ := os.ReadFile(dir + "/ca-key.pem"); !strings.Contains(string(b), "PRIVATE KEY") {
		t.Fatal("CA key not saved")
	}
	if !strings.Contains(string(cs.mobileconfig()), "com.apple.security.root") {
		t.Fatal("profile payload missing")
	}
}

type fakeConn struct {
	net.Conn
	local net.Addr
}

func (f fakeConn) LocalAddr() net.Addr { return f.local }
func (f fakeConn) RemoteAddr() net.Addr {
	return &net.TCPAddr{IP: net.ParseIP("10.1.2.50"), Port: 5555}
}
