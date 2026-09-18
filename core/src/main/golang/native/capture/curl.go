package capture

import (
	"encoding/base64"
	"errors"
	"strings"
)

func shellQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }

// curlCommand targets a POSIX shell. Base64 piping preserves binary bodies,
// including NUL bytes and trailing newlines, without shell interpolation.
func curlCommand(r Record) (string, error) {
	if r.RequestTruncated || strings.HasSuffix(r.RequestHeaders, "…") || strings.HasSuffix(r.URL, "…") {
		return "", errors.New("请求内容已截断，无法生成完整 cURL")
	}
	if _, err := base64.StdEncoding.DecodeString(r.RequestBody); err != nil {
		return "", errors.New("请求正文编码无效")
	}
	skip := map[string]bool{"content-length": true, "transfer-encoding": true, "connection": true, "proxy-connection": true, "keep-alive": true, "te": true, "trailer": true, "upgrade": true, "proxy-authorization": true, "proxy-authenticate": true}
	lines := strings.Split(r.RequestHeaders, "\n")
	for _, line := range lines {
		if strings.HasSuffix(line, "…") {
			return "", errors.New("请求头已截断，无法生成完整 cURL")
		}
		key, value, ok := strings.Cut(line, ":")
		if ok && strings.EqualFold(key, "Connection") {
			for _, token := range strings.Split(value, ",") {
				skip[strings.ToLower(strings.TrimSpace(token))] = true
			}
		}
	}
	args := []string{"curl --globoff", "--request " + shellQuote(r.Method), "--url " + shellQuote(r.URL)}
	for _, line := range lines {
		key, value, ok := strings.Cut(line, ":")
		if !ok || skip[strings.ToLower(key)] {
			continue
		}
		// curl uses a semicolon to explicitly send an empty header.
		if strings.TrimSpace(value) == "" {
			line = key + ";"
		}
		args = append(args, "--header "+shellQuote(line))
	}
	prefix := ""
	if r.RequestBody != "" {
		prefix = "printf '%s' " + shellQuote(r.RequestBody) + " | base64 --decode | "
		args = append(args, "--data-binary @-")
	}
	return prefix + strings.Join(args, " \\\n  "), nil
}
