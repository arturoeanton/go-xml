package xml

import (
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

/*
	r2/xml v1.0 - "The Enterprise Single-File XML Parser"
	=====================================================
	A robust, "Zero-Dependency", self-contained XML parser/mapper for Go.
	Designed to replace encoding/xml in dynamic scenarios and high-performance requirements.

	Feature List:
	1. MapXML Engine: Dynamic reading into map[string]any with automatic Text Simplification.
	2. Namespace Manager: Automatic URL cleaning (Aliasing) and injection on write (Definitions).
	3. ForceArray: Guarantees lists ([]any) by removing ambiguity for single items (JSON-Ready).
	4. Streaming Decoder: Efficient reading of giant files (GBs) using Generics (Stream[T]).
	5. Streaming Encoder: Direct writing to io.Writer without buffers, supporting Root Attributes.
	6. Query Engine: Deep navigation with filters ([k=v]), indices ([i]), and recursive search (QueryAll).
	7. Business Validation: Integrated rule engine (Required, Type, Min/Max, Regex, Enum).
	8. Smart Parsing: Type inference (int/bool), "Lenient Mode" (Dirty HTML), and Transformation Hooks.
	9. Rich Content Support: Native handling of CDATA blocks (#cdata) and Comment preservation (#comments).
	10. Soup Mode: Robust HTML scraping capabilities (case-insensitive, script protection, void tags).
*/

// ============================================================================
// 1. CONFIGURATION AND OPTIONS
// ============================================================================

type config struct {
	forceArrayKeys map[string]bool             // Tags that will always be treated as a list
	namespaces     map[string]string           // Namespace Aliases
	valueHooks     map[string]func(string) any // Transformation Hooks

	// Flags
	isLenient        bool // Tolerant mode for dirty HTML/XML
	inferTypes       bool // Automatic type inference (int, bool, float)
	prettyPrint      bool // Indentation for the encoder
	isSoupMode       bool // "Soup Mode" (Dirty HTML - Normalization & Sanitization)
	useCharsetReader bool // Use charset reader for ISO-8859-1 and Windows-1252

	htmlAutoClose []string // List of HTML Void Elements
}

// Option defines a function to modify the parser configuration.
type Option func(*config)

// defaultConfig devuelve la configuración por defecto.
func defaultConfig() *config {
	return &config{
		useCharsetReader: false,
	}
}

// ForceArray returns an Option that forces specific tags to be parsed as arrays ([]any).
// This prevents the common XML-to-JSON ambiguity where single items are parsed as objects.
func ForceArray(keys ...string) Option {
	return func(c *config) {
		for _, k := range keys {
			c.forceArrayKeys[k] = true
		}
	}
}

// EnableLegacyCharsets habilita el soporte para ISO-8859-1 y Windows-1252.
func EnableLegacyCharsets() Option {
	return func(c *config) {
		c.useCharsetReader = true
	}
}

// RegisterNamespace returns an Option that registers a short alias for a namespace URL.
// Example: RegisterNamespace("html", "http://www.w3.org/html")
func RegisterNamespace(alias, url string) Option {
	return func(c *config) { c.namespaces[url] = alias }
}

// WithValueHook returns an Option that registers a custom transformation function for a specific tag.
// Use this to parse dates, custom formats, or decrypt data on the fly.
func WithValueHook(tagName string, fn func(string) any) Option {
	return func(c *config) { c.valueHooks[tagName] = fn }
}

// EnableExperimental (Soup Mode) enables aggressive settings for parsing dirty HTML.
// It activates:
// 1. Type Inference (strings are converted to int/bool/float if possible).
// 2. Lenient Mode (ignores strict XML syntax errors).
// 3. Soup Mode (Case-insensitive normalization for tags and attributes).
// 4. Void Element support (e.g., <br>, <img>, <input>).
func EnableExperimental() Option {
	return func(c *config) {
		c.inferTypes = true
		c.isLenient = true
		c.isSoupMode = true // Activates lowercase normalization

		// Complete list of Void Elements (HTML5)
		c.htmlAutoClose = []string{
			"area", "base", "br", "col", "embed", "hr", "img", "input",
			"link", "meta", "param", "source", "track", "wbr",
			"command", "keygen", "menuitem",
		}
	}
}

// WithPrettyPrint enables indentation for the Encoder (output beautification).
func WithPrettyPrint() Option {
	return func(c *config) { c.prettyPrint = true }
}

// ============================================================================
// 2. PARSER CORE (MapXML - In-Memory Reading)
// ============================================================================

type node struct {
	tagName string
	data    *OrderedMap
}

// MapXML reads the entire XML input from the reader and returns a dynamic OrderedMap.
// It handles hierarchical data, attributes, CDATA, comments, and preserves mixed-content order via "#seq".
func MapXML(r io.Reader, opts ...Option) (*OrderedMap, error) {
	cfg := &config{
		forceArrayKeys: make(map[string]bool),
		namespaces:     make(map[string]string),
		valueHooks:     make(map[string]func(string) any),
	}
	for _, opt := range opts {
		opt(cfg)
	}

	// === NEW: SANITIZER INJECTION ===
	// If Soup Mode is active, we protect raw tags (like scripts) before parsing.
	if cfg.isSoupMode {
		r = sanitizeSoup(r)
	}

	decoder := xml.NewDecoder(r)
	if cfg.isLenient {
		decoder.Strict = false
		decoder.AutoClose = cfg.htmlAutoClose
		decoder.Entity = xml.HTMLEntity
	}

	// === NEW: CHARSET READER ===
	if cfg.useCharsetReader {
		decoder.CharsetReader = charsetReader
	}

	root := NewMap()
	stack := []*node{{tagName: "", data: root}}

	for {
		token, err := decoder.Token()
		if err != nil {
			// 1. Primero manejamos EOF (Fin de archivo), siempre debemos salir.
			if err == io.EOF {
				break // O return nil, dependiendo de tu lógica
			}

			// 2. Aquí aplicamos el parche para Soup Mode.
			// Si hay error (que no es EOF) y estamos en Soup Mode, lo ignoramos.
			if cfg.isSoupMode {
				continue // Ignoramos el error y buscamos el siguiente token
			}

			// 3. Si NO es Soup Mode, el error es fatal (comportamiento default).
			return nil, wrapError(err)
		}

		switch se := token.(type) {
		case xml.StartElement:
			// 1. TAG NORMALIZATION (Fix for Soup Mode)
			localName := se.Name.Local
			if cfg.isSoupMode {
				localName = strings.ToLower(localName)
			}

			tagName := resolveName(xml.Name{Space: se.Name.Space, Local: localName}, cfg.namespaces)
			currentMap := NewMap()

			for _, attr := range se.Attr {
				// 2. ATTRIBUTE NORMALIZATION
				attrName := attr.Name.Local
				if cfg.isSoupMode {
					attrName = strings.ToLower(attrName)
				}
				attrName = resolveName(xml.Name{Space: attr.Name.Space, Local: attrName}, cfg.namespaces)

				currentMap.Put("@"+attrName, processValue(attr.Value, "", cfg))
			}
			stack = append(stack, &node{tagName: tagName, data: currentMap})

		case xml.CharData:
			rawContent := string(se)

			// 1. Limpieza para #text (Vista de Datos - Trimmed)
			// Eliminamos ruido para que acceder a m["id"] devuelva "123" y no " 123 "
			textContent := strings.TrimSpace(rawContent)

			// 2. Limpieza para #seq (Vista de Documento - Preserved)
			// Solo normalizamos saltos de línea a espacios, pero NO hacemos trim de bordes
			// para no pegar palabras ("The " + "stock" -> "The stock").
			seqContent := strings.ReplaceAll(rawContent, "\n", " ")
			seqContent = strings.ReplaceAll(seqContent, "\t", " ")
			// Opcional: Colapsar múltiples espacios a uno solo si se desea muy limpio
			// pero para mixed content, preservar 1 espacio es clave.
			if strings.TrimSpace(seqContent) == "" {
				seqContent = "" // Si era solo indentación, lo ignoramos para no ensuciar
			}

			// Decisión de si procesamos este nodo (si tiene contenido útil)
			if textContent != "" || seqContent != "" {
				current := stack[len(stack)-1]

				// A. Guardamos en #text (Versión Limpia/Trimmed)
				if textContent != "" {
					if val := current.data.Get("#text"); val != nil {
						// Nota: Al concatenar #text legacy, agregamos un espacio por seguridad
						// ya que estamos uniendo fragmentos distantes.
						current.data.Put("#text", val.(string)+textContent)
					} else {
						current.data.Put("#text", textContent)
					}
				}

				// B. Guardamos en #seq (Versión Original/Spaced)
				// Esta es la que usa la función Text() y ExampleText
				if seqContent != "" {
					if seq, ok := current.data.Get("#seq").([]any); ok {
						current.data.Put("#seq", append(seq, seqContent))
					} else {
						current.data.Put("#seq", []any{seqContent})
					}
				}
			}

		case xml.Comment:
			content := string(se)
			current := stack[len(stack)-1]
			if list, ok := current.data.Get("#comments").([]string); ok {
				current.data.Put("#comments", append(list, content))
			} else {
				current.data.Put("#comments", []string{content})
			}

		case xml.EndElement:
			// Pop from stack
			childNode := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			parent := stack[len(stack)-1]
			tagName := childNode.tagName

			var finalValue any = childNode.data

			// 4. NODE SIMPLIFICATION
			// If the node only contains text (and the redundant sequence), simplify it to a string.
			if childNode.data.Len() == 1 {
				// Case: Only #text
				if val := childNode.data.Get("#text"); val != nil {
					finalValue = processValue(val.(string), tagName, cfg)
				}
			} else if childNode.data.Len() == 2 {
				// Case: #text AND #seq (redundant because there are no child tags)
				hasText := childNode.data.Has("#text")
				hasSeq := childNode.data.Has("#seq")
				if hasText && hasSeq {
					finalValue = processValue(childNode.data.Get("#text").(string), tagName, cfg)
				}
			}

			// 5. ASSIGN TO PARENT (Map Structure)
			existingValue := parent.data.Get(tagName)
			if existingValue == nil {
				if cfg.forceArrayKeys[tagName] {
					parent.data.Put(tagName, []any{finalValue})
				} else {
					parent.data.Put(tagName, finalValue)
				}
			} else {
				if list, ok := existingValue.([]any); ok {
					parent.data.Put(tagName, append(list, finalValue))
				} else {
					parent.data.Put(tagName, []any{existingValue, finalValue})
				}
			}

			// 6. ASSIGN TO PARENT SEQUENCE (Ordered Structure)
			// This is the key to supporting "Mixed Content". The parent knows exactly
			// when this child appeared relative to text nodes.
			if seq, ok := parent.data.Get("#seq").([]any); ok {
				parent.data.Put("#seq", append(seq, finalValue))
			} else {
				parent.data.Put("#seq", []any{finalValue})
			}

		case xml.ProcInst:
			current := stack[len(stack)-1]
			piContent := fmt.Sprintf("target=%s data=%s", se.Target, string(se.Inst))
			if list, ok := current.data.Get("#pi").([]string); ok {
				current.data.Put("#pi", append(list, piContent))
			} else {
				current.data.Put("#pi", []string{piContent})
			}

		case xml.Directive:
			current := stack[len(stack)-1]
			directive := string(se)
			if list, ok := current.data.Get("#directive").([]string); ok {
				current.data.Put("#directive", append(list, directive))
			} else {
				current.data.Put("#directive", []string{directive})
			}
		}
	}
	return root, nil
}

func resolveName(name xml.Name, nsMap map[string]string) string {
	if alias, ok := nsMap[name.Space]; ok && alias != "" {
		return alias + ":" + name.Local
	}
	return name.Local
}

func processValue(val string, tagName string, cfg *config) any {
	if hook, ok := cfg.valueHooks[tagName]; ok {
		return hook(val)
	}
	if cfg.inferTypes {
		return inferType(val)
	}
	return val
}

func inferType(val string) any {
	if val == "true" {
		return true
	}
	if val == "false" {
		return false
	}
	if i, err := strconv.Atoi(val); err == nil {
		return i
	}
	if f, err := strconv.ParseFloat(val, 64); err == nil && strings.Contains(val, ".") {
		return f
	}
	return val
}
