package xml

import (
	"bytes"
	"strings"
	"testing"
)

// ============================================================================
// ENCODER TESTS
// ============================================================================

func TestEncoder_Basic(t *testing.T) {
	// Estructura: <user id="101"><name>Alice</name><active>true</active></user>
	data := map[string]any{
		"user": map[string]any{
			"@id":    101,
			"name":   "Alice",
			"active": true,
		},
	}

	xmlStr, err := Marshal(data)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// El encoder ordena claves alfabéticamente: @id, active, name
	expected := `<user id="101"><active>true</active><name>Alice</name></user>`
	if xmlStr != expected {
		t.Errorf("Mismatch.\nGot:  %s\nWant: %s", xmlStr, expected)
	}
}

func TestEncoder_ArraysAndCDATA(t *testing.T) {
	// Estructura con lista repetida y CDATA
	data := map[string]any{
		"blog": map[string]any{
			"post": []any{
				map[string]any{
					"title":  "Intro to Go",
					"#cdata": "<p>Hello World</p>",
				},
				map[string]any{
					"title": "XML Parsing",
					"#text": "Just text",
				},
			},
		},
	}

	xmlStr, err := Marshal(data)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// Validaciones parciales porque el orden de los posts en el array es determinista (índice 0, índice 1)
	// pero las claves dentro del mapa se ordenan.
	if !strings.Contains(xmlStr, `<![CDATA[<p>Hello World</p>]]>`) {
		t.Error("CDATA not found or incorrect")
	}
	if !strings.Contains(xmlStr, `<title>Intro to Go</title>`) {
		t.Error("First post title missing")
	}
	if !strings.Contains(xmlStr, `<title>XML Parsing</title>`) {
		t.Error("Second post title missing")
	}
}

func TestEncoder_PrettyPrint(t *testing.T) {
	data := map[string]any{
		"root": map[string]any{
			"child": "value",
		},
	}

	var buf bytes.Buffer
	enc := NewEncoder(&buf, WithPrettyPrint())
	if err := enc.Encode(data); err != nil {
		t.Fatalf("Encode failed: %v", err)
	}

	output := buf.String()
	// Verificamos que haya saltos de línea y espacios
	if !strings.Contains(output, "\n  <child>value</child>") {
		t.Errorf("Pretty print failed. Got:\n%s", output)
	}
}

func TestEncoder_IgnoreMetadata(t *testing.T) {
	// ESTE ES EL TEST CRÍTICO PARA TU PROBLEMA ANTERIOR.
	// Simulamos un mapa sucio con #seq y #comments que el encoder DEBE ignorar.
	data := map[string]any{
		"config": map[string]any{
			"debug":     true,
			"#seq":      []any{"debug", "garbage"}, // Metadata interna
			"#comments": []string{"Ignore me"},     // Metadata interna
		},
	}

	xmlStr, err := Marshal(data)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	// No debe aparecer <#seq> ni <#comments> ni su contenido volcado
	if strings.Contains(xmlStr, "#seq") || strings.Contains(xmlStr, "garbage") {
		t.Errorf("Encoder leaked metadata keys! Output: %s", xmlStr)
	}

	expected := `<config><debug>true</debug></config>`
	if xmlStr != expected {
		t.Errorf("Output mismatch.\nGot:  %s\nWant: %s", xmlStr, expected)
	}
}

func TestEncoder_Errors(t *testing.T) {
	// Caso: Múltiples raíces (prohibido en XML estándar)
	data := map[string]any{
		"root1": "val",
		"root2": "val",
	}

	err := NewEncoder(&bytes.Buffer{}).Encode(data)
	if err == nil {
		t.Error("Expected error for multiple roots, got nil")
	} else if err.Error() != "root must have exactly 1 element" {
		t.Errorf("Incorrect error message: %v", err)
	}
}
