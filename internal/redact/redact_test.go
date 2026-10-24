package redact

import (
	"strings"
	"testing"
)

func TestText(t *testing.T) {
	text := Text("known value: runtime-value\nTOKEN=private-token\nAuthorization: Bearer abc\npostgres://user:pass@host/db\n-----BEGIN PRIVATE KEY-----\nkey-data\n-----END PRIVATE KEY-----", "runtime-value")
	for _, v := range []string{"runtime-value", "private-token", "abc", "user:pass", "key-data"} {
		if strings.Contains(text, v) {
			t.Fatalf("leaked %q: %s", v, text)
		}
	}
}

func TestStreamingPEMAndKnownValues(t *testing.T) {
	f := LineFilter{Values: []string{"split-secret"}}
	var output string
	for _, line := range []string{"-----BEGIN PRIVATE KEY-----", "short-key-data", "-----END PRIVATE KEY-----", "printed split-secret"} {
		output += f.Text(line) + "\n"
	}
	if strings.Contains(output, "short-key-data") || strings.Contains(output, "split-secret") {
		t.Fatal(output)
	}
	if f.Text("normal startup") != "normal startup" {
		t.Fatal("PEM suppression did not end")
	}
}
