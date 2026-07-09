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

// ToJSON is a smart polymorphic wrapper.
// It accepts:
// 1. *OrderedMap: Preserves order.
// 2. io.Reader: Reads an XML stream and converts it to JSON.
// 3. any: Fallback to standard json.Marshal.
func ToJSON(data any) (string, error) {
	// Case 1: OrderedMap (Already in memory)
	if om, ok := data.(*OrderedMap); ok {
		return om.ToJSON()
	}

	// Case 2: Stream (File / Stdin / HTTP Body)
	if r, ok := data.(io.Reader); ok {
		b, err := ReaderToJSON(r)
		return string(b), err
	}

	// Case 3: Fallback (Native map, Struct, etc.)
	b, err := json.Marshal(data)
	return string(b), err
}

// ReaderToJSON reads XML from a Reader and returns the JSON bytes.
// This function is used internally by ToJSON.
func ReaderToJSON(r io.Reader) ([]byte, error) {
	// 1. Parse XML into an OrderedMap
	m, err := MapXML(r)
	if err != nil {
		return nil, fmt.Errorf("parse error: %w", err)
	}

	// 2. Convert to JSON (Using OrderedMap's MarshalJSON method)
	return m.MarshalJSON()
}

// ToCSV writes a list of nodes in CSV format.
// Usage: r2xml csv data.xml --path="orders/order"
func ToCSV(w io.Writer, nodes []*OrderedMap) error {
	if len(nodes) == 0 {
		return nil
	}

	// 1. Discover Headers (Unify keys from all nodes to be robust)
	headerMap := make(map[string]bool)
	var headers []string

	for _, node := range nodes {
		for _, k := range node.Keys() {
			// Ignore attributes (@), text (#text) and cdata (#cdata) for clean CSV
			if !headerMap[k] && !strings.HasPrefix(k, "@") && !strings.HasPrefix(k, "#") {
				headerMap[k] = true
				headers = append(headers, k)
			}
		}
	}
	sort.Strings(headers) // Deterministic A-Z order

	// 2. Write Header
	if _, err := fmt.Fprintln(w, strings.Join(headers, ",")); err != nil {
		return err
	}

	// 3. Write Rows
	for _, node := range nodes {
		var row []string
		for _, h := range headers {
			val := node.String(h)

			// Escape double quotes for standard CSV (RFC 4180)
			val = strings.ReplaceAll(val, "\"", "\"\"")

			// If it contains commas, newlines or quotes, wrap in quotes
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

	// 1. Discover Headers
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
