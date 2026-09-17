package capture

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPathRegexSemanticsAndReload(t *testing.T) {
	c := Config{Paths: []string{"/exact", "/api/*", " re:(?i)scan ", `re:^/v[0-9]+/(login|logout)/?$`}}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{"/exact", "/api/one", "/v2/login", "/v12/logout/", "/qr/SCAN/result", "/scanner"} {
		if !c.MatchPath(path) {
			t.Errorf("did not match %q", path)
		}
	}
	for _, path := range []string{"/exact/", "/api", "/v2/login/extra", "/v2/LOGIN", "/qr/%73can"} {
		if c.MatchPath(path) {
			t.Errorf("unexpected match %q", path)
		}
	}
	m := New(t.TempDir())
	if err := m.update(c); err != nil {
		t.Fatal(err)
	}
	reloaded := New(m.dir)
	reloaded.Init(m.dir)
	saved, _, _ := reloaded.snapshot()
	if !saved.MatchPath("/qr/SCAN/result") || saved.MatchPath("/v2/login/extra") {
		t.Fatal("persisted regex did not reload")
	}
	// Revalidation must discard old compiled patterns, and internals must not
	// leak into the JSON configuration used by Android.
	c.Paths = []string{"re:^/new$"}
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
	if len(c.pathRegexps) != 1 || !c.MatchPath("/new") || c.MatchPath("/scan") {
		t.Fatal("stale regex")
	}
	data, err := json.Marshal(c)
	if err != nil || strings.Contains(string(data), "pathRegexps") {
		t.Fatal(string(data), err)
	}
}

func TestInvalidPathRegexKeepsActiveConfig(t *testing.T) {
	m := New(t.TempDir())
	if err := m.update(Config{Enabled: true, Paths: []string{"re:scan"}}); err != nil {
		t.Fatal(err)
	}
	for _, rule := range []string{"re:", "re:[", `re:(?=scan)`, `re:(scan)\1`, "re:x\ny", "re:" + strings.Repeat("x", 2048)} {
		if err := m.update(Config{Enabled: true, Paths: []string{rule}}); err == nil {
			t.Fatalf("accepted invalid regex %q", rule)
		}
		cfg, _, _ := m.snapshot()
		if !cfg.Enabled || !cfg.MatchPath("/scan") || cfg.MatchPath("/other") {
			t.Fatal("invalid rule replaced active config")
		}
	}
}
