package capture

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Diagnostics contain bounded connection metadata, never headers or bodies.
// Generation/epoch checks prevent a closed connection repopulating cleared data.
func (m *Manager) note(gen, epoch uint64, reason string, uid, port int, host string, detail ...string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.config.Enabled || gen != m.generation || epoch != m.epoch {
		return
	}
	m.countLocked(reason)
	line := fmt.Sprintf("%s · UID %d · :%d · %s", time.Now().Format("15:04:05.000"), uid, port, reason)
	if host != "" {
		line += " · " + clip(host, 253)
	}
	if len(detail) > 0 {
		line += " · " + clip(strings.Join(detail, " · "), 512)
	}
	if len(m.diagnosticEvents) == 200 {
		copy(m.diagnosticEvents, m.diagnosticEvents[1:])
		m.diagnosticEvents = m.diagnosticEvents[:199]
	}
	m.diagnosticEvents = append(m.diagnosticEvents, line)
}

func (m *Manager) countLocked(reason string) {
	if m.diagnosticCounts == nil {
		m.diagnosticCounts = make(map[string]uint64)
	}
	m.diagnosticCounts[reason]++
}

// UDP still passes through untouched. UID lookup is sampled at most once per
// second, avoiding a JNI lookup per packet. Packet totals include all apps.
func (m *Manager) ObserveUDP(port int, queryUID func() int) {
	m.mu.Lock()
	if !m.config.Enabled {
		m.mu.Unlock()
		return
	}
	m.countLocked("UDP 数据包（全部应用）")
	if port == 443 {
		m.countLocked("UDP 443 数据包（可能为 QUIC）")
	}
	now := time.Now()
	if now.Sub(m.lastUDPSample) < time.Second {
		m.mu.Unlock()
		return
	}
	m.lastUDPSample = now
	gen, epoch := m.generation, m.epoch
	m.mu.Unlock()
	m.note(gen, epoch, "UDP 抽样：不支持解密，已放行", queryUID(), port, "")
}

func (m *Manager) resetDiagnosticsLocked() {
	m.diagnosticCounts = nil
	m.diagnosticEvents = nil
	m.lastUDPSample = time.Time{}
}

func (m *Manager) diagnosticsLocked(limit int) string {
	var b strings.Builder
	if m.routing.Mode != "" {
		fmt.Fprintf(&b, "抓包独立接管：原分应用路由 %s，原路由 UID %v；额外接管应用直连，无法识别 UID 时拒绝连接。\n", m.routing.Mode, m.routing.UIDs)
	}
	fmt.Fprintf(&b, "应用白名单 UID：%v（空表示全部，-1 表示未识别）\n域名规则：%d · Path 规则：%d\n", m.config.UIDs, len(m.config.Domains), len(m.config.Paths))
	if len(m.diagnosticCounts) == 0 {
		b.WriteString("尚未观察到新的 TUN TCP 连接或 UDP 数据包。请重启目标应用后操作。\n")
	}
	keys := make([]string, 0, len(m.diagnosticCounts))
	for key := range m.diagnosticCounts {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		fmt.Fprintf(&b, "%s：%d\n", key, m.diagnosticCounts[key])
	}
	shown := limit
	if shown > len(m.diagnosticEvents) {
		shown = len(m.diagnosticEvents)
	}
	fmt.Fprintf(&b, "最近事件（显示 %d / 保留 %d 条；UDP UID 每秒抽样，不能代表全部 UDP 流量）：\n", shown, len(m.diagnosticEvents))
	for i := len(m.diagnosticEvents) - 1; i >= 0 && i >= len(m.diagnosticEvents)-limit; i-- {
		b.WriteString(m.diagnosticEvents[i] + "\n")
	}
	return b.String()
}
