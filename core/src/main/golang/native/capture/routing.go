package capture

// Routing is an immutable snapshot of the original Android VPN policy for a
// TUN whose app scope was expanded for capture. It is independent of filters.
type Routing struct {
	Mode string `json:"mode"`
	UIDs []int  `json:"uids"`
}

func (r Routing) Proxy(uid int) string {
	if r.Mode == "" {
		return ""
	}
	// Never turn an unidentified originally-proxied app into a direct leak.
	if uid < 0 {
		return "REJECT"
	}
	listed := false
	for _, candidate := range r.UIDs {
		if candidate == uid {
			listed = true
			break
		}
	}
	if r.Mode == "AcceptSelected" && !listed || r.Mode == "DenySelected" && listed {
		return "DIRECT"
	}
	return ""
}

func (m *Manager) Routing() Routing {
	m.mu.Lock()
	defer m.mu.Unlock()
	return Routing{Mode: m.routing.Mode, UIDs: append([]int(nil), m.routing.UIDs...)}
}
