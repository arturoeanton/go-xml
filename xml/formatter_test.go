package xml

import (
	"strings"
	"testing"
)

func TestFormat_Pretty(t *testing.T) {
	dirty := `<root>  <child>  content  </child></root>`

	// We haven't implemented a standalone Format(bytes) function in `xml/formatter.go` yet as per plan
	// But we have `Marshal` with `WithPrettyPrint`.
	// The implementation plan mentioned `formatter.go`. Let's check if we implemented it or if we should test `Marshal`.
	// The user manually implemented CLI `fmt` command using `MapXML` + `Encoder`.
	// So we can test that flow.

	r := strings.NewReader(dirty)
	m, err := MapXML(r)
	if err != nil {
		t.Fatalf("Parse failed: %v", err)
	}

	out, err := Marshal(m, WithPrettyPrint())
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	if !strings.Contains(out, "\n  <child>") {
		t.Errorf("Expected indentation in output: %s", out)
	}
}
