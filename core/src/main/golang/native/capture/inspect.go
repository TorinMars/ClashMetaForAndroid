package capture

import (
	"bufio"
	"bytes"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

type readerConn struct {
	net.Conn
	reader io.Reader
}

func (c *readerConn) Read(p []byte) (int, error) { return c.reader.Read(p) }

type probeConn struct {
	net.Conn
	read bytes.Buffer
}

func (c *probeConn) Read(p []byte) (int, error) {
	if c.read.Len() >= 65536 {
		return 0, errors.New("ClientHello too large")
	}
	if len(p) > 65536-c.read.Len() {
		p = p[:65536-c.read.Len()]
	}
	n, err := c.Conn.Read(p)
	c.read.Write(p[:n])
	return n, err
}

// The probe must never write a TLS alert or handshake to the real client.
func (c *probeConn) Write(p []byte) (int, error) { return len(p), nil }
func (c *probeConn) Close() error                { return nil }

func inspectTLS(conn net.Conn) (net.Conn, string, bool, string) {
	p := &probeConn{Conn: conn}
	host := ""
	supported := false
	detail := ""
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	err := tls.Server(p, &tls.Config{GetConfigForClient: func(hello *tls.ClientHelloInfo) (*tls.Config, error) {
		host = hello.ServerName
		detail = fmt.Sprintf("ClientHello ALPN=%v versions=%x", hello.SupportedProtos, hello.SupportedVersions)
		supported = len(hello.SupportedProtos) == 0
		for _, v := range hello.SupportedProtos {
			if v == "http/1.1" || v == "h2" {
				supported = true
			}
		}
		return nil, errors.New("inspection only")
	}}).Handshake()
	conn.SetReadDeadline(time.Time{})
	if detail == "" && err != nil {
		detail = err.Error()
	}
	return &readerConn{conn, io.MultiReader(bytes.NewReader(p.read.Bytes()), conn)}, host, supported, detail
}

func inspectHTTP(conn net.Conn) (net.Conn, string) {
	conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	br := bufio.NewReaderSize(conn, 32768)
	var prefix bytes.Buffer
	host := ""
	for prefix.Len() < 32768 {
		line, err := br.ReadSlice('\n')
		prefix.Write(line)
		if err != nil {
			break
		}
		if bytes.Equal(line, []byte("\r\n")) {
			if req, e := http.ReadRequest(bufio.NewReader(bytes.NewReader(prefix.Bytes()))); e == nil {
				host = req.Host
			}
			break
		}
	}
	conn.SetReadDeadline(time.Time{})
	return &readerConn{conn, io.MultiReader(bytes.NewReader(prefix.Bytes()), br)}, hostname(host)
}

func stripHop(h http.Header) {
	for _, v := range h.Values("Connection") {
		for _, key := range strings.Split(v, ",") {
			h.Del(strings.TrimSpace(key))
		}
	}
	for _, key := range []string{"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate", "Proxy-Authorization", "TE", "Trailer", "Transfer-Encoding", "Upgrade"} {
		h.Del(key)
	}
}
