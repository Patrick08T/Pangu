package diagnostic

import (
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

var credential = regexp.MustCompile(`(?i)(bearer\s+|(?:token|api[_-]?key|secret|password)["']?\s*[=:]\s*)("(?:\\.|[^"\\])*"|'(?:\\.|[^'\\])*'|[^\s"'&,;]+)`)
var userinfo = regexp.MustCompile(`(https?://)[^\s/@:]+:[^\s/@]+@`)

// Text 在截断前脱敏；配置值始终直接从进程环境读取。
func Text(value string, limit int) (string, bool) {
	value = redact(value)
	value = strings.ToValidUTF8(value, "�")
	if len(value) <= limit {
		return value, false
	}
	value = value[:max(0, limit)]
	for !utf8.ValidString(value) {
		value = value[:len(value)-1]
	}
	return value, true
}

func Sensitive(value []byte) bool {
	text := string(value)
	return redact(text) != text || strings.Contains(text, "PRIVATE KEY-----")
}

func redact(value string) string {
	for _, entry := range os.Environ() {
		name, secret, _ := strings.Cut(entry, "=")
		upper := strings.ToUpper(name)
		if len(secret) >= 8 && (strings.Contains(upper, "KEY") || strings.Contains(upper, "TOKEN") ||
			strings.Contains(upper, "SECRET") || strings.Contains(upper, "PASSWORD") || strings.Contains(upper, "DATABASE_URL")) {
			value = strings.ReplaceAll(value, secret, "[已脱敏]")
		}
	}
	value = credential.ReplaceAllString(value, "${1}[已脱敏]")
	value = userinfo.ReplaceAllString(value, "${1}[已脱敏]@")
	return value
}
