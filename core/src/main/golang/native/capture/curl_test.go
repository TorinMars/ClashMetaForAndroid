package capture

import (
	"bytes"
	"encoding/base64"
	"os/exec"
	"strings"
	"testing"
)

func TestCurlShellQuotingAndHeaders(t *testing.T) {
	r := Record{Method: "GET", URL: "https://example.com/scan?q='$(printf injected)&a[]=1", RequestHeaders: "Host: example.com\nX-Test: '$(printf injected)`printf injected`\nX-Empty: \nContent-Length: 999\nConnection: X-Hop\nX-Hop: remove\n"}
	cmd, err := curlCommand(r)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cmd, "Content-Length") || strings.Contains(cmd, "X-Hop") {
		t.Fatal(cmd)
	}
	// Substitute a shell function for curl: no network, and verify exact argv
	// after shell parsing, including quotes and command-substitution syntax.
	out, err := exec.Command("sh", "-c", "curl() { printf '%s\\n' \"$@\"; }; "+cmd).Output()
	if err != nil {
		t.Fatal(err)
	}
	want := "--globoff\n--request\nGET\n--url\n" + r.URL + "\n--header\nHost: example.com\n--header\nX-Test: '$(printf injected)`printf injected`\n--header\nX-Empty;\n"
	if string(out) != want {
		t.Fatalf("argv mismatch: %q", out)
	}
}

func TestCurlBinaryBodyAndTruncation(t *testing.T) {
	body := []byte{'a', 0, '\'', 255, '\n', '\n'}
	r := Record{Method: "POST", URL: "https://example.com/", RequestBody: base64.StdEncoding.EncodeToString(body)}
	cmd, err := curlCommand(r)
	if err != nil {
		t.Fatal(err)
	}
	out, err := exec.Command("sh", "-c", "curl() { cat; }; "+cmd).Output()
	if err != nil || !bytes.Equal(out, body) {
		t.Fatalf("body=%x err=%v", out, err)
	}
	r.RequestTruncated = true
	if _, err = curlCommand(r); err == nil {
		t.Fatal("accepted truncated body")
	}
	r.RequestTruncated = false
	r.RequestHeaders = "X-Test: clipped…\n"
	if _, err = curlCommand(r); err == nil {
		t.Fatal("accepted truncated headers")
	}
}
