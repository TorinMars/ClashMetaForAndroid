package capture

import (
	"encoding/json"
	"errors"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

const bodyLimit = 8192
const headerLimit = 8192
const recordLimit = 100

type Record struct {
	ID                int64  `json:"id"`
	Time              int64  `json:"time"`
	UID               int    `json:"uid"`
	Method            string `json:"method"`
	URL               string `json:"url"`
	Status            int    `json:"status"`
	Duration          int64  `json:"durationMs"`
	RequestHeaders    string `json:"requestHeaders,omitempty"`
	ResponseHeaders   string `json:"responseHeaders,omitempty"`
	RequestBody       string `json:"requestBody,omitempty"`
	ResponseBody      string `json:"responseBody,omitempty"`
	RequestTruncated  bool   `json:"requestTruncated,omitempty"`
	ResponseTruncated bool   `json:"responseTruncated,omitempty"`
	BodyEncoding      string `json:"bodyEncoding,omitempty"`
	Error             string `json:"error,omitempty"`
}

type Manager struct {
	mu         sync.Mutex
	dir        string
	config     Config
	ca         *authority
	records    []Record
	next       int64
	generation uint64
	epoch      uint64
	active     map[net.Conn]struct{}
	slots      chan struct{}
	lastError  string
}

var Default = New("")

func New(dir string) *Manager {
	return &Manager{dir: dir, active: make(map[net.Conn]struct{}), slots: make(chan struct{}, 64)}
}
func (m *Manager) Init(dir string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.dir = dir
	var c Config
	if data, err := os.ReadFile(filepath.Join(dir, "config.json")); err == nil && json.Unmarshal(data, &c) == nil && c.Validate() == nil {
		c.Enabled = false
		m.config = c
	}
}
func (m *Manager) Enabled() bool { m.mu.Lock(); defer m.mu.Unlock(); return m.config.Enabled }
func (m *Manager) snapshot() (Config, uint64, uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.config, m.generation, m.epoch
}
func (m *Manager) update(c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	m.mu.Lock()
	if c.HTTPS && c.Enabled && m.ca == nil {
		var err error
		m.ca, err = loadAuthority(m.dir)
		if err != nil {
			m.mu.Unlock()
			return err
		}
	}
	saved := c
	saved.Enabled = false
	data, err := json.Marshal(saved)
	if err == nil {
		err = writePrivate(filepath.Join(m.dir, "config.json"), data)
	}
	if err != nil {
		m.mu.Unlock()
		return err
	}
	m.config = c
	m.generation++
	m.lastError = ""
	conns := m.connectionsLocked()
	m.mu.Unlock()
	for _, conn := range conns {
		conn.Close()
	}
	return nil
}
func (m *Manager) connectionsLocked() []net.Conn {
	var out []net.Conn
	for conn := range m.active {
		out = append(out, conn)
	}
	return out
}
func (m *Manager) Stop() {
	m.mu.Lock()
	m.config.Enabled = false
	m.generation++
	conns := m.connectionsLocked()
	m.mu.Unlock()
	for _, conn := range conns {
		conn.Close()
	}
}
func (m *Manager) register(conn net.Conn, gen uint64) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.config.Enabled || gen != m.generation {
		return false
	}
	m.active[conn] = struct{}{}
	return true
}
func (m *Manager) unregister(conn net.Conn) { m.mu.Lock(); delete(m.active, conn); m.mu.Unlock() }
func (m *Manager) add(r Record, gen, epoch uint64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.config.Enabled || m.generation != gen || m.epoch != epoch {
		return
	}
	m.next++
	r.ID = m.next
	if len(m.records) == recordLimit {
		copy(m.records, m.records[1:])
		m.records[len(m.records)-1] = r
	} else {
		m.records = append(m.records, r)
	}
}
func (m *Manager) fail(err error) { m.mu.Lock(); m.lastError = clip(err.Error(), 512); m.mu.Unlock() }
func clip(s string, n int) string {
	if len(s) > n {
		return s[:n] + "…"
	}
	return s
}
func (m *Manager) Command(command, payload string) string {
	result, err := m.command(command, payload)
	if err != nil {
		result = map[string]any{"error": err.Error()}
	}
	data, err := json.Marshal(result)
	if err != nil {
		return `{"error":"JSON encoding failed"}`
	}
	return string(data)
}
func (m *Manager) command(command, payload string) (any, error) {
	if command == "stop" {
		m.Stop()
		if payload != "" {
			m.fail(errors.New(clip(payload, 512)))
		}
		command = "state"
	}
	if command == "configure" {
		if len(payload) > 65536 {
			return nil, errors.New("配置过大")
		}
		var c Config
		if err := json.Unmarshal([]byte(payload), &c); err != nil {
			return nil, err
		}
		if err := m.update(c); err != nil {
			return nil, err
		}
		command = "state"
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	switch command {
	case "state":
		return map[string]any{"config": m.config, "count": len(m.records), "lastError": m.lastError}, nil
	case "clear":
		m.records = nil
		m.epoch++
		m.lastError = ""
		return map[string]bool{"ok": true}, nil
	case "ca":
		if m.ca == nil {
			var err error
			m.ca, err = loadAuthority(m.dir)
			if err != nil {
				return nil, err
			}
		}
		return map[string]string{"pem": string(m.ca.pem), "sha256": m.ca.fingerprint()}, nil
	case "list":
		rows := make([]Record, 0, len(m.records))
		for i := len(m.records) - 1; i >= 0; i-- {
			r := m.records[i]
			r.RequestHeaders = ""
			r.ResponseHeaders = ""
			r.RequestBody = ""
			r.ResponseBody = ""
			rows = append(rows, r)
		}
		return map[string]any{"records": rows, "lastError": m.lastError, "enabled": m.config.Enabled}, nil
	case "get":
		id, err := strconv.ParseInt(payload, 10, 64)
		if err != nil {
			return nil, err
		}
		for _, r := range m.records {
			if r.ID == id {
				return r, nil
			}
		}
		return nil, errors.New("记录已被清空或淘汰")
	default:
		return nil, errors.New("unknown capture command")
	}
}
func nowMillis() int64 { return time.Now().UnixNano() / int64(time.Millisecond) }
