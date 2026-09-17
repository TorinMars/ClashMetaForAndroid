// Package capture implements opt-in, local HTTP inspection for the Android VPN.
package capture

import (
	"errors"
	"fmt"
	"net"
	"strings"
)

type Config struct {
	Enabled   bool     `json:"enabled"`
	HTTPS     bool     `json:"https"`
	TLS12Only bool     `json:"tls12Only"`
	Domains   []string `json:"domains"`
	Paths     []string `json:"paths"`
	Packages  []string `json:"packages"`
	UIDs      []int    `json:"uids"`
}

func (c *Config) Validate() error {
	if len(c.Domains) > 100 || len(c.Paths) > 100 || len(c.Packages) > 100 || len(c.UIDs) > 100 {
		return errors.New("每类白名单最多 100 项")
	}
	for i, s := range c.Domains {
		s = strings.ToLower(strings.TrimSuffix(strings.TrimSpace(s), "."))
		base := strings.TrimPrefix(s, "*.")
		if len(base) == 0 || len(base) > 253 || strings.ContainsAny(base, "/*:?#@ \\\r\n") {
			return fmt.Errorf("无效域名：%s", s)
		}
		for _, label := range strings.Split(base, ".") {
			if len(label) == 0 || len(label) > 63 || strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
				return fmt.Errorf("无效域名：%s", s)
			}
			for _, ch := range label {
				if !(ch >= 'a' && ch <= 'z' || ch >= '0' && ch <= '9' || ch == '-') {
					return errors.New("域名请使用 ASCII / Punycode")
				}
			}
		}
		c.Domains[i] = s
	}
	for i, s := range c.Paths {
		s = strings.TrimSpace(s)
		if !strings.HasPrefix(s, "/") || len(s) > 2048 || strings.ContainsAny(s, "?#\r\n") || strings.Contains(strings.TrimSuffix(s, "*"), "*") {
			return fmt.Errorf("无效 path（支持末尾 *）：%s", s)
		}
		c.Paths[i] = s
	}
	for _, uid := range c.UIDs {
		if uid < 0 {
			return errors.New("无效应用 UID")
		}
	}
	if len(c.Packages) > 0 && len(c.UIDs) == 0 {
		return errors.New("所选应用不可用，请重新选择")
	}
	return nil
}

func hostname(s string) string {
	if h, _, err := net.SplitHostPort(s); err == nil {
		s = h
	}
	return strings.ToLower(strings.TrimSuffix(s, "."))
}
func (c Config) MatchDomain(host string) bool {
	host = hostname(host)
	if host == "" {
		return false
	}
	if len(c.Domains) == 0 {
		return true
	}
	for _, d := range c.Domains {
		if host == d {
			return true
		}
		if strings.HasPrefix(d, "*.") && strings.HasSuffix(host, d[1:]) && host != d[2:] {
			return true
		}
	}
	return false
}
func (c Config) MatchPath(path string) bool {
	if len(c.Paths) == 0 {
		return true
	}
	for _, p := range c.Paths {
		if p == path || strings.HasSuffix(p, "*") && strings.HasPrefix(path, strings.TrimSuffix(p, "*")) {
			return true
		}
	}
	return false
}
func (c Config) MatchUID(uid int) bool {
	if len(c.Packages) == 0 && len(c.UIDs) == 0 {
		return true
	}
	for _, u := range c.UIDs {
		if uid == u {
			return true
		}
	}
	return false
}
