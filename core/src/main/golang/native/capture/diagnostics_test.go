package capture

import (
	"encoding/json"
	"net"
	"strings"
	"sync"
	"testing"
)

func TestDiagnosticsBypassAndReset(t *testing.T) {
	m := New(t.TempDir())
	if err := m.update(Config{Enabled: true, UIDs: []int{42}}); err != nil {
		t.Fatal(err)
	}
	client, server := net.Pipe()
	defer client.Close()
	defer server.Close()
	// These paths must not read or close the caller's connection.
	for _, c := range []struct{ uid, port int }{{-1, 443}, {99, 443}, {42, 5222}, {42, 443}} {
		if m.Handle(server, c.uid, c.port, nil, nil, nil) {
			t.Fatal("bypass consumed connection")
		}
	}
	if m.diagnosticCounts["TCP 新连接"] != 4 || m.diagnosticCounts["应用 UID 未识别"] != 1 || m.diagnosticCounts["应用白名单未匹配，已放行"] != 2 || m.diagnosticCounts["不支持此 TCP 端口，已放行"] != 1 || m.diagnosticCounts["未开启 HTTPS 解密，已放行"] != 1 {
		t.Fatal(m.diagnosticCounts)
	}
	_, gen, epoch := m.snapshot()
	var wg sync.WaitGroup
	for i := 0; i < 250; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); m.note(gen, epoch, "test", 42, 443, "example.com") }()
	}
	wg.Wait()
	if len(m.diagnosticEvents) != 200 || m.diagnosticCounts["test"] != 250 {
		t.Fatal("diagnostics not bounded or count lost")
	}
	var result map[string]any
	if err := json.Unmarshal([]byte(m.Command("diagnostics", "")), &result); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(result["log"].(string), "example.com") {
		t.Fatal(result)
	}
	m.Command("clear", "")
	m.note(gen, epoch, "stale", 42, 443, "example.com")
	if len(m.diagnosticEvents) != 0 || len(m.diagnosticCounts) != 0 {
		t.Fatal("clear accepted stale event")
	}
}

func TestUDPDiagnosticsSampling(t *testing.T) {
	m := New(t.TempDir())
	lookups := 0
	lookup := func() int { lookups++; return 42 }
	m.ObserveUDP(443, lookup)
	if lookups != 0 {
		t.Fatal("lookup while disabled")
	}
	if err := m.update(Config{Enabled: true}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 100; i++ {
		m.ObserveUDP(443, lookup)
	}
	if lookups != 1 || m.diagnosticCounts["UDP 数据包（全部应用）"] != 100 || len(m.diagnosticEvents) != 1 {
		t.Fatal("UDP sampling failed")
	}
	m.Command("clear", "")
	m.ObserveUDP(53, lookup)
	if lookups != 2 || m.diagnosticCounts["UDP 数据包（全部应用）"] != 1 || m.diagnosticCounts["UDP 443 数据包（可能为 QUIC）"] != 0 {
		t.Fatal("UDP reset failed")
	}
}
