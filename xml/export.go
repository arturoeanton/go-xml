package xml

import (
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
)

// ============================================================================
// EXPORT UTILITIES
// ============================================================================

// ToJSON es un wrapper polimórfico inteligente.
// Acepta:
// 1. *OrderedMap: Preserva orden.
// 2. io.Reader: Lee XML stream y convierte a JSON.
// 3. any: Fallback a json.Marshal estándar.
func ToJSON(data any) (string, error) {
	// Caso 1: OrderedMap (Ya en memoria)
	if om, ok := data.(*OrderedMap); ok {
		return om.ToJSON()
	}

	// Caso 2: Stream (File / Stdin / HTTP Body)
	if r, ok := data.(io.Reader); ok {
		b, err := ReaderToJSON(r)
		return string(b), err
	}

	// Caso 3: Fallback (Map nativo, Struct, etc.)
	b, err := json.Marshal(data)
	return string(b), err
}

// ReaderToJSON lee XML desde un Reader y devuelve los bytes JSON.
// Esta función es usada internamente por ToJSON.
func ReaderToJSON(r io.Reader) ([]byte, error) {
	// 1. Parsear XML a OrderedMap
	m, err := MapXML(r)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// 2. Convertir a JSON (Usando el método MarshalJSON de OrderedMap)
	return m.MarshalJSON()
}

// ToCSV escribe una lista de nodos en formato CSV.
// Uso: r2xml csv data.xml --path="orders/order"
func ToCSV(w io.Writer, nodes []*OrderedMap) error {
	if len(nodes) == 0 {
		return nil
	}

	// 1. Descubrir Headers (Unificar claves de todos los nodos para ser robusto)
	headerMap := make(map[string]bool)
	var headers []string

	for _, node := range nodes {
		for _, k := range node.Keys() {
			// Ignoramos atributos (@), texto (#text) y cdata (#cdata) para CSV limpio
			if !headerMap[k] && !strings.HasPrefix(k, "@") && !strings.HasPrefix(k, "#") {
				headerMap[k] = true
				headers = append(headers, k)
			}
		}
	}
	sort.Strings(headers) // Orden determinista A-Z

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
