package xml

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// ============================================================================
// 4. STREAMING ENCODER (Feature 3: Escritura Eficiente)
// ============================================================================

// Encoder escribe XML directamente a un io.Writer.
type Encoder struct {
	w   io.Writer
	cfg *config
}

// NewEncoder crea un encoder configurado.
func NewEncoder(w io.Writer, opts ...Option) *Encoder {
	cfg := &config{namespaces: make(map[string]string)}
	for _, opt := range opts {
		opt(cfg)
	}
	return &Encoder{w: w, cfg: cfg}
}

// Encode escribe el mapa como XML.
func (e *Encoder) Encode(data map[string]any) error {
	if len(data) != 1 {
		return errors.New("root debe tener exactamente 1 elemento")
	}
	for k, v := range data {
		if err := encodeNode(e.w, k, v, e.cfg, 0); err != nil {
			return err
		}
	}
	return nil
}

// Marshal Wrapper Legacy (para compatibilidad con v2)
func Marshal(data map[string]any, opts ...Option) (string, error) {
	var buf bytes.Buffer
	enc := NewEncoder(&buf, opts...)
	if err := enc.Encode(data); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// encodeNode es la función recursiva que ahora escribe a io.Writer
func encodeNode(w io.Writer, tag string, value any, cfg *config, depth int) error {
	indent := ""
	if cfg.prettyPrint {
		indent = "\n" + strings.Repeat("  ", depth)
	}

	// 1. Apertura del Tag
	fmt.Fprint(w, indent+"<"+tag)

	// Feature: Inyección de Namespace Reverso (Solo en raíz)
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
	var children map[string]any
	var isSimple bool
	var cdataContent string
	var comments []string

	// 2. Procesamiento de Atributos e Hijos
	switch v := value.(type) {
	case map[string]any:
		children = v
		keys := sortedKeys(v)
		for _, k := range keys {
			val := v[k]
			if strings.HasPrefix(k, "@") {
				// Feature: Soporte de Atributos en Root y Hijos
				// Escribe atributos directamente en el tag de apertura
				esc := escapeString(fmt.Sprintf("%v", val))
				fmt.Fprintf(w, ` %s="%s"`, strings.TrimPrefix(k, "@"), esc)
			} else if k == "#text" {
				content = val
			} else if k == "#cdata" {
				cdataContent = fmt.Sprintf("%v", val)
			} else if k == "#comments" {
				if list, ok := val.([]string); ok {
					comments = list
				}
			}
		}
	default:
		isSimple = true
		content = v
	}

	fmt.Fprint(w, ">")

	// 3. Escribir Comentarios
	if len(comments) > 0 {
		fmt.Fprint(w, "<!--")
	}

	for _, c := range comments {
		if cfg.prettyPrint {
			fmt.Fprint(w, "\n"+strings.Repeat("  ", depth+1))
		}
		fmt.Fprint(w, c)
	}

	if len(comments) > 0 {
		fmt.Fprint(w, "-->")
	}

	// 4. Escribir Contenido
	if cdataContent != "" {
		if cfg.prettyPrint {
			fmt.Fprint(w, "\n"+strings.Repeat("  ", depth+1))
		}
		fmt.Fprint(w, "<![CDATA["+cdataContent+"]]>")
	} else if content != nil {
		xml.EscapeText(w, []byte(fmt.Sprintf("%v", content)))
	}

	// 5. Hijos Recursivos
	if !isSimple && len(children) > 0 {
		keys := sortedKeys(children)
		hasComplex := false
		for _, k := range keys {
			val := children[k]
			if !strings.HasPrefix(k, "@") && k != "#text" && k != "#cdata" && k != "#comments" {
				hasComplex = true
				if list, ok := val.([]any); ok {
					for _, item := range list {
						if err := encodeNode(w, k, item, cfg, depth+1); err != nil {
							return err
						}
					}
				} else {
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

// Helpers Marshal
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

// ============================================================================
// REEMPLAZAR ESTA SECCIÓN EN main.go (VALIDACIÓN)
// ============================================================================

func Validate(data any, rules []Rule) []string {
	var errs []string
	for _, r := range rules {
		val, err := Query(data, r.Path)

		// 1. Chequeo de Existencia (Required)
		if err != nil {
			if r.Required {
				errs = append(errs, "Falta: "+r.Path)
			}
			continue
		}

		var floatVal float64
		var strVal string
		isNum := false
		isStr := false

		// 2. Chequeo de Tipos y Casteo
		switch r.Type {
		case "array":
			if _, ok := val.([]any); !ok {
				errs = append(errs, fmt.Sprintf("%s debe ser array", r.Path))
			}
		case "int", "float":
			if v, ok := asFloat(val); ok {
				floatVal = v
				isNum = true
			} else {
				errs = append(errs, fmt.Sprintf("%s debe ser numérico", r.Path))
			}
		case "string":
			strVal = fmt.Sprintf("%v", val)
			isStr = true
		}

		// 3. Reglas Avanzadas (Min, Max, Regex, Enum)
		if isNum {
			if r.Min != 0 && floatVal < r.Min {
				errs = append(errs, fmt.Sprintf("%s valor %.2f menor al mínimo %.2f", r.Path, floatVal, r.Min))
			}
			if r.Max != 0 && floatVal > r.Max {
				errs = append(errs, fmt.Sprintf("%s valor %.2f mayor al máximo %.2f", r.Path, floatVal, r.Max))
			}
		}

		if isStr {
			if r.Regex != "" {
				matched, _ := regexp.MatchString(r.Regex, strVal)
				if !matched {
					errs = append(errs, fmt.Sprintf("%s formato inválido (Regex)", r.Path))
				}
			}
			if len(r.Enum) > 0 {
				found := false
				for _, allowed := range r.Enum {
					if strVal == allowed {
						found = true
						break
					}
				}
				if !found {
					errs = append(errs, fmt.Sprintf("%s valor inválido. Permitidos: %v", r.Path, r.Enum))
				}
			}
		}
	}
	return errs
}

func asFloat(v any) (float64, bool) {
	switch i := v.(type) {
	case int:
		return float64(i), true
	case float64:
		return i, true
	case string:
		if f, err := strconv.ParseFloat(i, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}
