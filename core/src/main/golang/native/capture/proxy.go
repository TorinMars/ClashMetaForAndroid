package capture

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"fmt"
	"golang.org/x/net/http2"
	"io"
	"net"
	"net/http"
	"sort"
	"strings"
	"sync"
	"time"
)

// Dial preserves the caller's normal proxy routing and application metadata.
type Dial func(context.Context, string) (net.Conn, error)

// Handle returns false only when the original, unread connection should pass through.
// Capturing supports HTTP/1.x on TCP 80 and HTTP/1.x + HTTP/2 over TLS on TCP 443.
func (m *Manager) Handle(conn net.Conn, uid, port int, dial Dial, pass func(net.Conn), roots *x509.CertPool) bool {
	cfg, gen, _ := m.snapshot()
	if !cfg.Enabled || !cfg.MatchUID(uid) || (port != 80 && port != 443) || (port == 443 && !cfg.HTTPS) {
		return false
	}
	select {
	case m.slots <- struct{}{}:
	default:
		return false
	}
	slotHeld := true
	releaseSlot := func() {
		if slotHeld {
			<-m.slots
			slotHeld = false
		}
	}
	defer releaseSlot()
	// Register the original socket so stop/config changes also cancel probing and TLS.
	if !m.register(conn, gen) {
		return false
	}
	defer m.unregister(conn)
	var host string
	client := conn
	ok := true
	if port == 443 {
		client, host, ok = inspectTLS(conn)
	} else {
		client, host = inspectHTTP(conn)
	}
	if !ok || !cfg.MatchDomain(host) {
		m.unregister(conn)
		releaseSlot()
		pass(client)
		return true
	}
	defer conn.Close()
	if port == 443 {
		m.mu.Lock()
		authority := m.ca
		m.mu.Unlock()
		if authority == nil {
			return true
		}
		cert, err := authority.leaf(host)
		if err != nil {
			m.fail(err)
			return true
		}
		secure := tls.Server(client, &tls.Config{Certificates: []tls.Certificate{cert}, NextProtos: []string{"h2", "http/1.1"}, MinVersion: tls.VersionTLS12})
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		err = secure.HandshakeContext(ctx)
		cancel()
		if err != nil {
			m.fail(fmt.Errorf("HTTPS 握手失败（检查 CA 信任或证书固定）：%w", err))
			return true
		}
		client = secure
	}
	m.serve(client, cfg, gen, uid, port, host, dial, roots)
	return true
}

type singleListener struct {
	conn net.Conn
	once bool
	done chan struct{}
	stop sync.Once
}

func (l *singleListener) Accept() (net.Conn, error) {
	if !l.once {
		l.once = true
		return &closingConn{Conn: l.conn, closeDone: func() { l.Close() }}, nil
	}
	<-l.done
	return nil, net.ErrClosed
}
func (l *singleListener) Close() error   { l.stop.Do(func() { close(l.done) }); return nil }
func (l *singleListener) Addr() net.Addr { return l.conn.LocalAddr() }

type closingConn struct {
	net.Conn
	closeDone func()
}

func (c *closingConn) Close() error { err := c.Conn.Close(); c.closeDone(); return err }

type sample struct {
	mu   sync.Mutex
	data []byte
	size int64
}

func (s *sample) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.size += int64(len(p))
	n := len(p)
	left := bodyLimit - len(s.data)
	if left > 0 {
		if len(p) > left {
			p = p[:left]
		}
		s.data = append(s.data, p...)
	}
	return n, nil
}

func (s *sample) snapshot() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return base64.StdEncoding.EncodeToString(s.data), s.size > bodyLimit
}

type sampledBody struct {
	io.ReadCloser
	s *sample
}

func (b *sampledBody) Read(p []byte) (int, error) {
	n, e := b.ReadCloser.Read(p)
	b.s.Write(p[:n])
	return n, e
}
func headers(h http.Header) string {
	keys := make([]string, 0, len(h))
	for k := range h {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	for _, k := range keys {
		for _, v := range h[k] {
			b.WriteString(clip(k, 256))
			b.WriteString(": ")
			b.WriteString(clip(v, headerLimit))
			b.WriteString("\n")
			if b.Len() >= headerLimit {
				return clip(b.String(), headerLimit)
			}
		}
	}
	return b.String()
}
func (m *Manager) serve(conn net.Conn, cfg Config, gen uint64, uid, port int, host string, dial Dial, roots *x509.CertPool) {
	transport := &http.Transport{Proxy: nil, DialContext: func(ctx context.Context, network, address string) (net.Conn, error) { return dial(ctx, address) }, TLSClientConfig: &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12}, DisableCompression: true, ForceAttemptHTTP2: true, TLSHandshakeTimeout: 15 * time.Second, ResponseHeaderTimeout: 30 * time.Second, ExpectContinueTimeout: time.Second, MaxResponseHeaderBytes: 32768, MaxIdleConnsPerHost: 2, IdleConnTimeout: 30 * time.Second}
	defer transport.CloseIdleConnections()
	handler := http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
		_, _, epoch := m.snapshot()
		scheme := "http"
		if port == 443 {
			scheme = "https"
		}
		if req.Method == "CONNECT" || (req.ProtoMajor == 1 && req.URL.IsAbs()) || (port == 443 && hostname(req.Host) != hostname(host)) {
			http.Error(w, "Unsupported capture request", http.StatusMisdirectedRequest)
			return
		}
		path := req.URL.EscapedPath()
		if path == "" {
			path = "/"
		}
		capture := cfg.MatchDomain(req.Host) && cfg.MatchPath(path)
		started := time.Now()
		record := Record{Time: nowMillis(), UID: uid, Method: clip(req.Method, 32), URL: clip(scheme+"://"+req.Host+req.URL.RequestURI(), 2048), BodyEncoding: "base64"}
		var requestSample, responseSample sample
		if capture {
			record.RequestHeaders = "Host: " + clip(req.Host, 512) + "\n" + headers(req.Header)
		}
		defer func() {
			if capture {
				record.Duration = time.Since(started).Milliseconds()
				record.RequestBody, record.RequestTruncated = requestSample.snapshot()
				record.ResponseBody, record.ResponseTruncated = responseSample.snapshot()
				m.add(record, gen, epoch)
			}
		}()
		out := req.Clone(req.Context())
		out.RequestURI = ""
		out.Trailer = req.Trailer
		out.URL.Scheme = scheme
		out.URL.Host = req.Host
		upgrade := req.Header.Get("Upgrade")
		stripHop(out.Header)
		if upgrade != "" {
			out.Header.Set("Connection", "Upgrade")
			out.Header.Set("Upgrade", upgrade)
		}
		if capture && out.Body != nil && out.Body != http.NoBody {
			out.Body = &sampledBody{ReadCloser: out.Body, s: &requestSample}
		}
		resp, err := transport.RoundTrip(out)
		if err != nil {
			record.Status = 502
			record.Error = clip(err.Error(), 512)
			http.Error(w, "Capture upstream request failed", 502)
			return
		}
		defer resp.Body.Close()
		record.Status = resp.StatusCode
		if capture {
			record.ResponseHeaders = headers(resp.Header)
		}
		if resp.StatusCode == http.StatusSwitchingProtocols {
			// Only the HTTP upgrade handshake is recorded; frames remain a byte stream.
			rw, ok := resp.Body.(io.ReadWriteCloser)
			if !ok {
				http.Error(w, "Invalid upgrade", 502)
				return
			}
			hijacker, ok := w.(http.Hijacker)
			if !ok {
				http.Error(w, "Upgrade unavailable", 502)
				return
			}
			downstream, buffer, err := hijacker.Hijack()
			if err != nil {
				return
			}
			defer downstream.Close()
			fmt.Fprintf(buffer, "HTTP/1.1 101 Switching Protocols\r\n")
			resp.Header.Write(buffer)
			buffer.WriteString("\r\n")
			buffer.Flush()
			done := make(chan struct{})
			go func() { io.Copy(rw, buffer); rw.Close(); close(done) }()
			io.Copy(downstream, rw)
			downstream.Close()
			<-done
			return
		}
		stripHop(resp.Header)
		for k, v := range resp.Header {
			w.Header()[k] = v
		}
		trailerKeys := make([]string, 0, len(resp.Trailer))
		for k := range resp.Trailer {
			trailerKeys = append(trailerKeys, k)
		}
		if len(trailerKeys) > 0 {
			w.Header().Set("Trailer", strings.Join(trailerKeys, ", "))
		}
		w.WriteHeader(resp.StatusCode)
		var dst io.Writer = &flushingWriter{w}
		if capture {
			dst = io.MultiWriter(dst, &responseSample)
		}
		_, err = io.Copy(dst, resp.Body)
		for k, v := range resp.Trailer {
			w.Header()[http.TrailerPrefix+k] = v
		}
		if err != nil {
			record.Error = clip(err.Error(), 512)
		}
	})
	listener := &singleListener{conn: conn, done: make(chan struct{})}
	server := &http.Server{Handler: handler, ReadHeaderTimeout: 15 * time.Second, IdleTimeout: 30 * time.Second, MaxHeaderBytes: 32768}
	defer server.Close()
	if secure, ok := conn.(*tls.Conn); ok && secure.ConnectionState().NegotiatedProtocol == "h2" {
		h2 := &http2.Server{MaxConcurrentStreams: 16, IdleTimeout: 30 * time.Second}
		h2.ServeConn(conn, &http2.ServeConnOpts{BaseConfig: server, Handler: handler})
		return
	}
	_ = server.Serve(listener)
}

type flushingWriter struct{ w http.ResponseWriter }

func (f *flushingWriter) Write(p []byte) (int, error) {
	n, err := f.w.Write(p)
	if v, ok := f.w.(http.Flusher); ok {
		v.Flush()
	}
	return n, err
}
