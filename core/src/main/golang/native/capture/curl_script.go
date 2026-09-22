package capture

import (
	"fmt"
	"sort"
	"strings"
)

func curlScript(records []Record) (string, int, int) {
	rows := append([]Record(nil), records...)
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].Time < rows[j].Time })
	var b strings.Builder
	b.WriteString("#!/bin/sh\n# Saved capture requests, oldest first. Run with: sh requests.sh\n# Executing this script replays requests and may repeat server-side changes.\ncommand -v curl >/dev/null 2>&1 || exit 127\ncommand -v base64 >/dev/null 2>&1 || exit 127\nfailures=0\n\n")
	copied, skipped := 0, 0
	for _, r := range rows {
		cmd, err := curlCommand(r)
		if err != nil {
			fmt.Fprintf(&b, "# Skipped record %d: %s\n\n", r.ID, err)
			skipped++
			continue
		}
		copied++
		cmd = strings.Replace(cmd, "curl --globoff", "curl --globoff --connect-timeout 15 --max-time 120", 1)
		fmt.Fprintf(&b, "# Record %d\nprintf '%%s\\n' %s >&2\nif %s; then\n  printf '\\n'\nelse\n  code=$?\n  failures=$((failures + 1))\n  printf 'Request %d failed: curl exit %%s\\n' \"$code\" >&2\nfi\n\n", r.ID, shellQuote(fmt.Sprintf("[%d] %s %s", r.ID, r.Method, r.URL)), cmd, r.ID)
	}
	fmt.Fprintf(&b, "# Included: %d; skipped: %d\n[ \"$failures\" -eq 0 ]\n", copied, skipped)
	return b.String(), copied, skipped
}
