package xml

import (
	"encoding/csv"
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

// ============================================================================
// CSV EXPORT (configurable)
// ============================================================================

type csvConfig struct {
	delimiter  rune
	quoteAll   bool
	flattenSep string // "" = skip nested objects (ToCSV's current behavior)
}

// CSVOption configures ToCSVWithOptions.
type CSVOption func(*csvConfig)

// WithDelimiter sets the field separator (default ',').
func WithDelimiter(r rune) CSVOption {
	return func(c *csvConfig) { c.delimiter = r }
}

// WithQuoteAll forces every field to be quoted, instead of only the fields
// that RFC 4180 requires it for.
func WithQuoteAll(b bool) CSVOption {
	return func(c *csvConfig) { c.quoteAll = b }
}

// WithFlatten flattens one level of nested *OrderedMap children into
// columns named "parent<sep>child", instead of skipping them (ToCSV's
// default behavior).
func WithFlatten(sep string) CSVOption {
	return func(c *csvConfig) { c.flattenSep = sep }
}

// ToCSVWithOptions is ToCSV with configurable delimiter, quoting and
// nested-object flattening, built on encoding/csv (correct RFC 4180
// quoting/escaping, including embedded CRLF) instead of ToCSV's hand-rolled
// version.
func ToCSVWithOptions(w io.Writer, nodes []*OrderedMap, opts ...CSVOption) error {
	cfg := &csvConfig{delimiter: ','}
	for _, o := range opts {
		o(cfg)
	}
	if len(nodes) == 0 {
		return nil
	}

	// 1. Descubrir Headers
	headerMap := make(map[string]bool)
	var headers []string
	addHeader := func(h string) {
		if !headerMap[h] {
			headerMap[h] = true
			headers = append(headers, h)
		}
	}
	for _, node := range nodes {
		for _, k := range node.Keys() {
			if strings.HasPrefix(k, "@") || strings.HasPrefix(k, "#") {
				continue
			}
			if child, ok := node.Get(k).(*OrderedMap); ok {
				if cfg.flattenSep == "" {
					continue // nested objects skipped unless WithFlatten is set
				}
				for _, ck := range child.Keys() {
					if strings.HasPrefix(ck, "@") || strings.HasPrefix(ck, "#") {
						continue
					}
					addHeader(k + cfg.flattenSep + ck)
				}
				continue
			}
			addHeader(k)
		}
	}
	sort.Strings(headers)

	cw := csv.NewWriter(w)
	cw.Comma = cfg.delimiter

	writeRow := func(fields []string) error {
		if !cfg.quoteAll {
			return cw.Write(fields)
		}
		quoted := make([]string, len(fields))
		for i, f := range fields {
			quoted[i] = `"` + strings.ReplaceAll(f, `"`, `""`) + `"`
		}
		_, err := io.WriteString(w, strings.Join(quoted, string(cfg.delimiter))+"\r\n")
		return err
	}

	if err := writeRow(headers); err != nil {
		return err
	}

	for _, node := range nodes {
		row := make([]string, len(headers))
		for i, h := range headers {
			if cfg.flattenSep != "" && strings.Contains(h, cfg.flattenSep) {
				parts := strings.SplitN(h, cfg.flattenSep, 2)
				row[i] = node.String(parts[0] + "/" + parts[1])
			} else {
				row[i] = node.String(h)
			}
		}
		if err := writeRow(row); err != nil {
			return err
		}
	}

	if !cfg.quoteAll {
		cw.Flush()
		return cw.Error()
	}
	return nil
}
