package xml

import (
	"io"
	"strings"
	"testing"
)

func TestSanitizeSoup(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "Basic Script Wrapping",
			input:    `<script>console.log("hello")</script>`,
			expected: `<script><![CDATA[console.log("hello")]]></script>`,
		},
		{
			name:     "Script with Attributes",
			input:    `<script type="text/javascript" src="app.js">var x=1;</script>`,
			expected: `<script type="text/javascript" src="app.js"><![CDATA[var x=1;]]></script>`,
		},
		{
			name:     "Case Insensitive Tags",
			input:    `<STYLE>body { color: red; }</STYLE>`,
			expected: `<STYLE><![CDATA[body { color: red; }]]></STYLE>`,
		},
		{
			name:     "Multiline Content",
			input:    "<pre>\nLine 1\nLine 2\n</pre>",
			expected: "<pre><![CDATA[\nLine 1\nLine 2\n]]></pre>",
		},
		{
			name:     "Textarea with Attributes",
			input:    `<textarea name="msg" cols="50">User input</textarea>`,
			expected: `<textarea name="msg" cols="50"><![CDATA[User input]]></textarea>`,
		},
		{
			name:     "Code Tag",
			input:    `<code>if (a < b)</code>`,
			expected: `<code><![CDATA[if (a < b)]]></code>`,
		},
		{
			name:     "Ignores Normal Tags",
			input:    `<div><span>Hello</span></div>`,
			expected: `<div><span>Hello</span></div>`,
		},
		// === EL CASO CRÍTICO ===
		{
			name:  "Escape CDATA Closer (]]>)",
			input: `<script>var s = "]]>";</script>`,
			// Explicación: "]]>" se rompe en "]]]]><![CDATA[>"
			// Visualmente: <![CDATA[ var s = " ]] ]]><![CDATA[ > "; ]]>
			expected: `<script><![CDATA[var s = "]]]]><![CDATA[>";]]></script>`,
		},
		{
			name: "Complex Mixed Content",
			input: `<html>
<head>
	<script>
		if (a < b) alert("valid");
	</script>
</head>
<body>
	<div>Normal Content</div>
	<style>
		div > span { content: "]]>"; }
	</style>
</body>
</html>`,
			expected: `<html>
<head>
	<script><![CDATA[
		if (a < b) alert("valid");
	]]></script>
</head>
<body>
	<div>Normal Content</div>
	<style><![CDATA[
		div > span { content: "]]]]><![CDATA[>"; }
	]]></style>
</body>
</html>`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Ejecutamos la función
			reader := strings.NewReader(tt.input)
			outputReader := sanitizeSoup(reader)

			// Leemos el resultado completo
			outputBytes, err := io.ReadAll(outputReader)
			if err != nil {
				t.Fatalf("Failed to read output: %v", err)
			}
			got := string(outputBytes)

			// Verificamos
			if got != tt.expected {
				t.Errorf("Sanitization mismatch.\n--- Input ---\n%s\n--- Expected ---\n%s\n--- Got ---\n%s",
					tt.input, tt.expected, got)
			}
		})
	}
}

// TestSanitizeSoup_ZeroAlloc verifies that we handle empty or nil inputs gracefully
func TestSanitizeSoup_EdgeCases(t *testing.T) {
	t.Run("Empty Input", func(t *testing.T) {
		res := sanitizeSoup(strings.NewReader(""))
		b, _ := io.ReadAll(res)
		if len(b) != 0 {
			t.Error("Expected empty output for empty input")
		}
	})

	t.Run("No Matching Tags", func(t *testing.T) {
		in := "just plain text"
		res := sanitizeSoup(strings.NewReader(in))
		b, _ := io.ReadAll(res)
		if string(b) != in {
			t.Error("Content should be untouched if no tags match")
		}
	})
}
