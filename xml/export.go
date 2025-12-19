package xml

import (
	"fmt"
	"io"
	"sort"
	"strings"
)

// ============================================================================
// EXPORT UTILITIES (Bridges for CLI)
// ============================================================================

// ToJSON lee XML desde un Reader y devuelve el JSON en []byte.
// Esta es la función que usa 'cli.go'.
func ReaderToJSON(r io.Reader) ([]byte, error) {
	// 1. Parsear XML a OrderedMap
	m, err := MapXML(r)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// 2. Convertir a JSON (Usando el método MarshalJSON de OrderedMap)
	// Nota: m.MarshalJSON() preserva el orden.
	return m.MarshalJSON()
}

// ToCSV escribe una lista de nodos en formato CSV.
// Uso: r2xml csv data.xml --path="orders/order"
func ToCSV(w io.Writer, nodes []*OrderedMap) error {
	if len(nodes) == 0 {
		return nil
	}

	// 1. Descubrir Headers (Unificar claves de todos los nodos para ser robusto)
	// Esto es necesario porque algunos nodos pueden tener claves que otros no.
	headerMap := make(map[string]bool)
	var headers []string

	for _, node := range nodes {
		for _, k := range node.Keys() {
			// Ignoramos atributos (@) y texto crudo (#text) para el CSV,
			// o podrías incluirlos si quisieras. Por ahora limpiamos.
			if !headerMap[k] && k != "#text" && k != "#cdata" {
				headerMap[k] = true
				headers = append(headers, k)
			}
		}
	}
	sort.Strings(headers) // Orden determinista A-Z para las columnas

	// 2. Escribir Header
	if _, err := fmt.Fprintln(w, strings.Join(headers, ",")); err != nil {
		return err
	}

	// 3. Escribir Filas
	for _, node := range nodes {
		var row []string
		for _, h := range headers {
			val := node.String(h)

			// Escapar comillas dobles para CSV estándar (RFC 4180)
			val = strings.ReplaceAll(val, "\"", "\"\"")

			// Si contiene comas, saltos de línea o comillas, envolver en comillas
			if strings.ContainsAny(val, ",\n\"") {
				val = fmt.Sprintf("\"%s\"", val)
			}

			row = append(row, val)
		}
		if _, err := fmt.Fprintln(w, strings.Join(row, ",")); err != nil {
			return err
		}
	}
	return nil
}
