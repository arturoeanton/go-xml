package xml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
)

// ============================================================================
// 4. STREAMING ENCODER
// ============================================================================

// Encoder writes XML directly to an io.Writer.
type Encoder struct {
	w   io.Writer
	cfg *config
}

// NewEncoder creates a configured encoder.
func NewEncoder(w io.Writer, opts ...Option) *Encoder {
	cfg := &config{namespaces: make(map[string]string)}
	for _, opt := range opts {
		opt(cfg)
	}
	return &Encoder{w: w, cfg: cfg}
}

// Encode writes the map data as XML.
// It ensures there is exactly one root element (ignoring metadata keys like #seq).
func (e *Encoder) Encode(data map[string]any) error {
	// Validación de Raíz Única (ignorando metadata)
	rootCount := 0
	for k := range data {
		if !strings.HasPrefix(k, "#") {
			rootCount++
		}
	}
	if rootCount != 1 {
		return errors.New("root must have exactly 1 element")
	}

	// Ordenamos claves para output determinista
	keys := sortedKeys(data)
	for _, k := range keys {
		// Ignoramos metadata al nivel raíz
		if !strings.HasPrefix(k, "#") {
			if err := encodeNode(e.w, k, data[k], e.cfg, 0); err != nil {
				return err
			}
		}
	}
	return nil
}

// Marshal returns the XML as a string (Helper wrapper).
func Marshal(data map[string]any, opts ...Option) (string, error) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf, opts...)
	if err := enc.Encode(data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// encodeNode writes a single node recursively.
func encodeNode(w io.Writer, tag string, value any, cfg *config, depth int) error {
	indent := ""
	if cfg.prettyPrint {
		indent = "\n" + strings.Repeat("  ", depth)
	}

	// 1. Tag Opening
	fmt.Fprint(w, indent+"<"+tag)

	// Inyectar Namespaces en el Root (profundidad 0)
	if depth == 0 {
		var urls []string
		for u := range cfg.namespaces {
			urls = append(urls, u)
		}
		sort.Strings(urls)
		for _, u := range urls {
			alias := cfg.namespaces[u]
			fmt.Fprintf(w, ` xmlns:%s="%s"`, alias, u)
		}
	}

	var content any
	var cdataContent string
	var children map[string]any
	var isSimple bool

	// 2. Procesar Atributos y Contenido
	switch v := value.(type) {
	case map[string]any:
		children = v
		keys := sortedKeys(v)
		for _, k := range keys {
			val := v[k]
			// Atributos
			if strings.HasPrefix(k, "@") {
				esc := escapeString(fmt.Sprintf("%v", val))
				fmt.Fprintf(w, ` %s="%s"`, strings.TrimPrefix(k, "@"), esc)
			} else if k == "#text" {
				content = val
			} else if k == "#cdata" {
				cdataContent = fmt.Sprintf("%v", val)
			}
			// NOTA: Ignoramos #seq, #comments aquí para evitar duplicación o basura
		}
	default:
		// Caso primitivo (string, int, etc.)
		isSimple = true
		content = v
	}

	fmt.Fprint(w, ">")

	// 3. Escribir Contenido (#cdata tiene prioridad sobre #text)
	if cdataContent != "" {
		if cfg.prettyPrint {
			fmt.Fprint(w, "\n"+strings.Repeat("  ", depth+1))
		}
		fmt.Fprint(w, "<![CDATA["+cdataContent+"]]>")
	} else if content != nil {
		xml.EscapeText(w, []byte(fmt.Sprintf("%v", content)))
	}

	// 4. Hijos Recursivos
	if !isSimple && len(children) > 0 {
		keys := sortedKeys(children)
		hasComplex := false

		for _, k := range keys {
			val := children[k]

			// === FIX CRÍTICO ===
			// Excluir TODA metadata (Keys que empiezan con # o @)
			// Esto evita que #seq se escriba como un tag <#seq> o vuelque basura.
			if !strings.HasPrefix(k, "@") && !strings.HasPrefix(k, "#") {
				hasComplex = true

				// Manejo de Arrays (Repetir tag)
				if list, ok := val.([]any); ok {
					for _, item := range list {
						if err := encodeNode(w, k, item, cfg, depth+1); err != nil {
							return err
						}
					}
				} else {
					// Nodo simple/mapa
					if err := encodeNode(w, k, val, cfg, depth+1); err != nil {
						return err
					}
				}
			}
		}

		if hasComplex && cfg.prettyPrint {
			fmt.Fprint(w, indent)
		}
	}

	fmt.Fprint(w, "</"+tag+">")
	return nil
}

// Helpers
func sortedKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func escapeString(s string) string {
	var b bytes.Buffer
	xml.EscapeText(&b, []byte(s))
	return b.String()
}
