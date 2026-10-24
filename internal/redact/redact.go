// Package redact sanitizes untrusted diagnostics before persistence or display.
package redact

import (
	"encoding/base64"
	"encoding/json"
	"net/url"
	"regexp"
	"sort"
	"strings"
)

var pem = regexp.MustCompile(`(?s)-----BEGIN [^-]+-----.*?(-----END [^-]+-----|$)`)
var assignment = regexp.MustCompile(`(?im)^.*(?:password|passwd|secret|token|credential|authorization|api[_-]?key|private[_-]?key|vapid|cookie).*[:=].*$`)
var bearer = regexp.MustCompile(`(?i)Bearer\s+[^\s"']+`)
var userinfo = regexp.MustCompile(`(https?|redis|postgres(?:ql)?|amqp)://[^\s/@]+@`)
var opaque = regexp.MustCompile(`[A-Za-z0-9_+/=-]{32,}`)

// Text masks known runtime values (including common encodings) and likely credentials.
func Text(text string, values ...string) string {
	var known []string
	for _, v := range values {
		if v == "" {
			continue
		}
		for _, line := range strings.Split(v, "\n") {
			if line != "" {
				known = append(known, line)
			}
		}
		known = append(known, strings.Trim(v, "\"'"), v, strings.ReplaceAll(v, "$$", "$"), url.QueryEscape(v), base64.StdEncoding.EncodeToString([]byte(v)))
		b, _ := json.Marshal(v)
		if len(b) > 2 {
			known = append(known, string(b[1:len(b)-1]))
		}
	}
	sort.Slice(known, func(i, j int) bool { return len(known[i]) > len(known[j]) })
	for _, v := range known {
		if v == "" {
			continue
		}
		text = strings.ReplaceAll(text, v, "[REDACTED]")
	}
	text = pem.ReplaceAllString(text, "[REDACTED PRIVATE KEY]")
	text = assignment.ReplaceAllString(text, "[REDACTED sensitive diagnostic]")
	text = bearer.ReplaceAllString(text, "Bearer [REDACTED]")
	text = userinfo.ReplaceAllString(text, "$1://[REDACTED]@")
	return opaque.ReplaceAllString(text, "[REDACTED]")
}

// LineFilter redacts complete streaming log lines, withholding multiline PEM blocks.
type LineFilter struct {
	Values []string
	inPEM  bool
}

func (f *LineFilter) Text(line string) string {
	if strings.Contains(line, "-----BEGIN ") {
		f.inPEM = true
	}
	if f.inPEM {
		if strings.Contains(line, "-----END ") {
			f.inPEM = false
		}
		return "[REDACTED PRIVATE KEY]"
	}
	return Text(line, f.Values...)
}
