package xml

import (
	"strings"
	"testing"
)

func TestXMLError(t *testing.T) {
	// Malformed XML at line 3
	malformed := `
<root>
	<valid>ok</valid>
	<broken>oops
</root>`

	r := strings.NewReader(malformed)
	_, err := MapXML(r)

	if err == nil {
		t.Fatal("Expected error, got nil")
	}

	// Check whether it is our type
	syntaxErr, ok := err.(*SyntaxError)
	if !ok {
		// INTENT: The error could come wrapped or be direct.
		// At least verify that the string contains "line"
		if !strings.Contains(err.Error(), "line") {
			t.Errorf("Error expected to contain 'line', got: %v", err)
		}
		return
	}

	// It should be line 4 or 5 depending on how the parser detects the unexpected EOF or unclosed tag
	if syntaxErr.Line <= 0 {
		t.Errorf("Expected Line > 0, got %d", syntaxErr.Line)
	}
	t.Logf("Got expected error: %v", syntaxErr)
}
