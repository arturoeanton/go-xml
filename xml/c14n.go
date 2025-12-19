package xml

import (
	"bytes"
	"fmt"
	"sort"
	"strings"
)

// Canonicalize transforma tu estructura (Map/OrderedMap) a bytes
// cumpliendo las reglas básicas de C14N (Orden atributos, sin self-closing).
func Canonicalize(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	if err := writeCanonical(&buf, v, ""); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func writeCanonical(buf *bytes.Buffer, v interface{}, tagName string) error {
	switch val := v.(type) {

	case *OrderedMap: // Soporte para tu estructura principal
		return writeMapCanonical(buf, val, tagName)

	case map[string]interface{}: // Soporte para mapas crudos
		// Convertimos a OrderedMap al vuelo para reutilizar lógica
		om := NewMap()
		for k, v := range val {
			om.Set(k, v)
		}
		return writeMapCanonical(buf, om, tagName)

	case []interface{}: // Arrays de elementos
		for _, item := range val {
			if err := writeCanonical(buf, item, tagName); err != nil {
				return err
			}
		}
		return nil

	case string:
		buf.WriteString(escapeText(val))
		return nil

	default:
		// Ints, Floats, etc.
		buf.WriteString(escapeText(fmt.Sprintf("%v", val)))
		return nil
	}
}

func writeMapCanonical(buf *bytes.Buffer, m *OrderedMap, tagName string) error {
	// 1. Si hay tagName, abrimos la etiqueta
	if tagName != "" {
		buf.WriteByte('<')
		buf.WriteString(tagName)
	}

	// 2. ATRIBUTOS: Paso Crítico de C14N (Ordenar alfabéticamente)
	var attrs []string
	keys := m.Keys() // Asumo que tu OrderedMap tiene método Keys()

	// Separar atributos (empiezan con @) del contenido
	for _, k := range keys {
		if strings.HasPrefix(k, "@") {
			attrs = append(attrs, k)
		}
	}
	sort.Strings(attrs) // <--- LA MAGIA: Orden Alfabético

	for _, k := range attrs {
		val := m.Get(k)
		attrName := k[1:] // Quitar el @
		// Escribir atributo: name="valor"
		buf.WriteByte(' ')
		buf.WriteString(attrName)
		buf.WriteString(`="`)
		buf.WriteString(escapeAttr(fmt.Sprintf("%v", val)))
		buf.WriteString(`"`)
	}

	// Cerrar apertura de tag
	if tagName != "" {
		buf.WriteByte('>')
	}

	// 3. CONTENIDO (Hijos o Texto)
	hasContent := false

	// Primero buscamos si tiene contenido texto directo (#text)
	if textVal := m.Get("#text"); textVal != nil {
		buf.WriteString(escapeText(fmt.Sprintf("%v", textVal)))
		hasContent = true
	}

	// Luego procesamos los hijos (Keys que NO son atributos ni #text)
	for _, k := range keys {
		if !strings.HasPrefix(k, "@") && k != "#text" {
			val := m.Get(k)
			// Recursividad
			if err := writeCanonical(buf, val, k); err != nil {
				return err
			}
			hasContent = true
		}
	}
	if !hasContent {
		buf.WriteString("/>")
	} else {
		buf.WriteString("</")
		buf.WriteString(tagName)
		buf.WriteString(">")
	}

	return nil
}

// Helpers de escape (Mínimo necesario para XML)
func escapeText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, "\r", "&#xD;")
	return s
}

func escapeAttr(s string) string {
	s = escapeText(s)
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "\n", "&#xA;")
	s = strings.ReplaceAll(s, "\t", "&#x9;")
	return s
}
