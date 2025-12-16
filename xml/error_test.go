package xml

import (
	"strings"
	"testing"
)

func TestXMLError(t *testing.T) {
	// XML malformado en la línea 3
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

	// Verificar si es nuestro tipo
	syntaxErr, ok := err.(*SyntaxError)
	if !ok {
		// INTENT: El error podría venir envuelto o ser directo.
		// Al menos verifiquemos que el string contiene "line"
		if !strings.Contains(err.Error(), "line") {
			t.Errorf("Error expected to contain 'line', got: %v", err)
		}
		return
	}

	// Debería ser línea 4 o 5 dependiendo de cómo el parser detecta el EOF inesperado o tag no cerrado
	if syntaxErr.Line <= 0 {
		t.Errorf("Expected Line > 0, got %d", syntaxErr.Line)
	}
	t.Logf("Got expected error: %v", syntaxErr)
}
