package capture

import (
	"bufio"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"golang.org/x/net/http2"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWhitelist(t *testing.T) {
	c := Config{Domains: []string{"Example.COM", "*.example.org"}, Paths: []string{"/exact", "/api/*"}, UIDs: []int{10001}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"example.com", "EXAMPLE.COM:443", "a.example.org"} {
		if !c.MatchDomain(host) {
			t.Error(host)
		}
	}
	for _, host := range []string{"badexample.com", "example.org", "example.com.attacker.test", ""} {
		if c.MatchDomain(host) {
			t.Error(host)
		}
	}
	for _, p := range []string{"/exact", "/api/one"} {
		if !c.MatchPath(p) {
			t.Error(p)
		}
	}
	for _, p := range []string{"/exact/", "/apiary", "/api", "/%61pi/one"} {
		if c.MatchPath(p) {
			t.Error(p)
		}
	}
	if !c.MatchUID(10001) || c.MatchUID(10002) || c.MatchUID(-1) {
		t.Fatal("UID filter")
	}
	for _, c := range []Config{{Domains: []string{"https://example.com"}}, {Domains: []string{"example.*"}}, {Domains: []string{"a..b"}}, {Paths: []string{"/api/*/x"}}, {Paths: []string{"x"}}, {Packages: []string{"missing"}}} {
		if c.Validate() == nil {
			t.Error("invalid filter accepted", c)
		}
	}
}
func TestAuthorityAndRestart(t *testing.T) {
	dir := t.TempDir()
	a, err := loadAuthority(dir)
	if err != nil {
		t.Fatal(err)
	}
	b, err := loadAuthority(dir)
	if err != nil || a.fingerprint() != b.fingerprint() {
		t.Fatal("CA changed", err)
	}
	c, err := loadAuthority(t.TempDir())
	if err != nil || a.fingerprint() == c.fingerprint() {
		t.Fatal("CA shared", err)
	}
	fi, _ := os.Stat(filepath.Join(dir, "authority.pem"))
	if fi.Mode().Perm() != 0600 {
		t.Fatal(fi.Mode())
	}
	leaf, err := a.leaf("example.com")
	if err != nil {
		t.Fatal(err)
	}
	cert, _ := x509.ParseCertificate(leaf.Certificate[0])
	roots := x509.NewCertPool()
	roots.AddCert(a.cert)
	if _, err = cert.Verify(x509.VerifyOptions{DNSName: "example.com", Roots: roots}); err != nil {
		t.Fatal(err)
	}
	m := New(dir)
	if err = m.update(Config{Enabled: true, HTTPS: true}); err != nil {
		t.Fatal(err)
	}
	n := New(dir)
	n.Init(dir)
	if n.Enabled() {
		t.Fatal("capture unexpectedly resumed")
	}
	if strings.Contains(m.Command("ca", ""), "PRIVATE KEY") {
		t.Fatal("private key exported")
	}
}
func TestHTTPAndTLSCapture(t *testing.T) {
	for _, secure := range []bool{false, true} {
		t.Run(map[bool]string{false: "http", true: "https"}[secure], func(t *testing.T) {
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				b, _ := io.ReadAll(r.Body)
				w.Header().Set("X-Test", "yes")
				w.Write(append([]byte("response:"), b...))
			}))
			if secure {
				server.StartTLS()
			} else {
				server.Start()
			}
			defer server.Close()
			m := New(t.TempDir())
			cfg := Config{Enabled: true, HTTPS: secure, Domains: []string{"example.test"}, Paths: []string{"/api/*"}, UIDs: []int{42}}
			if err := m.update(cfg); err != nil {
				t.Fatal(err)
			}
			roots := x509.NewCertPool()
			if secure {
				roots.AddCert(server.Certificate())
			}
			// Route to the local fixture while still verifying its certificate
			// against the original logical hostname, example.com.
			dial := func(ctx context.Context, address string) (net.Conn, error) {
				return (&net.Dialer{}).DialContext(ctx, "tcp", server.Listener.Addr().String())
			}
			if secure {
				cfg.Domains = []string{"example.com"}
				if err := m.update(cfg); err != nil {
					t.Fatal(err)
				}
			}
			host := "example.test"
			if secure {
				host = "example.com"
			}
			client, inbound := net.Pipe()
			done := make(chan struct{})
			port := 80
			if secure {
				port = 443
			}
			go func() {
				defer close(done)
				if !m.Handle(inbound, 42, port, dial, func(net.Conn) { t.Error("unexpected bypass") }, roots) {
					t.Error("not handled")
				}
			}()
			var conn net.Conn = client
			if secure {
				trust := x509.NewCertPool()
				trust.AddCert(m.ca.cert)
				conn = tls.Client(client, &tls.Config{RootCAs: trust, ServerName: host, NextProtos: []string{"http/1.1"}})
			}
			conn.SetDeadline(time.Now().Add(10 * time.Second))
			br := bufio.NewReader(conn)
			for _, path := range []string{"/skip", "/api/one?query=1", "/api/after-clear"} {
				payload := "hello"
				if path == "/api/after-clear" {
					m.Command("clear", "")
					payload = strings.Repeat("x", bodyLimit+64)
				}
				req, _ := http.NewRequest("POST", "http://"+host+path, strings.NewReader(payload))
				req.Write(conn)
				resp, err := http.ReadResponse(br, req)
				if err != nil {
					t.Fatal(err)
				}
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				if resp.StatusCode != 200 || string(body) != "response:"+payload {
					t.Fatalf("%d %s", resp.StatusCode, body)
				}
			}
			conn.Close()
			select {
			case <-done:
			case <-time.After(10 * time.Second):
				t.Fatal("connection leak")
			}
			m.mu.Lock()
			defer m.mu.Unlock()
			if len(m.records) != 1 {
				t.Fatalf("records=%d", len(m.records))
			}
			rec := m.records[0]
			body, _ := base64.StdEncoding.DecodeString(rec.ResponseBody)
			if string(body) != ("response:" + strings.Repeat("x", bodyLimit))[:bodyLimit] || !strings.Contains(rec.URL, "/api/after-clear") || !rec.RequestTruncated || !rec.ResponseTruncated {
				t.Fatal(rec)
			}
		})
	}
}
func TestTLSBypassPreservesClientHello(t *testing.T) {
	m := New(t.TempDir())
	if err := m.update(Config{Enabled: true, HTTPS: true, Domains: []string{"allowed.test"}}); err != nil {
		t.Fatal(err)
	}
	upstream := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("bypass")) }))
	defer upstream.Close()
	client, inbound := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.Handle(inbound, 1, 443, nil, func(c net.Conn) {
			remote, e := net.Dial("tcp", upstream.Listener.Addr().String())
			if e != nil {
				t.Error(e)
				return
			}
			defer remote.Close()
			defer c.Close()
			go io.Copy(remote, c)
			io.Copy(c, remote)
		}, nil)
	}()
	roots := x509.NewCertPool()
	roots.AddCert(upstream.Certificate())
	secure := tls.Client(client, &tls.Config{RootCAs: roots, ServerName: "example.com", NextProtos: []string{"http/1.1"}})
	secure.SetDeadline(time.Now().Add(10 * time.Second))
	req, _ := http.NewRequest("GET", "https://example.com/", nil)
	req.Close = true
	req.Write(secure)
	resp, err := http.ReadResponse(bufio.NewReader(secure), req)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	secure.Close()
	if string(b) != "bypass" {
		t.Fatal(string(b))
	}
	<-done
	if len(m.records) != 0 {
		t.Fatal("bypass recorded")
	}
}
func TestUntrustedUpstreamRejected(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write([]byte("secret")) }))
	defer server.Close()
	m := New(t.TempDir())
	m.update(Config{Enabled: true, HTTPS: true})
	client, inbound := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.Handle(inbound, 1, 443, func(ctx context.Context, s string) (net.Conn, error) {
			return net.Dial("tcp", server.Listener.Addr().String())
		}, func(net.Conn) { t.Error("bypass") }, x509.NewCertPool())
	}()
	roots := x509.NewCertPool()
	roots.AddCert(m.ca.cert)
	conn := tls.Client(client, &tls.Config{RootCAs: roots, ServerName: "example.com"})
	conn.SetDeadline(time.Now().Add(10 * time.Second))
	req, _ := http.NewRequest("GET", "https://example.com/", nil)
	req.Close = true
	req.Write(conn)
	resp, err := http.ReadResponse(bufio.NewReader(conn), req)
	if err != nil {
		t.Fatal(err)
	}
	io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	conn.Close()
	<-done
	if resp.StatusCode != 502 {
		t.Fatalf("invalid upstream accepted: %d", resp.StatusCode)
	}
}
func TestBoundedStorageAndClearEpoch(t *testing.T) {
	m := New(t.TempDir())
	m.update(Config{Enabled: true})
	_, gen, epoch := m.snapshot()
	var s sample
	s.Write([]byte(strings.Repeat("x", bodyLimit*3)))
	if len(s.data) != bodyLimit || s.size != bodyLimit*3 {
		t.Fatal("body bound")
	}
	for i := 0; i < recordLimit+5; i++ {
		m.add(Record{Status: i}, gen, epoch)
	}
	if len(m.records) != recordLimit || m.records[0].Status != 5 {
		t.Fatal("record bound")
	}
	m.Command("clear", "")
	m.add(Record{}, gen, epoch)
	if len(m.records) != 0 {
		t.Fatal("clear race")
	}
	var result map[string]any
	json.Unmarshal([]byte(m.Command("get", "1")), &result)
	if result["error"] == nil {
		t.Fatal("missing error")
	}
	client, server := net.Pipe()
	defer client.Close()
	m.register(server, gen)
	m.Stop()
	if _, err := client.Write([]byte("x")); err == nil {
		t.Fatal("stop left connection open")
	}
}

func TestHTTP2AndTrailers(t *testing.T) {
	upstream := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.ProtoMajor != 2 {
			t.Errorf("upstream protocol %s", r.Proto)
		}
		w.Header().Set("Trailer", "Grpc-Status")
		w.Header().Set("Content-Type", "application/grpc")
		w.Write([]byte("h2 payload"))
		w.Header().Set("Grpc-Status", "0")
	}))
	upstream.EnableHTTP2 = true
	upstream.StartTLS()
	defer upstream.Close()
	m := New(t.TempDir())
	if err := m.update(Config{Enabled: true, HTTPS: true, Domains: []string{"example.com"}, Paths: []string{"/allowed"}}); err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(upstream.Certificate())
	client, inbound := net.Pipe()
	done := make(chan struct{})
	go func() {
		defer close(done)
		m.Handle(inbound, 5, 443, func(ctx context.Context, s string) (net.Conn, error) {
			return (&net.Dialer{}).DialContext(ctx, "tcp", upstream.Listener.Addr().String())
		}, func(net.Conn) { t.Error("h2 bypass") }, roots)
	}()
	trust := x509.NewCertPool()
	trust.AddCert(m.ca.cert)
	secure := tls.Client(client, &tls.Config{RootCAs: trust, ServerName: "example.com", NextProtos: []string{"h2"}})
	secure.SetDeadline(time.Now().Add(10 * time.Second))
	if err := secure.Handshake(); err != nil {
		t.Fatal(err)
	}
	transport := &http2.Transport{}
	c, err := transport.NewClientConn(secure)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"/skip", "/allowed"} {
		req, _ := http.NewRequest("GET", "https://example.com"+p, nil)
		resp, err := c.RoundTrip(req)
		if err != nil {
			t.Fatal(err)
		}
		data, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			t.Fatal(err)
		}
		if string(data) != "h2 payload" || resp.ProtoMajor != 2 || resp.Trailer.Get("Grpc-Status") != "0" {
			t.Fatalf("invalid h2 response: %s, %s, %v", resp.Proto, data, resp.Trailer)
		}
	}
	c.Close()
	select {
	case <-done:
	case <-time.After(10 * time.Second):
		t.Fatal("h2 connection leak")
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if len(m.records) != 1 {
		t.Fatalf("h2 records %d", len(m.records))
	}
}

func TestUIDBypassDoesNotRead(t *testing.T) {
	m := New(t.TempDir())
	m.update(Config{Enabled: true, UIDs: []int{1}})
	client, inbound := net.Pipe()
	defer client.Close()
	defer inbound.Close()
	if m.Handle(inbound, 2, 80, nil, nil, nil) {
		t.Fatal("unmatched UID handled")
	}
	go client.Write([]byte("untouched"))
	b := make([]byte, 9)
	if _, err := io.ReadFull(inbound, b); err != nil || string(b) != "untouched" {
		t.Fatal(string(b), err)
	}
}

func TestHTTPDomainBypassPreservesBytes(t *testing.T) {
	m := New(t.TempDir())
	m.update(Config{Enabled: true, Domains: []string{"allowed.test"}})
	client, inbound := net.Pipe()
	defer client.Close()
	payload := "POST /skip HTTP/1.1\r\nHost: other.test\r\nContent-Length: 4\r\n\r\nbody"
	done := make(chan struct{})
	go func() {
		defer close(done)
		defer inbound.Close()
		m.Handle(inbound, 1, 80, nil, func(c net.Conn) {
			if len(m.slots) != 0 {
				t.Error("bypass consumes capture slot")
			}
			b := make([]byte, len(payload))
			_, err := io.ReadFull(c, b)
			if err != nil || string(b) != payload {
				t.Error("HTTP bypass modified bytes", err)
			}
		}, nil)
	}()
	client.SetDeadline(time.Now().Add(5 * time.Second))
	client.Write([]byte(payload))
	<-done
}
