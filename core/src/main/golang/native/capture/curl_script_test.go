package capture

import (
	"encoding/json"
	"os/exec"
	"strings"
	"testing"
)

func TestCurlScriptOrderSkipAndContinue(t *testing.T) {
	rows := []Record{
		{ID: 1, Time: 300, Method: "GET", URL: "https://example.com/last"},
		{ID: 2, Time: 100, Method: "GET", URL: "https://example.com/fail"},
		{ID: 3, Time: 200, Method: "GET", URL: "https://example.com/'$(printf injected)"},
		{ID: 4, Time: 400, Method: "POST", URL: "https://example.com/skip", RequestTruncated: true},
	}
	script, copied, skipped := curlScript(rows)
	if copied != 3 || skipped != 1 || !strings.HasPrefix(script, "#!/bin/sh\n") {
		t.Fatal(copied, skipped)
	}
	syntax := exec.Command("sh", "-n")
	syntax.Stdin = strings.NewReader(script)
	if err := syntax.Run(); err != nil {
		t.Fatal(err)
	}
	// Fake curl prints the URL and fails the first call. No real requests run.
	fake := `curl() { while [ "$#" -gt 0 ]; do if [ "$1" = "--url" ]; then shift; printf '%s\n' "$1"; case "$1" in */fail) return 7;; esac; fi; shift; done; }
`
	out, err := exec.Command("sh", "-c", fake+script).Output()
	if e, ok := err.(*exec.ExitError); !ok || e.ExitCode() != 1 {
		t.Fatalf("expected failed-request status, got %v", err)
	}
	want := rows[1].URL + "\n" + rows[2].URL + "\n\n" + rows[0].URL + "\n\n"
	if string(out) != want {
		t.Fatalf("order/escaping/continue: %q", out)
	}
	if !strings.Contains(script, "Skipped record 4") {
		t.Fatal("missing skip explanation")
	}
}

func TestCurlScriptSnapshotChunks(t *testing.T) {
	m := New(t.TempDir())
	m.records = []Record{{ID: 1, Method: "GET", URL: "https://example.com/", RequestHeaders: "X-Test: " + strings.Repeat("汉", 20000)}}
	want, _, _ := curlScript(m.records)
	start, err := m.command("curl-script-start", "")
	if err != nil {
		t.Fatal(err)
	}
	info := start.(map[string]any)
	token := info["token"].(uint64)
	// New traffic can evict all records without changing the export snapshot.
	m.records = nil
	var b strings.Builder
	for offset := 0; offset < info["length"].(int); {
		payload, _ := json.Marshal(map[string]any{"token": token, "offset": offset})
		result, err := m.command("curl-script-part", string(payload))
		if err != nil {
			t.Fatal(err)
		}
		part := result.(map[string]any)
		b.WriteString(part["text"].(string))
		offset = part["next"].(int)
	}
	if b.String() != want {
		t.Fatal("snapshot or Unicode changed across chunk boundaries")
	}
	payload, _ := json.Marshal(map[string]any{"token": token})
	if _, err = m.command("curl-script-end", string(payload)); err != nil {
		t.Fatal(err)
	}
	if _, err = m.command("curl-script-part", string(payload)); err == nil {
		t.Fatal("released snapshot readable")
	}
	if _, err = m.command("curl-script-start", ""); err == nil {
		t.Fatal("empty capture accepted")
	}
}
